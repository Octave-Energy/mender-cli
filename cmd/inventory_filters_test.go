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

package cmd

import (
	"net/url"
	"testing"
)

func TestInventoryFilterKeyValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		wantKey string
		wantVal string
		wantErr bool
	}{
		{name: "simple", in: "hostname=gw", wantKey: "hostname", wantVal: "gw"},
		{name: "scoped", in: "inventory/mac=00:11", wantKey: "inventory/mac", wantVal: "00:11"},
		{name: "trims spaces", in: "  k=v  ", wantKey: "k", wantVal: "v"},
		{name: "value with equals", in: "k=a=b", wantKey: "k", wantVal: "a=b"},
		{name: "empty", in: "   ", wantErr: true},
		{name: "no equals", in: "novalue", wantErr: true},
		{name: "empty key", in: "=v", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, val, err := inventoryFilterKeyValue(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key != tc.wantKey || val != tc.wantVal {
				t.Fatalf("got (%q,%q), want (%q,%q)", key, val, tc.wantKey, tc.wantVal)
			}
		})
	}
}

func TestValidateInventoryFilters(t *testing.T) {
	t.Parallel()
	if err := validateInventoryFilters([]string{"a=1", "b/c=2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := validateInventoryFilters([]string{"a=1", "bad"}); err == nil {
		t.Fatalf("expected error for invalid filter")
	}
}

func TestAddInventoryFilters(t *testing.T) {
	t.Parallel()
	q := url.Values{}
	if err := addInventoryFilters(q, []string{"hostname=gw", "tags/env=prod"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := q.Get("hostname"); got != "gw" {
		t.Fatalf("hostname = %q, want %q", got, "gw")
	}
	if got := q.Get("tags/env"); got != "prod" {
		t.Fatalf("tags/env = %q, want %q", got, "prod")
	}

	// An invalid filter is reported and leaves nothing partially applied beyond
	// the failing entry.
	if err := addInventoryFilters(url.Values{}, []string{"bad"}); err == nil {
		t.Fatalf("expected error for invalid filter")
	}
}
