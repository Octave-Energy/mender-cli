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

// Package client provides shared HTTP helpers used by the Mender API client
// packages, including a configurable HTTP client and basic GET/POST request
// helpers.
package client

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/mendersoftware/mender-cli/log"
)

const (
	httpErrorBoundary = 300
)

// NewHTTPClient returns an *http.Client configured for talking to the Mender
// server. When skipVerify is true, TLS certificate verification is disabled.
func NewHTTPClient(skipVerify bool) *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
	}

	return &http.Client{
		Transport: tr,
	}
}

// JoinURL joins a base URL and a path, ensuring exactly one slash between them.
func JoinURL(base, url string) string {
	url = strings.TrimPrefix(url, "/")
	if !strings.HasSuffix(base, "/") {
		base = base + "/"
	}
	return base + url
}

// DoGetRequest performs an authenticated GET request to urlPath and returns the
// response body. It returns an error if the request fails or the server
// responds with a non-200 status.
func DoGetRequest(token, urlPath string, client *http.Client) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, urlPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	reqDump, err := httputil.DumpRequest(req, false)
	if err != nil {
		return nil, err
	}
	log.Verbf("sending request: \n%s", string(reqDump))

	rsp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s request failed: %w", urlPath, err)
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s request failed with status %d", urlPath, rsp.StatusCode)
	}

	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// DoPostRequest performs an authenticated JSON POST request to urlPath with the
// given body and returns the response body. It returns an error if the request
// fails or the server responds with a status above the error boundary.
func DoPostRequest(
	token, urlPath string,
	client *http.Client,
	requestBody io.Reader,
) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, urlPath, requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	reqDump, err := httputil.DumpRequest(req, false)
	if err != nil {
		return nil, err
	}
	log.Verbf("sending request: \n%s", string(reqDump))

	rsp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s request failed: %w", urlPath, err)
	}
	if rsp.StatusCode > httpErrorBoundary {
		return nil, fmt.Errorf("POST %s request failed with status %d", urlPath, rsp.StatusCode)
	}

	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
