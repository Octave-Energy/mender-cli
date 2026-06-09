// Copyright 2026 Northern.tech AS
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

package inventory

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

// makePage returns a JSON array of n objects of the form {"id":"<prefix>-<i>"}.
func makePage(prefix string, n int) []byte {
	items := make([]map[string]string, n)
	for i := 0; i < n; i++ {
		items[i] = map[string]string{"id": fmt.Sprintf("%s-%d", prefix, i)}
	}
	b, _ := json.Marshal(items)
	return b
}

func TestListDeviceInventoriesMergesPages(t *testing.T) {
	t.Parallel()

	total := autoPageSize + 3 // forces a second (short) page
	var requestedPages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		requestedPages = append(requestedPages, page)
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			_, _ = w.Write(makePage("p1", autoPageSize))
		case "2":
			_, _ = w.Write(makePage("p2", 3))
		default:
			_, _ = w.Write([]byte("[]"))
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, true)
	res, err := c.ListDeviceInventories("tok", url.Values{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var merged []map[string]string
	if err := json.Unmarshal(res.Body, &merged); err != nil {
		t.Fatalf("merged body is not a JSON array: %v", err)
	}
	if len(merged) != total {
		t.Fatalf("merged %d devices, want %d", len(merged), total)
	}
	if res.TotalCount != strconv.Itoa(total) {
		t.Fatalf("TotalCount = %q, want %q", res.TotalCount, strconv.Itoa(total))
	}
	if len(requestedPages) != 2 {
		t.Fatalf("expected 2 page requests, got %v", requestedPages)
	}
}

func TestListDeviceInventoriesStripsClientPagination(t *testing.T) {
	t.Parallel()

	var gotPerPage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPerPage = r.URL.Query().Get("per_page")
		w.Header().Set("X-Total-Count", "0")
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, true)
	// Caller-provided pagination must be ignored in favor of the internal size.
	_, err := c.ListDeviceInventories("tok", url.Values{"per_page": {"7"}, "page": {"9"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPerPage != strconv.Itoa(autoPageSize) {
		t.Fatalf("per_page = %q, want %q", gotPerPage, strconv.Itoa(autoPageSize))
	}
}

func TestCountDeviceInventories(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("per_page") != "1" {
			t.Errorf("per_page = %q, want 1", r.URL.Query().Get("per_page"))
		}
		w.Header().Set("X-Total-Count", "42")
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, true)
	count, err := c.CountDeviceInventories("tok", url.Values{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 42 {
		t.Fatalf("count = %d, want 42", count)
	}
}

func TestCountDeviceInventoriesMissingHeader(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, true)
	if _, err := c.CountDeviceInventories("tok", url.Values{}); err == nil {
		t.Fatalf("expected error when X-Total-Count is missing")
	}
}

func TestWithoutPagination(t *testing.T) {
	t.Parallel()
	in := url.Values{
		"page":     {"3"},
		"per_page": {"50"},
		"hostname": {"gw"},
		"status":   {"accepted"},
	}
	out := withoutPagination(in)
	if out.Has("page") || out.Has("per_page") {
		t.Fatalf("expected pagination stripped, got %v", out)
	}
	if out.Get("hostname") != "gw" || out.Get("status") != "accepted" {
		t.Fatalf("expected other params preserved, got %v", out)
	}
}
