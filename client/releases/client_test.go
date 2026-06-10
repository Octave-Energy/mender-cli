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

package releases

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListReleases(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]releaseData{{
			Name: "my-app-v1.0.0",
			Tags: []string{"stable"},
		}})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := NewClient(srv.URL, true)
	client.output = &buf
	err := client.ListReleases("token", 1, nil, false)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if !strings.Contains(buf.String(), "my-app-v1.0.0") {
		t.Errorf("Output does not contain release name: output follows")
		t.Error(buf.String())
		t.FailNow()
	}
	if !strings.Contains(buf.String(), "stable") {
		t.Errorf("Output does not contain release tag: output follows")
		t.Error(buf.String())
		t.FailNow()
	}
}

func TestGetRelease(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/my-app-v1.0.0") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(releaseData{
			Name: "my-app-v1.0.0",
			Tags: []string{"stable"},
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := NewClient(srv.URL, true)
	client.output = &buf
	err := client.GetRelease("token", "my-app-v1.0.0", 1, false)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if !strings.Contains(buf.String(), "my-app-v1.0.0") {
		t.Errorf("Output does not contain release name: output follows")
		t.Error(buf.String())
		t.FailNow()
	}
}
