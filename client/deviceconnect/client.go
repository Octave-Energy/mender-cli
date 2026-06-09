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

// Package deviceconnect provides a client for the Mender deviceconnect API. It
// manages the websocket session used for remote terminal access, TCP/UDP
// port-forwarding and file upload/download to connected devices.
package deviceconnect

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mendersoftware/go-lib-micro/ws"
	"github.com/vmihailenco/msgpack"

	"github.com/mendersoftware/mender-cli/client"
	"github.com/mendersoftware/mender-cli/log"
)

const (
	// protocols
	httpProtocol = "http"
	wsProtocol   = "ws"

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 1 * time.Minute

	// deviceconnect API path
	devicePath        = "/api/management/v1/deviceconnect/devices/:deviceID"
	deviceConnectPath = "/api/management/v1/deviceconnect/devices/:deviceID/connect"

	// fileUploadURL API path
	fileUploadURL = "/api/management/v1/deviceconnect/"
)

// Client talks to the Mender deviceconnect API over both HTTP and a websocket
// connection, used for remote terminal, port-forwarding and file transfer.
type Client struct {
	url        string
	skipVerify bool
	conn       *websocket.Conn
	readMutex  *sync.Mutex
	writeMutex *sync.Mutex
	token      string
	client     *http.Client
}

// NewClient returns a deviceconnect API client for the given server URL using
// the provided JWT token. When skipVerify is true, TLS certificate
// verification is disabled.
func NewClient(url string, token string, skipVerify bool) *Client {
	return &Client{
		url:        url,
		token:      token,
		skipVerify: skipVerify,
		client:     client.NewHTTPClient(skipVerify),
		readMutex:  &sync.Mutex{},
		writeMutex: &sync.Mutex{},
	}
}

// Connect opens a websocket connection to the given device.
func (c *Client) Connect(deviceID string, token string) error {
	fmt.Fprintf(os.Stderr, "Connecting to the device %s...\n", deviceID)
	u, err := url.Parse(
		strings.TrimSuffix(
			c.url,
			"/",
		) + strings.Replace(
			deviceConnectPath,
			":deviceID",
			deviceID,
			1,
		),
	)
	if err != nil {
		return fmt.Errorf("unable to parse the server URL: %w", err)
	}
	u.Scheme = strings.Replace(u.Scheme, httpProtocol, wsProtocol, 1)

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.skipVerify,
		},
	}
	conn, rsp, err := dialer.Dial(u.String(), headers)
	if err != nil {
		return fmt.Errorf("unable to connect to the device: %w", err)
	}
	defer rsp.Body.Close()

	err = conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		return fmt.Errorf("unable to set the read deadline: %w", err)
	}

	c.conn = conn
	return nil
}

// GetDevice returns the device
func (c *Client) GetDevice(deviceID string) (*Device, error) {
	path := strings.Replace(devicePath, ":deviceID", deviceID, 1)
	body, err := client.DoGetRequest(c.token, client.JoinURL(c.url, path), c.client)
	if err != nil {
		return nil, err
	}

	var device Device
	err = json.Unmarshal(body, &device)
	if err != nil {
		return nil, err
	}
	return &device, nil
}

