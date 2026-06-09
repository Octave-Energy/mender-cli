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
	"fmt"
	"strings"
	"testing"
)

func TestDecodeDeviceIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		want    []string
		wantErr bool
	}{
		{
			name: "multiple devices",
			body: `[{"id":"a"},{"id":"b"},{"id":"c"}]`,
			want: []string{"a", "b", "c"},
		},
		{
			name: "empty array",
			body: `[]`,
			want: []string{},
		},
		{
			name:    "malformed json",
			body:    `not json`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeDeviceIDs([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestErrTooManyMatches(t *testing.T) {
	t.Parallel()

	t.Run("lists all when under cap", func(t *testing.T) {
		ids := []string{"a", "b", "c"}
		msg := errTooManyMatches(ids).Error()
		if !strings.Contains(msg, "matched 3 devices") {
			t.Fatalf("expected count in message, got %q", msg)
		}
		for _, id := range ids {
			if !strings.Contains(msg, id) {
				t.Fatalf("expected id %q in message, got %q", id, msg)
			}
		}
		if strings.Contains(msg, "and ") && strings.Contains(msg, "more") {
			t.Fatalf("did not expect truncation suffix, got %q", msg)
		}
	})

	t.Run("truncates when over cap", func(t *testing.T) {
		ids := make([]string, maxReportedMatches+5)
		for i := range ids {
			ids[i] = fmt.Sprintf("id-%d", i)
		}
		msg := errTooManyMatches(ids).Error()
		if !strings.Contains(msg, fmt.Sprintf("matched %d devices", len(ids))) {
			t.Fatalf("expected total count in message, got %q", msg)
		}
		if !strings.Contains(msg, "... and 5 more") {
			t.Fatalf("expected truncation suffix, got %q", msg)
		}
		// The id just past the cap must not be shown verbatim in the list.
		if strings.Contains(msg, fmt.Sprintf("id-%d\n", maxReportedMatches)) {
			t.Fatalf("did not expect capped id to be listed, got %q", msg)
		}
	})
}

func TestResolveDeviceIDExplicit(t *testing.T) {
	t.Parallel()
	// An explicit id is returned verbatim without contacting any server.
	got, err := resolveDeviceID("", "", true, "device-123", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "device-123" {
		t.Fatalf("got %q, want %q", got, "device-123")
	}
}
