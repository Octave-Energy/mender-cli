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

package deployments

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/mendersoftware/mender-cli/client"
	"github.com/mendersoftware/mender-cli/log"
)

const (
	deploymentsListURL  = "/api/management/v2/deployments/deployments"
	deploymentURL       = "/api/management/v1/deployments/deployments"
	deploymentsPageSize = 500
)

type deploymentData struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ArtifactName string    `json:"artifact_name"`
	Status       string    `json:"status"`
	Type         string    `json:"type"`
	Created      time.Time `json:"created"`
	Finished     time.Time `json:"finished"`
	DeviceCount  int       `json:"device_count"`
	Groups       []string  `json:"groups"`
	Artifacts    []string  `json:"artifacts"`
	Statistics   *struct {
		Status    statsData `json:"status"`
		TotalSize int       `json:"total_size"`
	} `json:"statistics"`
	Filter *deploymentFilter `json:"filter"`
}

type deploymentFilter struct {
	Terms []filterTerm `json:"terms"`
}

type filterTerm struct {
	Scope     string          `json:"scope"`
	Attribute string          `json:"attribute"`
	Type      string          `json:"type"`
	Value     json.RawMessage `json:"value"`
}

type statsData struct {
	Success               int `json:"success"`
	Pending               int `json:"pending"`
	Downloading           int `json:"downloading"`
	Rebooting             int `json:"rebooting"`
	Installing            int `json:"installing"`
	Failure               int `json:"failure"`
	NoArtifact            int `json:"noartifact"`
	ArtifactTooBig        int `json:"artifact_too_big"`
	AlreadyInstalled      int `json:"already-installed"`
	Aborted               int `json:"aborted"`
	PauseBeforeInstalling int `json:"pause_before_installing"`
	PauseBeforeRebooting  int `json:"pause_before_rebooting"`
	PauseBeforeCommitting int `json:"pause_before_committing"`
}

