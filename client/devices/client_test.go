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

package devices

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListDevices(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]deviceData{{
			ID:     "1234",
			Status: "accepted",
		}})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := NewClient(srv.URL, true)
	client.output = &buf
	err := client.ListDevices("token", 0, "", false)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if !strings.Contains(buf.String(), "1234") {
		t.Errorf("Output does not contain device ID: output follows")
		t.Error(buf.String())
		t.FailNow()
	}
	if !strings.Contains(buf.String(), "accepted") {
		t.Errorf("Output does not contain device status: output follows")
		t.Error(buf.String())
		t.FailNow()
	}
}

func TestCountDevices(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/devices/count") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]int{"count": 42})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, true)
	count, err := client.CountDevices("token", "pending")
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if count != 42 {
		t.Errorf("Expected count 42, got %d", count)
	}
}

func TestGetDevice(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/devices/1234") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(deviceData{
			ID:     "1234",
			Status: "accepted",
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := NewClient(srv.URL, true)
	client.output = &buf
	err := client.GetDevice("token", "1234", 0, false)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if !strings.Contains(buf.String(), "1234") {
		t.Errorf("Output does not contain device ID: output follows")
		t.Error(buf.String())
		t.FailNow()
	}
	if !strings.Contains(buf.String(), "accepted") {
		t.Errorf("Output does not contain device status: output follows")
		t.Error(buf.String())
		t.FailNow()
	}
}
