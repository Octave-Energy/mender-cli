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

// Package devices provides a client for the Mender device authentication
// management API, used to list devices and their authentication sets.
package devices

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"

	"github.com/mendersoftware/mender-cli/client"
	"github.com/mendersoftware/mender-cli/log"
)

type deviceData struct {
	ID           string `json:"id"`
	IdentityData struct {
		Mac string `json:"mac"`
		Sku string `json:"sku"`
		Sn  string `json:"sn"`
	} `json:"identity_data"`
	Status    string `json:"status"`
	CreatedTs string `json:"created_ts"`
	UpdatedTs string `json:"updated_ts"`
	AuthSets  []struct {
		ID           string `json:"id"`
		PubKey       string `json:"pubkey"`
		IdentityData struct {
			Mac string `json:"mac"`
			Sku string `json:"sku"`
			Sn  string `json:"sn"`
		} `json:"identity_data"`
		Status string `json:"status"`
		Ts     string `json:"ts"`
	} `json:"auth_sets"`
	Decommissioning bool `json:"decommissioning"`
}

const (
	devicesListURL = "/api/management/v2/devauth/devices"
	// autoPageSize is the per-page batch used when transparently fetching all
	// pages of the paginated device list. Pagination is abstracted away from
	// the end user, so this is purely an implementation detail.
	autoPageSize = 500
)

// Client talks to the Mender device authentication management API.
type Client struct {
	url            string
	devicesListURL string
	client         *http.Client
	output         io.Writer
}

// NewClient returns a device authentication API client for the given server
// URL. When skipVerify is true, TLS certificate verification is disabled.
func NewClient(url string, skipVerify bool) *Client {
	return &Client{
		url:            url,
		devicesListURL: client.JoinURL(url, devicesListURL),
		client:         client.NewHTTPClient(skipVerify),
		output:         os.Stdout,
	}
}

// ListDevices fetches every device (optionally filtered by authentication
// status), following pagination transparently, then renders them at the given
// detail level. When raw is true the merged JSON array is written verbatim.
func (c *Client) ListDevices(token string, detailLevel int, status string, raw bool) error {
	if detailLevel > 3 || detailLevel < 0 {
		return fmt.Errorf("invalid devices detail")
	}

	merged, err := c.fetchAllDevices(token, status)
	if err != nil {
		return err
	}

	if raw {
		body, err := json.Marshal(merged)
		if err != nil {
			return fmt.Errorf("encode merged response: %w", err)
		}
		if _, err := c.output.Write(body); err != nil {
			return fmt.Errorf("error writing response body: %w", err)
		}
		return nil
	}

	for _, item := range merged {
		var dev deviceData
		if err := json.Unmarshal(item, &dev); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		listDevice(c.output, dev, detailLevel)
	}
	return nil
}

// GetDevice fetches a single device by id and renders it at the given detail
// level, or writes the raw JSON when raw is true.
func (c *Client) GetDevice(token, id string, detailLevel int, raw bool) error {
	if detailLevel > 3 || detailLevel < 0 {
		return fmt.Errorf("invalid devices detail")
	}

	deviceURL := client.JoinURL(c.url, devicesListURL+"/"+url.PathEscape(id))
	req, err := http.NewRequest(http.MethodGet, deviceURL, nil)
	if err != nil {
		return fmt.Errorf("failed to prepare request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	reqDump, err := httputil.DumpRequest(req, false)
	if err != nil {
		return err
	}
	log.Verbf("sending request: \n%s", string(reqDump))

	rsp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("device %s not found", id)
	}
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s request failed with status %d",
			req.URL.RequestURI(), rsp.StatusCode)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if raw {
		if _, err := c.output.Write(body); err != nil {
			return fmt.Errorf("error writing response body: %w", err)
		}
		return nil
	}

	var dev deviceData
	if err := json.Unmarshal(body, &dev); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	listDevice(c.output, dev, detailLevel)
	return nil
}

