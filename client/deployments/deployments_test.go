// Copyright 2025 Northern.tech AS
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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListDeployments(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]deploymentData{{
			ID:     "dep-1",
			Name:   "production rollout",
			Status: "inprogress",
		}})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := NewClient(srv.URL, true)
	client.output = &buf
	err := client.ListDeployments("token", 1, nil, false)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if !strings.Contains(buf.String(), "dep-1") {
		t.Errorf("Output does not contain deployment id: output follows")
		t.Error(buf.String())
		t.FailNow()
	}
	if !strings.Contains(buf.String(), "production rollout") {
		t.Errorf("Output does not contain deployment name: output follows")
		t.Error(buf.String())
		t.FailNow()
	}
}

func TestSearchDeploymentsByGroup(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]deploymentData{
			{ID: "dep-groups", Name: "via groups list", Groups: []string{"edgebox-PROD"}},
			{ID: "dep-term", Name: "via filter term", Filter: &deploymentFilter{
				Terms: []filterTerm{{
					Scope:     "system",
					Attribute: "group",
					Type:      "$eq",
					Value:     json.RawMessage(`"edgebox-PROD"`),
				}},
			}},
			{ID: "dep-other", Name: "other group", Groups: []string{"edgebox-STAGING"}},
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := NewClient(srv.URL, true)
	client.output = &buf
	if err := client.SearchDeployments("token", 0, nil, "edgebox-PROD", "", false); err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	out := buf.String()
	if !strings.Contains(out, "dep-groups") || !strings.Contains(out, "dep-term") {
		t.Errorf("Expected both group-matched deployments in output:\n%s", out)
		t.FailNow()
	}
	if strings.Contains(out, "dep-other") {
		t.Errorf("Did not expect non-matching deployment in output:\n%s", out)
		t.FailNow()
	}
}

func TestSearchDeploymentsByDevice(t *testing.T) {
	t.Parallel()
	const target = "479daf01-d10e-477f-a0a1-a876313a9d1a"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]deploymentData{
			{ID: "dep-match", Name: "targets device", Filter: &deploymentFilter{
				Terms: []filterTerm{{
					Scope:     "identity",
					Attribute: "id",
					Type:      "$in",
					Value:     json.RawMessage(`["107d91b5-7085-460a-8ad6-b2163cabd1b9","` + target + `"]`),
				}},
			}},
			{ID: "dep-nomatch", Name: "targets other device", Filter: &deploymentFilter{
				Terms: []filterTerm{{
					Scope:     "identity",
					Attribute: "id",
					Type:      "$in",
					Value:     json.RawMessage(`["f3822894-0013-46e6-967c-8abf12c48805"]`),
				}},
			}},
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := NewClient(srv.URL, true)
	client.output = &buf
	if err := client.SearchDeployments("token", 0, nil, "", target, false); err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	out := buf.String()
	if !strings.Contains(out, "dep-match") {
		t.Errorf("Expected device-matched deployment in output:\n%s", out)
		t.FailNow()
	}
	if strings.Contains(out, "dep-nomatch") {
		t.Errorf("Did not expect non-matching deployment in output:\n%s", out)
		t.FailNow()
	}
}

func TestCountDeployments(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("per_page") != "1" {
			t.Errorf("expected per_page=1, got %q", r.URL.Query().Get("per_page"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "42")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]deploymentData{})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, true)
	count, err := client.CountDeployments("token", nil)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if count != 42 {
		t.Errorf("expected count 42, got %d", count)
	}
}

func TestCountDeploymentsMissingHeader(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]deploymentData{})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, true)
	if _, err := client.CountDeployments("token", nil); err == nil {
		t.Errorf("expected error when X-Total-Count is missing")
	}
}

func TestGetDeployment(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/deployments/dep-1") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(deploymentData{
			ID:     "dep-1",
			Name:   "production rollout",
			Status: "finished",
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := NewClient(srv.URL, true)
	client.output = &buf

	if err := client.GetDeployment("token", "dep-1", 1, false); err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if !strings.Contains(buf.String(), "dep-1") {
		t.Errorf("Output does not contain deployment id: output follows")
		t.Error(buf.String())
		t.FailNow()
	}

	err := client.GetDeployment("token", "missing", 1, false)
	if err == nil {
		t.Errorf("Expected not-found error for missing deployment")
		t.FailNow()
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Unexpected error: %s", err.Error())
	}
}

func TestDeploymentDeviceLog(t *testing.T) {
	t.Parallel()
	const logBody = "2025-01-01 00:00:00 +0000 UTC info: update successful\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/devices/dev-1/log") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(logBody))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := NewClient(srv.URL, true)
	client.output = &buf

	if err := client.DeploymentDeviceLog("token", "dep-1", "dev-1"); err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if buf.String() != logBody {
		t.Errorf("Log body not echoed verbatim: got %q", buf.String())
		t.FailNow()
	}

	err := client.DeploymentDeviceLog("token", "dep-1", "missing")
	if err == nil {
		t.Errorf("Expected not-found error for missing device log")
		t.FailNow()
	}
	if !strings.Contains(err.Error(), "no deployment log found") {
		t.Errorf("Unexpected error: %s", err.Error())
	}
}
