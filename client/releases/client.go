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

// Package releases provides a client for the Mender deployments releases API,
// used to list releases (groups of artifacts sharing a release name).
package releases

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/mendersoftware/mender-cli/client"
	"github.com/mendersoftware/mender-cli/log"
)

const (
	releasesListURL = "/api/management/v2/deployments/deployments/releases"
	// autoPageSize is the per-page batch used when transparently fetching all
	// pages of the paginated release list. Pagination is abstracted away from
	// the end user, so this is purely an implementation detail.
	autoPageSize = 500
)

type releaseData struct {
	Name      string    `json:"name"`
	Modified  time.Time `json:"modified"`
	Tags      []string  `json:"tags"`
	Notes     string    `json:"notes"`
	Artifacts []struct {
		ID                    string   `json:"id"`
		Name                  string   `json:"name"`
		Description           string   `json:"description"`
		DeviceTypesCompatible []string `json:"device_types_compatible"`
		Signed                bool     `json:"signed"`
		Size                  int      `json:"size"`
		Updates               []struct {
			TypeInfo struct {
				Type string `json:"type"`
			} `json:"type_info"`
		} `json:"updates"`
	} `json:"artifacts"`
}

// Client talks to the Mender deployments releases API.
type Client struct {
	url             string
	releasesListURL string
	client          *http.Client
	output          io.Writer
}

// NewClient returns a releases API client for the given server URL. When
// skipVerify is true, TLS certificate verification is disabled.
func NewClient(url string, skipVerify bool) *Client {
	return &Client{
		url:             url,
		releasesListURL: client.JoinURL(url, releasesListURL),
		client:          client.NewHTTPClient(skipVerify),
		output:          os.Stdout,
	}
}

// ListReleases fetches every release matching the given filters, following
// pagination transparently, then renders them at the given detail level. When
// raw is true the merged JSON array is written verbatim. The filters values are
// passed through to the server as query parameters (e.g. name, tag,
// update_type, sort).
func (c *Client) ListReleases(token string, detailLevel int, filters url.Values, raw bool) error {
	if detailLevel > 3 || detailLevel < 0 {
		return fmt.Errorf("invalid releases detail")
	}

	merged, err := c.fetchAllReleases(token, filters)
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
		var rel releaseData
		if err := json.Unmarshal(item, &rel); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		listRelease(c.output, rel, detailLevel)
	}
	return nil
}

// GetRelease fetches a single release by name and renders it at the given
// detail level, or writes the raw JSON when raw is true.
func (c *Client) GetRelease(token, name string, detailLevel int, raw bool) error {
	if detailLevel > 3 || detailLevel < 0 {
		return fmt.Errorf("invalid releases detail")
	}

	releaseURL := client.JoinURL(c.url, releasesListURL+"/"+url.PathEscape(name))
	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
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
		return fmt.Errorf("release %q not found", name)
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

	var rel releaseData
	if err := json.Unmarshal(body, &rel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	listRelease(c.output, rel, detailLevel)
	return nil
}

// fetchAllReleases repeatedly GETs the paginated release list endpoint, merging
// every page into a single JSON array. Pagination is driven internally.
func (c *Client) fetchAllReleases(token string, filters url.Values) ([]json.RawMessage, error) {
	merged := []json.RawMessage{}
	for page := 1; ; page++ {
		q := url.Values{}
		for key, vals := range filters {
			for _, v := range vals {
				q.Add(key, v)
			}
		}
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(autoPageSize))

		batch, err := c.getReleasesPage(token, q)
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

func (c *Client) getReleasesPage(token string, q url.Values) ([]json.RawMessage, error) {
	req, err := http.NewRequest(http.MethodGet, c.releasesListURL, nil)
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

func listRelease(out io.Writer, r releaseData, detailLevel int) {
	fmt.Fprintf(out, "Name: %s\n", r.Name)
	fmt.Fprintf(out, "Artifacts: %d\n", len(r.Artifacts))
	if detailLevel >= 1 {
		if !r.Modified.IsZero() {
			fmt.Fprintf(out, "Modified: %s\n", r.Modified.Format(time.RFC3339))
		}
		if len(r.Tags) > 0 {
			fmt.Fprintf(out, "Tags: %v\n", r.Tags)
		}
		if r.Notes != "" {
			fmt.Fprintf(out, "Notes: %s\n", r.Notes)
		}
	}
	if detailLevel >= 2 {
		for i, a := range r.Artifacts {
			fmt.Fprintf(out, "Artifact[%d]:\n", i)
			fmt.Fprintf(out, "  ID: %s\n", a.ID)
			fmt.Fprintf(out, "  Name: %s\n", a.Name)
			if a.Description != "" {
				fmt.Fprintf(out, "  Description: %s\n", a.Description)
			}
			if len(a.DeviceTypesCompatible) > 0 {
				fmt.Fprintf(out, "  Compatible types: %v\n", a.DeviceTypesCompatible)
			}
			for _, u := range a.Updates {
				if u.TypeInfo.Type != "" {
					fmt.Fprintf(out, "  Update type: %s\n", u.TypeInfo.Type)
				}
			}
			if detailLevel >= 3 {
				fmt.Fprintf(out, "  Signed: %t\n", a.Signed)
				fmt.Fprintf(out, "  Size: %d\n", a.Size)
			}
		}
	}
	fmt.Fprintf(
		out, "--------------------------------------------------------------------------------\n",
	)
}
