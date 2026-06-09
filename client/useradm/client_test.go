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

package useradm

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     int
		wantErr    bool
		wantVerify bool
		wantStatus int
	}{
		{name: "ok", status: http.StatusOK},
		{name: "unauthorized", status: http.StatusUnauthorized, wantErr: true, wantVerify: true, wantStatus: http.StatusUnauthorized},
		{name: "forbidden", status: http.StatusForbidden, wantErr: true, wantVerify: true, wantStatus: http.StatusForbidden},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			c := NewClient(srv.URL, true)
			err := c.Verify("tok")
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error for status %d", tc.status)
			}
			var ve *VerifyError
			if tc.wantVerify {
				if !errors.As(err, &ve) {
					t.Fatalf("expected *VerifyError, got %T: %v", err, err)
				}
				if ve.StatusCode != tc.wantStatus {
					t.Fatalf("VerifyError.StatusCode = %d, want %d", ve.StatusCode, tc.wantStatus)
				}
			} else if errors.As(err, &ve) {
				t.Fatalf("did not expect *VerifyError for status %d", tc.status)
			}
		})
	}
}

func TestLoginErrorStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, true)
	if _, err := c.Login("user", "pass", ""); err == nil {
		t.Fatalf("expected error on 401 login")
	}
}

func TestLoginSuccess(t *testing.T) {
	t.Parallel()
	const token = "the.jwt.token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(token))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, true)
	body, err := c.Login("user", "pass", "123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != token {
		t.Fatalf("token body = %q, want %q", string(body), token)
	}
}