type deploymentDevice struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	DeviceType string    `json:"device_type"`
	Started    time.Time `json:"started"`
	Finished   time.Time `json:"finished"`
	State      string    `json:"state"`
	Substate   string    `json:"substate"`
	Log        bool      `json:"log"`
	Image      *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"image"`
}

// ListDeployments fetches every deployment matching the given filters,
// following pagination transparently, then renders them at the given detail
// level. When raw is true the merged JSON array is written verbatim.
func (c *Client) ListDeployments(token string, detailLevel int, filters url.Values, raw bool) error {
	if detailLevel > 3 || detailLevel < 0 {
		return fmt.Errorf("invalid deployments detail")
	}

	merged, err := c.fetchAllDeployments(token, filters)
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
		var dep deploymentData
		if err := json.Unmarshal(item, &dep); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		listDeployment(c.output, dep, detailLevel)
	}
	return nil
}

// SearchDeployments fetches every deployment matching the given server-side
// filters, then keeps only those whose declared targeting matches the given
// group or device id (exactly one of group/deviceID must be non-empty). The
// matched deployments are rendered at the given detail level, or written as a
// raw JSON array when raw is true.
func (c *Client) SearchDeployments(
	token string, detailLevel int, filters url.Values, group, deviceID string, raw bool,
) error {
	if detailLevel > 3 || detailLevel < 0 {
		return fmt.Errorf("invalid deployments detail")
	}

	merged, err := c.fetchAllDeployments(token, filters)
	if err != nil {
		return err
	}

	matched := []json.RawMessage{}
	for _, item := range merged {
		var dep deploymentData
		if err := json.Unmarshal(item, &dep); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		var keep bool
		switch {
		case group != "":
			keep = deploymentMatchesGroup(dep, group)
		case deviceID != "":
			keep = deploymentMatchesDevice(dep, deviceID)
		}
		if keep {
			matched = append(matched, item)
		}
	}

	if raw {
		body, err := json.Marshal(matched)
		if err != nil {
			return fmt.Errorf("encode merged response: %w", err)
		}
		if _, err := c.output.Write(body); err != nil {
			return fmt.Errorf("error writing response body: %w", err)
		}
		return nil
	}

	for _, item := range matched {
		var dep deploymentData
		if err := json.Unmarshal(item, &dep); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		listDeployment(c.output, dep, detailLevel)
	}
	return nil
}

// deploymentMatchesGroup reports whether the deployment declares targeting of
// the given group, either via its groups[] list or a group filter term.
func deploymentMatchesGroup(d deploymentData, group string) bool {
	for _, g := range d.Groups {
		if g == group {
			return true
		}
	}
	if d.Filter != nil {
		for _, t := range d.Filter.Terms {
			if t.Attribute == "group" && termMatchesValue(t.Value, group) {
				return true
			}
		}
	}
	return false
}

// deploymentMatchesDevice reports whether the deployment declares targeting of
// the given device id via an "id" filter term.
func deploymentMatchesDevice(d deploymentData, deviceID string) bool {
	if d.Filter != nil {
		for _, t := range d.Filter.Terms {
			if t.Attribute == "id" && termMatchesValue(t.Value, deviceID) {
				return true
			}
		}
	}
	return false
}

// termMatchesValue reports whether the JSON-encoded filter value (a string or
// an array of strings) contains the wanted value.
func termMatchesValue(v json.RawMessage, want string) bool {
	if len(v) == 0 {
		return false
	}
	var single string
	if err := json.Unmarshal(v, &single); err == nil {
		return single == want
	}
	var arr []string
	if err := json.Unmarshal(v, &arr); err == nil {
		for _, e := range arr {
			if e == want {
				return true
			}
		}
	}
	return false
}

func (c *Client) fetchAllDeployments(token string, filters url.Values) ([]json.RawMessage, error) {
	merged := []json.RawMessage{}
	for page := 1; ; page++ {
		q := url.Values{}
		for key, vals := range filters {
			for _, v := range vals {
				q.Add(key, v)
			}
		}
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(deploymentsPageSize))

		batch, err := c.getJSONArray(token, client.JoinURL(c.url, deploymentsListURL), q)
		if err != nil {
			return nil, err
		}
		merged = append(merged, batch...)

		// A short page means we've reached the end.
		if len(batch) < deploymentsPageSize {
			break
		}
	}
	return merged, nil
}

// CountDeployments returns the total number of deployments matching the given
// filters, read from the X-Total-Count response header. It requests a single
// result to minimize transfer; any page/per_page in filters is overridden.
func (c *Client) CountDeployments(token string, filters url.Values) (int, error) {
	q := url.Values{}
	for key, vals := range filters {
		if key == "page" || key == "per_page" {
			continue
		}
		for _, v := range vals {
			q.Add(key, v)
		}
	}
	q.Set("page", "1")
	q.Set("per_page", "1")

	baseURL := client.JoinURL(c.url, deploymentsListURL)
	req, err := http.NewRequest(http.MethodGet, baseURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.URL.RawQuery = q.Encode()

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

	total := rsp.Header.Get("X-Total-Count")
	if total == "" {
		return 0, fmt.Errorf("server did not return an X-Total-Count header")
	}
	count, err := strconv.Atoi(total)
	if err != nil {
		return 0, fmt.Errorf("invalid X-Total-Count %q: %w", total, err)
	}
	return count, nil
}

// GetDeployment fetches a single deployment by id and renders it at the given
// detail level, or writes the raw JSON when raw is true.
func (c *Client) GetDeployment(token, id string, detailLevel int, raw bool) error {
	if detailLevel > 3 || detailLevel < 0 {
		return fmt.Errorf("invalid deployments detail")
	}

	depURL := client.JoinURL(c.url, deploymentURL+"/"+url.PathEscape(id))
	body, err := c.getBody(token, depURL, fmt.Sprintf("deployment %q not found", id))
	if err != nil {
		return err
	}

	if raw {
		if _, err := c.output.Write(body); err != nil {
			return fmt.Errorf("error writing response body: %w", err)
		}
		return nil
	}

	var dep deploymentData
	if err := json.Unmarshal(body, &dep); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	listDeployment(c.output, dep, detailLevel)
	return nil
}

// DeploymentStatistics fetches the per-status device counts for a deployment.
func (c *Client) DeploymentStatistics(token, id string, raw bool) error {
	statsURL := client.JoinURL(c.url, deploymentURL+"/"+url.PathEscape(id)+"/statistics")
	body, err := c.getBody(token, statsURL, fmt.Sprintf("deployment %q not found", id))
	if err != nil {
		return err
	}

	if raw {
		if _, err := c.output.Write(body); err != nil {
			return fmt.Errorf("error writing response body: %w", err)
		}
		return nil
	}

	var stats statsData
	if err := json.Unmarshal(body, &stats); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	printStatistics(c.output, stats)
	return nil
}

// ListDeploymentDevices fetches the devices (and their per-device status) of a
// deployment, following pagination transparently. When status is non-empty it
// filters by the device's deployment status.
func (c *Client) ListDeploymentDevices(
	token, id string, detailLevel int, status string, raw bool,
) error {
	if detailLevel > 3 || detailLevel < 0 {
		return fmt.Errorf("invalid deployments detail")
	}

	baseURL := client.JoinURL(c.url, deploymentURL+"/"+url.PathEscape(id)+"/devices/list")
	merged := []json.RawMessage{}
	for page := 1; ; page++ {
		q := url.Values{}
		if status != "" {
			q.Set("status", status)
		}
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(deploymentsPageSize))

		batch, err := c.getJSONArray(token, baseURL, q)
		if err != nil {
			return err
		}
		merged = append(merged, batch...)
		if len(batch) < deploymentsPageSize {
			break
		}
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
		var dev deploymentDevice
		if err := json.Unmarshal(item, &dev); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		listDeploymentDevice(c.output, dev, detailLevel)
	}
	return nil
}

// DeploymentDeviceLog fetches the deployment log of a single device (text/plain)
// and writes it verbatim.
func (c *Client) DeploymentDeviceLog(token, id, deviceID string) error {
	logURL := client.JoinURL(
		c.url,
		deploymentURL+"/"+url.PathEscape(id)+"/devices/"+url.PathEscape(deviceID)+"/log",
	)
	req, err := http.NewRequest(http.MethodGet, logURL, nil)
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
		return fmt.Errorf("no deployment log found for device %q in deployment %q", deviceID, id)
	}
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s request failed with status %d",
			req.URL.RequestURI(), rsp.StatusCode)
	}

	if _, err := io.Copy(c.output, rsp.Body); err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}
	return nil
}

// getJSONArray performs a GET expecting a JSON array body and returns its
// elements as raw messages.
func (c *Client) getJSONArray(token, baseURL string, q url.Values) ([]json.RawMessage, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL, nil)
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

// getBody performs a GET and returns the response body, mapping 404 to notFound.
func (c *Client) getBody(token, fullURL, notFound string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

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
	if rsp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s", notFound)
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s request failed with status %d",
			req.URL.RequestURI(), rsp.StatusCode)
	}
	return io.ReadAll(rsp.Body)
}

func listDeployment(out io.Writer, d deploymentData, detailLevel int) {
	fmt.Fprintf(out, "ID: %s\n", d.ID)
	fmt.Fprintf(out, "Name: %s\n", d.Name)
	fmt.Fprintf(out, "Status: %s\n", d.Status)
	if detailLevel >= 1 {
		fmt.Fprintf(out, "Artifact name: %s\n", d.ArtifactName)
		if d.Type != "" {
			fmt.Fprintf(out, "Type: %s\n", d.Type)
		}
		if !d.Created.IsZero() {
			fmt.Fprintf(out, "Created: %s\n", d.Created.Format(time.RFC3339))
		}
		if !d.Finished.IsZero() {
			fmt.Fprintf(out, "Finished: %s\n", d.Finished.Format(time.RFC3339))
		}
		fmt.Fprintf(out, "Device count: %d\n", d.DeviceCount)
		if len(d.Groups) > 0 {
			fmt.Fprintf(out, "Groups: %v\n", d.Groups)
		}
	}
	if detailLevel >= 2 {
		if len(d.Artifacts) > 0 {
			fmt.Fprintf(out, "Artifacts: %v\n", d.Artifacts)
		}
		if d.Statistics != nil {
			fmt.Fprintln(out, "Statistics:")
			printStatisticsIndented(out, d.Statistics.Status)
		}
	}
	fmt.Fprintf(
		out, "--------------------------------------------------------------------------------\n",
	)
}

func listDeploymentDevice(out io.Writer, d deploymentDevice, detailLevel int) {
	fmt.Fprintf(out, "ID: %s\n", d.ID)
	fmt.Fprintf(out, "Status: %s\n", d.Status)
	if detailLevel >= 1 {
		if d.DeviceType != "" {
			fmt.Fprintf(out, "Device type: %s\n", d.DeviceType)
		}
		if !d.Started.IsZero() {
			fmt.Fprintf(out, "Started: %s\n", d.Started.Format(time.RFC3339))
		}
		if !d.Finished.IsZero() {
			fmt.Fprintf(out, "Finished: %s\n", d.Finished.Format(time.RFC3339))
		}
		fmt.Fprintf(out, "Log available: %t\n", d.Log)
	}
	if detailLevel >= 2 {
		if d.State != "" {
			fmt.Fprintf(out, "State: %s\n", d.State)
		}
		if d.Substate != "" {
			fmt.Fprintf(out, "Substate: %s\n", d.Substate)
		}
		if d.Image != nil && (d.Image.ID != "" || d.Image.Name != "") {
			fmt.Fprintf(out, "Artifact: %s (%s)\n", d.Image.Name, d.Image.ID)
		}
	}
	fmt.Fprintf(
		out, "--------------------------------------------------------------------------------\n",
	)
}

func printStatistics(out io.Writer, s statsData) {
	printStatLine(out, "", s)
}

func printStatisticsIndented(out io.Writer, s statsData) {
	printStatLine(out, "  ", s)
}

func printStatLine(out io.Writer, indent string, s statsData) {
	fmt.Fprintf(out, "%sSuccess: %d\n", indent, s.Success)
	fmt.Fprintf(out, "%sFailure: %d\n", indent, s.Failure)
	fmt.Fprintf(out, "%sPending: %d\n", indent, s.Pending)
	fmt.Fprintf(out, "%sDownloading: %d\n", indent, s.Downloading)
	fmt.Fprintf(out, "%sInstalling: %d\n", indent, s.Installing)
	fmt.Fprintf(out, "%sRebooting: %d\n", indent, s.Rebooting)
	fmt.Fprintf(out, "%sNo artifact: %d\n", indent, s.NoArtifact)
	fmt.Fprintf(out, "%sArtifact too big: %d\n", indent, s.ArtifactTooBig)
	fmt.Fprintf(out, "%sAlready installed: %d\n", indent, s.AlreadyInstalled)
	fmt.Fprintf(out, "%sAborted: %d\n", indent, s.Aborted)
	fmt.Fprintf(out, "%sPause before installing: %d\n", indent, s.PauseBeforeInstalling)
	fmt.Fprintf(out, "%sPause before rebooting: %d\n", indent, s.PauseBeforeRebooting)
	fmt.Fprintf(out, "%sPause before committing: %d\n", indent, s.PauseBeforeCommitting)
}
