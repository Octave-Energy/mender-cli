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
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/deviceconnect"
	"github.com/mendersoftware/mender-cli/client/inventory"
)

// ensureDeviceConnected verifies that the target device is currently connected
// and reachable via the deviceconnect API before a session (terminal,
// port-forward or file transfer) is established.
func ensureDeviceConnected(client *deviceconnect.Client, deviceID string) error {
	device, err := client.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("unable to get the device: %w", err)
	}
	if device.Status != deviceconnect.StatusConnected {
		return errors.New("the device is not connected")
	}
	return nil
}

// argDeviceID is the flag used to target a device by its verbatim id, shared by
// the commands that operate on a single device (get, terminal, port-forward, cp).
const argDeviceID = "id"

// maxReportedMatches caps how many device ids are listed when a --filter
// matches more devices than expected.
const maxReportedMatches = 20

// addDeviceTargetFlags registers the --id and -f/--filter flags used to target
// a single device either verbatim or via inventory attribute filters.
func addDeviceTargetFlags(cmd *cobra.Command) {
	cmd.Flags().String(argDeviceID, "", "device id to target (verbatim)")
	cmd.Flags().StringSliceP(
		argInventoryFilter,
		"f",
		nil,
		"filter by attribute: name=value or scope/name=value "+
			"(scope defaults to inventory); repeat -f for multiple",
	)
}

// deviceTargetProvided reports whether the device was targeted via --id or
// --filter (as opposed to a positional argument).
func deviceTargetProvided(cmd *cobra.Command) (bool, error) {
	id, err := cmd.Flags().GetString(argDeviceID)
	if err != nil {
		return false, err
	}
	filters, err := cmd.Flags().GetStringSlice(argInventoryFilter)
	if err != nil {
		return false, err
	}
	return id != "" || len(filters) > 0, nil
}

// resolveDeviceID returns a single device id from either an explicit id (used
// verbatim, no API call) or a set of inventory filters resolved via the
// inventory search endpoint, which must match exactly one device.
func resolveDeviceID(
	server, token string,
	skipVerify bool,
	id string,
	filters []string,
) (string, error) {
	if id != "" {
		return id, nil
	}

	q := url.Values{}
	// Filters are passed verbatim; the server defaults a missing scope to
	// "inventory", matching the behavior of "inventory devices list".
	if err := addInventoryFilters(q, filters); err != nil {
		return "", err
	}

	cli := inventory.NewClient(server, skipVerify)
	res, err := cli.ListDeviceInventories(token, q)
	if err != nil {
		return "", err
	}

	ids, err := decodeDeviceIDs(res.Body)
	if err != nil {
		return "", err
	}
	switch len(ids) {
	case 0:
		return "", errors.New("no device matches the given filter(s)")
	case 1:
		return ids[0], nil
	default:
		return "", errTooManyMatches(ids)
	}
}

// decodeDeviceIDs extracts the device ids from a device inventory list response.
func decodeDeviceIDs(body []byte) ([]string, error) {
	var list []deviceInventory
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	ids := make([]string, 0, len(list))
	for _, dev := range list {
		ids = append(ids, dev.ID)
	}
	return ids, nil
}

// errTooManyMatches builds an error listing the matched device ids (capped),
// used when a --filter is expected to match exactly one device.
func errTooManyMatches(ids []string) error {
	shown := ids
	suffix := ""
	if len(shown) > maxReportedMatches {
		shown = shown[:maxReportedMatches]
		suffix = fmt.Sprintf("\n  ... and %d more", len(ids)-maxReportedMatches)
	}
	return fmt.Errorf(
		"filter matched %d devices, expected exactly one; "+
			"refine --filter or use 'inventory devices list':\n  %s%s",
		len(ids),
		strings.Join(shown, "\n  "),
		suffix,
	)
}
