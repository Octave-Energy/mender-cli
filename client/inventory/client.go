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

// Package inventory provides a client for the Mender inventory API. It
// transparently fetches all pages of paginated list endpoints and supports
// filtering and counting device inventories.
package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mendersoftware/mender-cli/client"
	"github.com/mendersoftware/mender-cli/log"
)

const (
	inventoryAPIPrefix       = "/api/management/v1/inventory"
	devicesInventoryListPath = inventoryAPIPrefix + "/devices"
	groupsListPath           = inventoryAPIPrefix + "/groups"
	requestTimeout           = 60 * time.Second
	// autoPageSize is the per-page batch used when transparently fetching all
	// pages of a paginated list endpoint. Pagination is abstracted away from
	// the end user, so this is purely an implementation detail.
	autoPageSize = 500
)

// ListResponse is the body of a successful GET plus optional pagination headers.
type ListResponse struct {
	Body       []byte
	TotalCount string
	Link       string
	// ETag is the device object's current ETag (only meaningful for the
	// single-device inventory endpoint, and only reflects the 'tags' scope).
	ETag string
}

// Tag is an inventory attribute in the "tags" scope.
type Tag struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// Client talks to the Mender inventory API.
type Client struct {
	server string
	client *http.Client
}

// NewClient returns an inventory API client for the given server URL. When
// skipVerify is true, TLS certificate verification is disabled.
func NewClient(serverURL string, skipVerify bool) *Client {
	return &Client{
		server: serverURL,
		client: client.NewHTTPClient(skipVerify),
	}
}

func (c *Client) devicesListURL() string {
	return client.JoinURL(c.server, devicesInventoryListPath)
}

// ListDeviceInventories performs GET /api/management/v1/inventory/devices with
// the given query, transparently fetching every page and merging the results
// into a single JSON array.
func (c *Client) ListDeviceInventories(token string, q url.Values) (*ListResponse, error) {
	u, err := url.Parse(c.devicesListURL())
	if err != nil {
		return nil, err
	}
	return c.fetchAllPages(token, u.String(), u.Path, q)
}

// CountDeviceInventories returns the total number of devices matching the given
// query, read from the X-Total-Count response header. It requests a single
// result to minimize transfer; any page/per_page in q is overridden.
func (c *Client) CountDeviceInventories(token string, q url.Values) (int, error) {
	u, err := url.Parse(c.devicesListURL())
	if err != nil {
		return 0, err
	}
	cq := withoutPagination(q)
	cq.Set("page", "1")
	cq.Set("per_page", "1")
	u.RawQuery = cq.Encode()

	res, err := c.doGetExpectOK(token, u.String(), u.Path)
	if err != nil {
		return 0, err
	}
	if res.TotalCount == "" {
		return 0, fmt.Errorf("server did not return an X-Total-Count header")
	}
	count, err := strconv.Atoi(res.TotalCount)
	if err != nil {
		return 0, fmt.Errorf("invalid X-Total-Count %q: %w", res.TotalCount, err)
	}
	return count, nil
}

// ListGroups performs GET /api/management/v1/inventory/groups.
func (c *Client) ListGroups(token string, status string) (*ListResponse, error) {
	u, err := url.Parse(client.JoinURL(c.server, groupsListPath))
	if err != nil {
		return nil, err
	}
	if status != "" {
		q := url.Values{"status": {status}}
		u.RawQuery = q.Encode()
	}
	return c.doGetExpectOK(token, u.String(), u.Path)
}

// fetchAllPages repeatedly GETs a paginated list endpoint (which returns a JSON
// array body and an X-Total-Count header), merging every page into a single
// JSON array. Any page/per_page values in q are ignored: pagination is driven
// internally. The returned TotalCount reflects the server-reported total.
func (c *Client) fetchAllPages(token, baseURL, pathForErrors string, q url.Values) (*ListResponse, error) {
	merged := []json.RawMessage{}
	var totalCount string

	for page := 1; ; page++ {
		pq := withoutPagination(q)
		pq.Set("page", strconv.Itoa(page))
		pq.Set("per_page", strconv.Itoa(autoPageSize))

		u, err := url.Parse(baseURL)
		if err != nil {
			return nil, err
		}
		u.RawQuery = pq.Encode()

		res, err := c.doGetExpectOK(token, u.String(), pathForErrors)
		if err != nil {
			return nil, err
		}
		if res.TotalCount != "" {
			totalCount = res.TotalCount
		}

		var batch []json.RawMessage
		if err := json.Unmarshal(res.Body, &batch); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		merged = append(merged, batch...)

		// A short page means we've reached the end.
		if len(batch) < autoPageSize {
			break
		}
		// Otherwise stop early if the server-reported total is reached.
		if total, errc := strconv.Atoi(totalCount); errc == nil && len(merged) >= total {
			break
		}
	}

	body, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("encode merged response: %w", err)
	}
	return &ListResponse{Body: body, TotalCount: totalCount}, nil
}