// PingPong handles the ping-pong connection health check
func (c *Client) PingPong(ctx context.Context) {
	pingPeriod := (pongWait * 9) / 10
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	c.conn.SetPongHandler(func(string) error {
		ticker.Reset(pingPeriod)
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	c.conn.SetPingHandler(func(msg string) error {
		ticker.Reset(pingPeriod)
		err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			return err
		}
		return c.conn.WriteControl(
			websocket.PongMessage,
			[]byte(msg),
			time.Now().Add(writeWait),
		)
	})

	for {
		select {
		case <-ticker.C:
			pongWaitString := strconv.Itoa(int(pongWait.Seconds()))
			_ = c.conn.WriteControl(
				websocket.PingMessage,
				[]byte(pongWaitString),
				time.Now().Add(writeWait),
			)

		case <-ctx.Done():
			return
		}
	}
}

// ReadMessage reads a Proto message from the websocket
func (c *Client) ReadMessage() (*ws.ProtoMsg, error) {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	m := &ws.ProtoMsg{}
	err = msgpack.Unmarshal(data, m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// WriteMessage writes a Proto message to the websocket
func (c *Client) WriteMessage(m *ws.ProtoMsg) error {
	data, err := msgpack.Marshal(m)
	if err != nil {
		return fmt.Errorf("unable to marshal the message from the websocket: %w", err)
	}
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return fmt.Errorf("unable to set the write deadline: %w", err)
	}
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("unable to write the message: %w", err)
	}
	return nil
}

// Close closes the connection
func (c *Client) Close() {
	c.conn.Close()
}

func NewFileTransferClient(url string, token string, skipVerify bool) *Client {
	return &Client{
		url:    url,
		token:  token,
		client: client.NewHTTPClient(skipVerify),
	}
}

type DeviceSpec struct {
	DeviceID   string
	DevicePath string
}

type DeviceConnectError struct {
	ErrorStr  string `json:"error"`
	RequestID string `json:"request_id"`
}

func (d *DeviceConnectError) Error() string {
	if d.ErrorStr != "" {
		if d.RequestID != "" {
			return fmt.Sprintf("Error: [%s] %s", d.RequestID, d.ErrorStr)
		}
		return fmt.Sprintf("Error: %s", d.ErrorStr)
	}
	return "No Error string returned from the server. This is unexpected behaviour"
}

func NewDeviceConnectError(errCode int, r io.Reader) *DeviceConnectError {
	body, err := io.ReadAll(r)
	if err != nil {
		return &DeviceConnectError{
			ErrorStr: fmt.Sprintf("Failed to upload the file. HTTP status code: %d", errCode),
		}
	}
	d := &DeviceConnectError{}
	if err = json.Unmarshal(body, d); err != nil {
		d.ErrorStr = string(body) // Just hope there is something sensible in the body
	}
	return d
}

func (c *Client) Upload(sourcePath string, deviceSpec *DeviceSpec) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	fi, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	log.Verbf("Uploading the file to %s\n", deviceSpec.DevicePath)
	if err = writer.WriteField("path", deviceSpec.DevicePath); err != nil {
		return err
	}
	part, err := writer.CreateFormFile("file", sourcePath)
	if err != nil {
		return err
	}
	if _, err = io.Copy(part, file); err != nil {
		return err
	}
	if err = writer.WriteField("mode", fmt.Sprintf("%o", fi.Mode())); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut,
		c.url+fileUploadURL+"devices/"+deviceSpec.DeviceID+"/upload",
		body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.token)

	reqDump, _ := httputil.DumpRequest(req, false)
	log.Verbf("sending request: \n%v", string(reqDump))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusBadRequest:
		log.Err("Error: Bad request\n")
	case http.StatusForbidden:
		log.Err("Error: You are not allowed to access the given resource\n")
	case http.StatusNotFound:
		log.Err("Error: Resource not found\n")
	case http.StatusConflict:
		log.Err("Error: Device not connected\n")
	case http.StatusInternalServerError:
		log.Errf("Error: Internal Server Error\n")
	default:
		log.Errf("Error: Received unexpected response code: %d\n",
			resp.StatusCode)
	}
	return NewDeviceConnectError(resp.StatusCode, resp.Body)
}

func (c *Client) Download(deviceSpec *DeviceSpec, sourcePath string) error {
	req, err := http.NewRequest(http.MethodGet,
		c.url+fileUploadURL+"devices/"+deviceSpec.DeviceID+"/download",
		nil,
	)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	q := req.URL.Query()
	q.Add("path", deviceSpec.DevicePath)
	req.URL.RawQuery = q.Encode()

	reqDump, _ := httputil.DumpRequest(req, false)
	log.Verbf("sending request: \n%v", string(reqDump))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	rspDump, _ := httputil.DumpResponse(resp, true)
	log.Verbf("Response: \n%v\n", string(rspDump))

	switch resp.StatusCode {
	case http.StatusOK:
		return c.downloadFile(sourcePath, resp)
	case http.StatusBadRequest:
		log.Err("Bad request\n")
	case http.StatusForbidden:
		log.Err("Forbidden")
	case http.StatusNotFound:
		log.Err("File not found on the device\n")
	case http.StatusConflict:
		log.Err("The device is not connected\n")
	case http.StatusInternalServerError:
		log.Err("Internal server error\n")
	default:
		log.Errf("Error: Received unexpected response code: %d\n",
			resp.StatusCode)
	}
	return NewDeviceConnectError(resp.StatusCode, resp.Body)
}

func (c *Client) downloadFile(localFileName string, resp *http.Response) error {
	path := resp.Header.Get("X-MEN-FILE-PATH")
	uid := resp.Header.Get("X-MEN-FILE-UID")
	gid := resp.Header.Get("X-MEN-FILE-GID")
	mode := resp.Header.Get("X-MEN-FILE-MODE")
	if mode == "" {
		return errors.New("missing X-MEN-FILE-MODE header")
	}
	modeo, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return err
	}
	_size := resp.Header.Get("X-MEN-FILE-SIZE")
	size, err := strconv.ParseInt(_size, 10, 64)
	if err != nil {
		return fmt.Errorf("no proper size given for the file: %s", _size)
	}
	var n int64
	file, err := os.OpenFile(localFileName, os.O_CREATE|os.O_WRONLY, os.FileMode(modeo))
	if err != nil {
		log.Errf("Failed to create the file %s locally\n", path)
		return err
	}
	defer file.Close()

	if resp.Header.Get("Content-Type") != "application/octet-stream" {
		return fmt.Errorf("unexpected Content-Type header: %s", resp.Header.Get("Content-Type"))
	}
	n, err = io.Copy(file, resp.Body)
	log.Verbf("wrote: %d\n", n)
	if err != nil {
		return err
	}
	if n != size {
		return errors.New(
			"downloaded file does not match the expected length in 'X-MEN-FILE-SIZE'",
		)
	}
	// Set the proper permissions and {G,U}ID's if present
	if uid != "" && gid != "" {
		uidi, err := strconv.Atoi(uid)
		if err != nil {
			return err
		}
		gidi, err := strconv.Atoi(gid)
		if err != nil {
			return err
		}
		err = os.Chown(file.Name(), uidi, gidi)
		if err != nil {
			return err
		}
	}
	return nil
}
