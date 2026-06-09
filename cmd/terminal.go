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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mendersoftware/go-lib-micro/ws"
	wsshell "github.com/mendersoftware/go-lib-micro/ws/shell"

	"github.com/mendersoftware/mender-cli/client/deviceconnect"
	"github.com/mendersoftware/mender-cli/log"
)

const (
	// default terminal size
	defaultTermWidth  = 80
	defaultTermHeight = 40

	// dummy delay for playback
	playbackSleep = time.Millisecond * 32

	// sessionMaxDuration bounds how long a remote session (terminal or
	// port-forward) may stay open before it is closed.
	sessionMaxDuration = 24 * time.Hour

	// stdinReadSize is the buffer size used when reading from stdin.
	stdinReadSize = 1024

	// terminalQuitChar is the byte (CTRL+], ASCII GS) that terminates the
	// interactive session.
	terminalQuitChar = 29

	// cli args
	argRecord   = "record"
	argPlayback = "playback"
)

const terminalExamples = `  mender-cli terminal 0123456789abcdef0123456789abcdef
  mender-cli terminal --id 0123456789abcdef0123456789abcdef
  mender-cli terminal -f hostname=my-gateway
  mender-cli terminal 0123456789abcdef0123456789abcdef --record session.rec
  mender-cli terminal --playback session.rec`

var terminalCmd = &cobra.Command{
	Use:   "terminal [DEVICE_ID]",
	Short: "Remotely access a terminal on a device.",
	Long: "Remotely access a terminal on a device.\n\n" +
		"The target device can be selected with a positional DEVICE_ID, the --id " +
		"flag, or a --filter expression that matches exactly one device. This " +
		"starts a new terminal session with the remote device. The session can be " +
		"saved locally using the --record flag. When using the --playback flag, no " +
		"device is required and no connection will be established.",
	Example: terminalExamples,
	Args:    cobra.RangeArgs(0, 1),
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewTerminalCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	terminalCmd.Flags().StringP(argRecord, "", "", "recording file path to save the session to")
	terminalCmd.Flags().
		StringP(argPlayback, "", "", "recording file path to playback the session from")
	addDeviceTargetFlags(terminalCmd)
}

// TerminalCmd handles the terminal command
type TerminalCmd struct {
	server             string
	token              string
	skipVerify         bool
	deviceID           string
	sessionID          string
	running            bool
	healthcheck        chan int
	stop               chan struct{}
	err                error
	recordFile         string
	recording          bool
	stopRecording      chan bool
	playbackFile       string
	terminalOutputChan chan []byte
}