// withoutPagination returns a copy of q with any page/per_page parameters
// removed, so pagination can be driven internally.
func withoutPagination(q url.Values) url.Values {
	out := url.Values{}
	for k, vs := range q {
		if k == "page" || k == "per_page" {
			continue
		}
		for _, v := range vs {
			out.Add(k, v)
		}
	}
	return out
}

// GetDeviceInventory performs GET /api/management/v1/inventory/devices/{id}.
func (c *Client) GetDeviceInventory(token, deviceID string) (*ListResponse, error) {
	path := fmt.Sprintf("%s/devices/%s", inventoryAPIPrefix, url.PathEscape(deviceID))
	u, err := url.Parse(client.JoinURL(c.server, path))
	if err != nil {
		return nil, err
	}
	return c.doGetExpectOK(token, u.String(), u.Path)
}

func (c *Client) doGetExpectOK(token, fullURL, pathForErrors string) (*ListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	reqDump, err := httputil.DumpRequest(req, false)
	if err != nil {
		return nil, err
	}
	log.Verbf("sending request: \n%s", string(reqDump))

	rsp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET request failed: %w", err)
	}
	defer rsp.Body.Close()

	log.Verbf("response: status=%d X-Total-Count=%q Link=%q",
		rsp.StatusCode, rsp.Header.Get("X-Total-Count"), rsp.Header.Get("Link"))

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read response body: %w", err)
	}

	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s failed with status %d", pathForErrors, rsp.StatusCode)
	}

	return &ListResponse{
		Body:       body,
		TotalCount: rsp.Header.Get("X-Total-Count"),
		Link:       rsp.Header.Get("Link"),
		ETag:       rsp.Header.Get("ETag"),
	}, nil
}

func (c *Client) deviceTagsURL(deviceID string) string {
	path := fmt.Sprintf("%s/devices/%s/tags", inventoryAPIPrefix, url.PathEscape(deviceID))
	return client.JoinURL(c.server, path)
}

// ReplaceDeviceTags performs PUT /api/management/v1/inventory/devices/{id}/tags,
// replacing the device's entire set of tags with the given list (tags not
// present are removed). When etag is non-empty it is sent as If-Match for
// optimistic concurrency control.
func (c *Client) ReplaceDeviceTags(token, deviceID, etag string, tags []Tag) error {
	return c.writeDeviceTags(token, http.MethodPut, deviceID, etag, tags)
}

// UpsertDeviceTags performs PATCH /api/management/v1/inventory/devices/{id}/tags,
// adding or overwriting the given tags without touching the others. When etag
// is non-empty it is sent as If-Match.
func (c *Client) UpsertDeviceTags(token, deviceID, etag string, tags []Tag) error {
	return c.writeDeviceTags(token, http.MethodPatch, deviceID, etag, tags)
}

func (c *Client) writeDeviceTags(token, method, deviceID, etag string, tags []Tag) error {
	// The API requires a JSON array; ensure an empty set serializes as [] and
	// not null (e.g. when deleting the last remaining tag via PUT).
	if tags == nil {
		tags = []Tag{}
	}
	payload, err := json.Marshal(tags)
	if err != nil {
		return fmt.Errorf("encode tags: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx, method, c.deviceTagsURL(deviceID), bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}

	rsp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", method, err)
	}
	defer rsp.Body.Close()

	switch rsp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusPreconditionFailed:
		return fmt.Errorf("device tags changed concurrently (ETag mismatch); please retry")
	default:
		body, _ := io.ReadAll(rsp.Body)
		return fmt.Errorf(
			"%s %s/devices/%s/tags failed with status %d: %s",
			method, inventoryAPIPrefix, deviceID, rsp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}
}