// CountDevices returns the number of devices, optionally filtered by
// authentication status, without listing them.
func (c *Client) CountDevices(token, status string) (int, error) {
	countURL := client.JoinURL(c.url, devicesListURL+"/count")
	req, err := http.NewRequest(http.MethodGet, countURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	if status != "" {
		q := url.Values{}
		q.Set("status", status)
		req.URL.RawQuery = q.Encode()
	}

	reqDump, err := httputil.DumpRequest(req, false)
	if err != nil {
		return 0, err
	}
	log.Verbf("sending request: \n%s", string(reqDump))

	rsp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("GET %s request failed with status %d",
			req.URL.RequestURI(), rsp.StatusCode)
	}

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	return result.Count, nil
}

// fetchAllDevices repeatedly GETs the paginated device list endpoint, merging
// every page into a single JSON array. Pagination is driven internally.
func (c *Client) fetchAllDevices(token, status string) ([]json.RawMessage, error) {
	merged := []json.RawMessage{}
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(autoPageSize))
		if status != "" {
			q.Set("status", status)
		}

		batch, err := c.getDevicesPage(token, q)
		if err != nil {
			return nil, err
		}
		merged = append(merged, batch...)

		// A short page means we've reached the end.
		if len(batch) < autoPageSize {
			break
		}
	}
	return merged, nil
}

func (c *Client) getDevicesPage(token string, q url.Values) ([]json.RawMessage, error) {
	req, err := http.NewRequest(http.MethodGet, c.devicesListURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.URL.RawQuery = q.Encode()

	reqDump, err := httputil.DumpRequest(req, false)
	if err != nil {
		return nil, err
	}
	log.Verbf("sending request: \n%s", string(reqDump))

	rsp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s request failed with status %d",
			req.URL.RequestURI(), rsp.StatusCode)
	}

	var batch []json.RawMessage
	if err := json.NewDecoder(rsp.Body).Decode(&batch); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return batch, nil
}

func listDevice(out io.Writer, a deviceData, detailLevel int) {
	fmt.Fprintf(out, "ID: %s\n", a.ID)
	fmt.Fprintf(out, "Status: %s\n", a.Status)
	if detailLevel >= 1 {
		fmt.Println("IdentityData:")
		if a.IdentityData.Mac != "" {
			fmt.Fprintf(out, "  MAC address: %s\n", a.IdentityData.Mac)
		}
		if a.IdentityData.Sku != "" {
			fmt.Fprintf(out, "  Stock keeping unit: %s\n", a.IdentityData.Sku)
		}
		if a.IdentityData.Sn != "" {
			fmt.Fprintf(out, "  Serial number: %s\n", a.IdentityData.Sn)
		}
	}
	if detailLevel >= 1 {
		fmt.Fprintf(out, "CreatedTs: %s\n", a.CreatedTs)
		fmt.Fprintf(out, "UpdatedTs: %s\n", a.UpdatedTs)
		fmt.Fprintf(out, "Decommissioning: %t\n", a.Decommissioning)
	}
	if detailLevel >= 2 {
		for i, v := range a.AuthSets {
			fmt.Fprintf(out, "AuthSet[%d]:\n", i)
			fmt.Fprintf(out, "  ID: %s\n", v.ID)
			fmt.Fprintf(out, "  PubKey:\n%s", v.PubKey)
			fmt.Println("  IdentityData:")
			if v.IdentityData.Mac != "" {
				fmt.Fprintf(out, "    MAC address: %s\n", v.IdentityData.Mac)
			}
			if v.IdentityData.Sku != "" {
				fmt.Fprintf(out, "    Stock keeping unit: %s\n", v.IdentityData.Sku)
			}
			if v.IdentityData.Sn != "" {
				fmt.Fprintf(out, "    Serial number: %s\n", v.IdentityData.Sn)
			}
			fmt.Fprintf(out, "  Status: %s\n", v.Status)
			fmt.Fprintf(out, "  Ts: %s\n", v.Ts)
		}
	}

	fmt.Fprintf(
		out, "--------------------------------------------------------------------------------\n",
	)
}