// NewTerminalCmd returns a new TerminalCmd
func NewTerminalCmd(cmd *cobra.Command, args []string) (*TerminalCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	recordFile, err := cmd.Flags().GetString(argRecord)
	if err != nil {
		return nil, err
	}

	playbackFile, err := cmd.Flags().GetString(argPlayback)
	if err != nil {
		return nil, err
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	deviceID, err := terminalDeviceID(cmd, args, playbackFile, server, token, skipVerify)
	if err != nil {
		return nil, err
	}

	return &TerminalCmd{
		server:             server,
		token:              token,
		skipVerify:         skipVerify,
		deviceID:           deviceID,
		healthcheck:        make(chan int),
		stop:               make(chan struct{}),
		recordFile:         recordFile,
		stopRecording:      make(chan bool),
		terminalOutputChan: make(chan []byte),
		playbackFile:       playbackFile,
	}, nil
}

// terminalDeviceID resolves the target device from the positional arg, --id, or
// --filter (at most one of them). A device is required unless a playback file
// is given.
func terminalDeviceID(
	cmd *cobra.Command,
	args []string,
	playbackFile, server, token string,
	skipVerify bool,
) (string, error) {
	flags := cmd.Flags()
	idFlag, err := flags.GetString(argDeviceID)
	if err != nil {
		return "", err
	}
	filters, err := flags.GetStringSlice(argInventoryFilter)
	if err != nil {
		return "", err
	}

	positional := ""
	if len(args) == 1 {
		positional = args[0]
	}

	sources := 0
	for _, set := range []bool{positional != "", idFlag != "", len(filters) > 0} {
		if set {
			sources++
		}
	}
	if sources > 1 {
		return "", errors.New("specify only one of DEVICE_ID, --id, or --filter")
	}
	if sources == 0 {
		if playbackFile != "" {
			return "", nil
		}
		return "", errors.New("no device specified")
	}

	if len(filters) > 0 {
		if err := validateInventoryFilters(filters); err != nil {
			return "", err
		}
		return resolveDeviceID(server, token, skipVerify, "", filters)
	}
	if idFlag != "" {
		return idFlag, nil
	}
	return positional, nil
}

// send the shell start message
func (c *TerminalCmd) startShell(client *deviceconnect.Client, termWidth, termHeight int) error {
	m := &ws.ProtoMsg{
		Header: ws.ProtoHdr{
			Proto:   ws.ProtoTypeShell,
			MsgType: wsshell.MessageTypeSpawnShell,
			Properties: map[string]any{
				"terminal_width":  termWidth,
				"terminal_height": termHeight,
			},
		},
	}
	if err := client.WriteMessage(m); err != nil {
		return err
	}
	return nil
}

// send the stop shell message
func (c *TerminalCmd) stopShell(client *deviceconnect.Client) error {
	m := &ws.ProtoMsg{
		Header: ws.ProtoHdr{
			Proto:     ws.ProtoTypeShell,
			MsgType:   wsshell.MessageTypeStopShell,
			SessionID: c.sessionID,
		},
	}
	if err := client.WriteMessage(m); err != nil {
		return err
	}
	return nil
}

// Run executes the command
func (c *TerminalCmd) Run() error {
	ctx, cancelContext := context.WithCancel(context.Background())
	defer cancelContext()

	// get the terminal width and height
	termWidth := defaultTermWidth
	termHeight := defaultTermHeight
	termID := int(os.Stdout.Fd())

	// when playing back, no further processing is required
	if c.playbackFile != "" {
		if _, err := os.Stat(c.playbackFile); err == nil {
			return c.playback(os.Stdout)
		} else {
			return err
		}
	}

	// start recording when applicable
	if _, err := os.Stat(c.recordFile); os.IsNotExist(err) {
		if len(c.recordFile) > 0 {
			c.recording = true
			go c.record()
		}
	} else {
		log.Err(fmt.Sprintf(
			"Can't create recording file: %s exists, refused to record.",
			c.recordFile,
		))
	}

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

	// set the terminal in raw mode
	if term.IsTerminal(termID) {
		termWidth, termHeight, err = term.GetSize(termID)
		if err != nil {
			return fmt.Errorf("unable to get the terminal size: %w", err)
		}

		fmt.Fprintln(os.Stderr, "Press CTRL+] to quit the session")

		oldState, err := term.MakeRaw(termID)
		if err != nil {
			return fmt.Errorf("unable to set the terminal in raw mode: %w", err)
		}
		defer func() {
			_ = term.Restore(termID, oldState)
		}()
	}

	// start the shell
	if err := c.startShell(client, termWidth, termHeight); err != nil {
		return err
	}

	// wait for CTRL+C, signals or stop
	c.runLoop(ctx, client, termID, termWidth, termHeight)

	// cancel the context
	cancelContext()

	// stop shell message
	if err := c.stopShell(client); err != nil {
		return err
	}

	// return the error message (if any)
	return c.err
}

// runLoop drives the interactive terminal session: it forwards stdin to the
// device and device output to stdout until the session ends or ctx is canceled.
func (c *TerminalCmd) runLoop(
	ctx context.Context,
	client *deviceconnect.Client,
	termID, termWidth, termHeight int,
) {
	// message channel
	msgChan := make(chan *ws.ProtoMsg)

	c.running = true
	go c.pipeStdin(msgChan, os.Stdin)
	go c.pipeStdout(msgChan, client, os.Stdout)

	// handle CTRL+C and signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// resize the terminal window
	go c.resizeTerminal(ctx, msgChan, termID, termWidth, termHeight)

	healthcheckTimeout := time.Now().Add(sessionMaxDuration)
	for c.running {
		select {
		case msg := <-msgChan:
			err := client.WriteMessage(msg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				c.running = false
			}
		case healthcheckInterval := <-c.healthcheck:
			healthcheckTimeout = time.Now().Add(time.Duration(healthcheckInterval) * time.Second)
		case <-time.After(time.Until(healthcheckTimeout)):
			_ = c.stopShell(client)
			c.err = errors.New("health check failed, connection with the device lost")
			c.running = false
		case <-quit:
			c.running = false
		case <-c.stop:
			c.running = false
		}
	}
}

func (c *TerminalCmd) resizeTerminal(
	ctx context.Context,
	msgChan chan *ws.ProtoMsg,
	termID int,
	termWidth int,
	termHeight int,
) {
	resize := make(chan os.Signal, 1)
	stopResize := notifyTerminalResize(ctx, resize)
	defer stopResize()

	for {
		select {
		case <-ctx.Done():
			return
		case <-resize:
			newTermWidth, newTermHeight, _ := term.GetSize(termID)
			if newTermWidth != termWidth || newTermHeight != termHeight {
				termWidth = newTermWidth
				termHeight = newTermHeight
				m := &ws.ProtoMsg{
					Header: ws.ProtoHdr{
						Proto:   ws.ProtoTypeShell,
						MsgType: wsshell.MessageTypeResizeShell,
						Properties: map[string]any{
							"terminal_width":  termWidth,
							"terminal_height": termHeight,
						},
					},
				}
				msgChan <- m
			}
		}
	}
}

func (c *TerminalCmd) Stop() {
	c.running = false
	c.stop <- struct{}{}
	if c.recording {
		c.stopRecording <- true
	}
}

func (c *TerminalCmd) pipeStdin(msgChan chan *ws.ProtoMsg, r io.Reader) {
	s := bufio.NewReader(r)
	for c.running {
		raw := make([]byte, stdinReadSize)
		n, err := s.Read(raw)
		if err != nil {
			if c.running {
				if err != io.EOF {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
				}
			} else {
				c.Stop()
			}
			break
		}
		// CTRL+] terminates the session
		if raw[0] == terminalQuitChar {
			c.Stop()
			return
		}

		m := &ws.ProtoMsg{
			Header: ws.ProtoHdr{
				Proto:     ws.ProtoTypeShell,
				MsgType:   wsshell.MessageTypeShellCommand,
				SessionID: c.sessionID,
			},
			Body: raw[:n],
		}
		msgChan <- m
	}
}

func (c *TerminalCmd) pipeStdout(
	msgChan chan *ws.ProtoMsg,
	client *deviceconnect.Client,
	w io.Writer,
) {
	for c.running {
		m, err := client.ReadMessage()
		if err != nil {
			if c.running {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			} else {
				c.Stop()
			}
			break
		}
		if m.Header.Proto == ws.ProtoTypeShell &&
			m.Header.MsgType == wsshell.MessageTypeShellCommand {
			if _, err := w.Write(m.Body); err != nil {
				break
			}
			if c.recording {
				c.terminalOutputChan <- m.Body
			}
		} else if m.Header.Proto == ws.ProtoTypeShell &&
			m.Header.MsgType == wsshell.MessageTypePingShell {
			if healthcheckTimeout, ok := m.Header.Properties["timeout"].(int64); ok &&
				healthcheckTimeout > 0 {
				c.healthcheck <- int(healthcheckTimeout)
			}
			m := &ws.ProtoMsg{
				Header: ws.ProtoHdr{
					Proto:     ws.ProtoTypeShell,
					MsgType:   wsshell.MessageTypePongShell,
					SessionID: c.sessionID,
				},
			}
			msgChan <- m
		} else if m.Header.Proto == ws.ProtoTypeShell &&
			m.Header.MsgType == wsshell.MessageTypeSpawnShell {
			status, ok := m.Header.Properties["status"].(int64)
			if ok && status == int64(wsshell.ErrorMessage) {
				c.err = fmt.Errorf("unable to start the shell: %s", string(m.Body))
				c.Stop()
			} else {
				c.sessionID = string(m.Header.SessionID)
			}
		} else if m.Header.Proto == ws.ProtoTypeShell &&
			m.Header.MsgType == wsshell.MessageTypeStopShell {
			c.Stop()
			break
		}
	}
}
