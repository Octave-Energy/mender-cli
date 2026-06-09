// Copyright 2023 Northern.tech AS
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/mendersoftware/go-lib-micro/ws"
	wspf "github.com/mendersoftware/go-lib-micro/ws/portforward"
	"github.com/vmihailenco/msgpack"

	"github.com/mendersoftware/mender-cli/client/deviceconnect"
	"github.com/mendersoftware/mender-cli/log"
)

// Run executes the port-forward session, transparently restarting it when the
// device requests a restart.
func (c *PortForwardCmd) Run() error {
	for {
		if err := c.run(); !errors.Is(err, errRestart) {
			return err
		}
	}
}

func (c *PortForwardCmd) run() error {
	ctx, cancelContext := context.WithCancel(context.Background())
	defer cancelContext()

	client := deviceconnect.NewClient(c.server, c.token, c.skipVerify)

	// check if the device is connected
	if err := ensureDeviceConnected(client, c.deviceID); err != nil {
		return err
	}

	// connect to the websocket and start the ping-pong connection health-check
	err := client.Connect(c.deviceID, c.token)
	if err != nil {
		return err
	}

	go client.PingPong(ctx)
	defer client.Close()

	// perform ws protocol handshake
	err = c.handshake(client)
	if err != nil {
		return err
	}

	// message channel
	msgChan := make(chan *ws.ProtoMsg)

	// start the local TCP listeners
	for _, portMapping := range c.portMappings {
		switch portMapping.Protocol {
		case protocolTCP:
			forwarder, err := NewTCPPortForwarder(c.bindingHost, portMapping.LocalPort,
				portMapping.RemoteHost, portMapping.RemotePort, c.proto)
			if err != nil {
				return err
			}
			go forwarder.Run(ctx, c.sessionID, msgChan, c.recvChans)
		case protocolUDP:
			forwarder, err := NewUDPPortForwarder(c.bindingHost, portMapping.LocalPort,
				portMapping.RemoteHost, portMapping.RemotePort, c.proto)
			if err != nil {
				return err
			}
			go forwarder.Run(ctx, c.sessionID, msgChan, c.recvChans)
		default:
			return errors.New("unknown protocol: " + portMapping.Protocol)
		}
	}

	c.running = true
	go c.processIncomingMessages(msgChan, client)

	// handle CTRL+C and signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// wait for CTRL+C, signals or stop
	restart := false
	timeout := time.Now().Add(portForwardMaxDuration)
	for c.running {
		select {
		case msg := <-msgChan:
			err := client.WriteMessage(msg)
			if err != nil {
				c.err = err
				c.running = false
			}
		case <-ctx.Done():
			c.err = ctx.Err()
			c.running = false
		case <-time.After(time.Until(timeout)):
			c.err = errors.New("port forward timed out: max duration reached")
			c.running = false
		case <-quit:
			c.running = false
		case <-c.stop:
			restart = true
			c.running = false
		}
	}

	// cancel the context
	cancelContext()

	// close the ws session
	err = c.closeSession(client)
	if c.err == nil && err != nil {
		c.err = err
	}

	// if stopping because of an error, restart the port-forwarding command
	if restart {
		return errRestart
	}

	// return the error message (if any)
	return c.err
}

func (c *PortForwardCmd) Stop() {
	c.stop <- struct{}{}
}

// handshake initiates a handshake and checks that the device
// is willing to accept port forward requests.
func (c *PortForwardCmd) handshake(client *deviceconnect.Client) error {
	// open the session
	body, err := msgpack.Marshal(&ws.Open{
		Versions: []int{ws.ProtocolVersion},
	})
	if err != nil {
		return err
	}
	m := &ws.ProtoMsg{
		Header: ws.ProtoHdr{
			Proto:   ws.ProtoTypeControl,
			MsgType: ws.MessageTypeOpen,
		},
		Body: body,
	}
	err = client.WriteMessage(m)
	if err != nil {
		return err
	}

	msg, err := client.ReadMessage()
	if err != nil {
		return err
	}
	if msg.Header.MsgType == ws.MessageTypeError {
		erro := new(ws.Error)
		_ = msgpack.Unmarshal(msg.Body, erro)
		return fmt.Errorf("handshake error from client: %s", erro.Error)
	} else if msg.Header.MsgType != ws.MessageTypeAccept {
		return errPortForwardNotImplemented
	}

	accept := new(ws.Accept)
	err = msgpack.Unmarshal(msg.Body, accept)
	if err != nil {
		return err
	}
	if slices.Contains(accept.Protocols, ws.ProtoTypePortForwardV2) {
		c.proto = ws.ProtoTypePortForwardV2
	} else if slices.Contains(accept.Protocols, ws.ProtoTypePortForward) {
		c.proto = ws.ProtoTypePortForward
	} else {
		return errPortForwardNotImplemented
	}

	c.sessionID = msg.Header.SessionID
	return nil
}

// closeSession closes the WS session
func (c *PortForwardCmd) closeSession(client *deviceconnect.Client) error {
	m := &ws.ProtoMsg{
		Header: ws.ProtoHdr{
			Proto:   ws.ProtoTypeControl,
			MsgType: ws.MessageTypeClose,
		},
	}
	err := client.WriteMessage(m)
	if err != nil {
		return err
	}

	return nil
}

func (c *PortForwardCmd) processIncomingMessages(
	msgChan chan *ws.ProtoMsg,
	client *deviceconnect.Client,
) {
	for c.running {
		m, err := client.ReadMessage()
		if err != nil {
			c.err = err
			c.running = false
			c.Stop()
			break
		}
		switch m.Header.Proto {
		case ws.ProtoTypeControl:
			if m.Header.MsgType == ws.MessageTypePing {
				m := &ws.ProtoMsg{
					Header: ws.ProtoHdr{
						Proto:     ws.ProtoTypeControl,
						MsgType:   ws.MessageTypePong,
						SessionID: c.sessionID,
					},
				}
				msgChan <- m
			} else {
				c.err = fmt.Errorf("invalid message type control/%s", m.Header.MsgType)
				log.Err(c.err.Error())
				c.running = false
				c.Stop()
			}
		case c.proto:
			switch m.Header.MsgType {
			case wspf.MessageTypeError:
				erro := new(ws.Error)
				if err := msgpack.Unmarshal(m.Body, erro); err != nil &&
					erro.MessageType != wspf.MessageTypePortForwardStop {
					c.err = fmt.Errorf(
						"unable to start the port-forwarding: %s",
						string(m.Body),
					)
					c.running = false
					c.Stop()
				}
			case wspf.MessageTypePortForwardAck:
				if c.proto == ws.ProtoTypePortForwardV2 {
					c.err = fmt.Errorf("invalid message type %s", m.Header.MsgType)
					c.running = false
					c.Stop()
					break
				}
				fallthrough
			case wspf.MessageTypePortForward,
				wspf.MessageTypePortForwardStop,
				wspf.MessageTypePortForwardNew:
				connectionID, _ := m.Header.Properties[wspf.PropertyConnectionID].(string)
				if connectionID != "" {
					if recvChan, ok := c.recvChans[connectionID]; ok {
						recvChan <- m
					}
				}

			default:
				c.err = fmt.Errorf("invalid message type %s", m.Header.MsgType)
				log.Err(c.err.Error())
				c.running = false
				c.Stop()
			}
		default:
			c.err = fmt.Errorf("invalid protocol ID %d", m.Header.Proto)
			log.Err(c.err.Error())
			c.running = false
			c.Stop()

		}
	}
}
