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
	"os"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/inventory"
	"github.com/mendersoftware/mender-cli/log"
)

const (
	// Example is shown once by Cobra under "Examples:".
	inventoryDevicesGetExamples = `  mender-cli inventory devices get --id 0123456789abcdef0123456789abcdef
  mender-cli inventory devices get --id 0123456789abcdef0123456789abcdef --raw
  mender-cli inventory devices get -f hostname=my-gateway
  mender-cli inventory devices get -f inventory/mac=00:11:22:33:44:55 -f tags/env=prod`
)

var inventoryDevicesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show device inventory (attributes, tags) by id or filter.",
	Long: "Show the reported inventory (attributes, tags) of a single device.\n\n" +
		"Specify the target with either --id or a --filter expression. A filter " +
		"must match exactly one device; if multiple devices match, their ids are " +
		"listed so you can refine the query.",
	Example: inventoryDevicesGetExamples,
	Args:    cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewInventoryDevicesGetCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	inventoryDevicesGetCmd.Flags().BoolP(argRawMode, "r", false, "output the raw JSON returned by the Mender server")
	inventoryDevicesGetCmd.Flags().IntP(argDetailLevel, "d", 0, "inventory detail level [0..3]")
	addDeviceTargetFlags(inventoryDevicesGetCmd)
}

// InventoryDevicesGetCmd implements `mender-cli inventory devices get`.
type InventoryDevicesGetCmd struct {
	server      string
	skipVerify  bool
	token       string
	deviceID    string
	filters     []string
	rawMode     bool
	detailLevel int
}

// NewInventoryDevicesGetCmd validates flags and returns a new InventoryDevicesGetCmd.
func NewInventoryDevicesGetCmd(cmd *cobra.Command, args []string) (*InventoryDevicesGetCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}
	flags := cmd.Flags()
	rawMode, err := flags.GetBool(argRawMode)
	if err != nil {
		return nil, err
	}
	detailLevel, err := flags.GetInt(argDetailLevel)
	if err != nil {
		return nil, err
	}
	deviceID, err := flags.GetString(argDeviceID)
	if err != nil {
		return nil, err
	}
	filters, err := flags.GetStringSlice(argInventoryFilter)
	if err != nil {
		return nil, err
	}

	if deviceID == "" && len(filters) == 0 {
		return nil, errors.New("one of --id or --filter is required")
	}
	if deviceID != "" && len(filters) > 0 {
		return nil, errors.New("only one of --id or --filter may be used")
	}
	if err := validateInventoryFilters(filters); err != nil {
		return nil, err
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}
	return &InventoryDevicesGetCmd{
		server:      server,
		skipVerify:  skipVerify,
		token:       token,
		deviceID:    deviceID,
		filters:     filters,
		rawMode:     rawMode,
		detailLevel: detailLevel,
	}, nil
}

func (c *InventoryDevicesGetCmd) Run() error {
	cli := inventory.NewClient(c.server, c.skipVerify)
	if len(c.filters) > 0 {
		return c.runByFilter(cli)
	}
	return c.runByID(cli)
}

func (c *InventoryDevicesGetCmd) runByID(cli *inventory.Client) error {
	res, err := cli.GetDeviceInventory(c.token, c.deviceID)
	if err != nil {
		return err
	}
	if c.rawMode {
		_, err := os.Stdout.Write(res.Body)
		return err
	}
	var dev deviceInventory
	if err := json.Unmarshal(res.Body, &dev); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	listInventoryDevice(os.Stdout, dev, c.detailLevel)
	return nil
}

func (c *InventoryDevicesGetCmd) runByFilter(cli *inventory.Client) error {
	q := url.Values{}
	// Filters are passed verbatim; the server defaults a missing scope to
	// "inventory", matching the behavior of "inventory devices list".
	if err := addInventoryFilters(q, c.filters); err != nil {
		return err
	}

	res, err := cli.ListDeviceInventories(c.token, q)
	if err != nil {
		return err
	}
	if res.TotalCount != "" {
		log.Verb(fmt.Sprintf("X-Total-Count: %s", res.TotalCount))
	}

	var matches []json.RawMessage
	if err := json.Unmarshal(res.Body, &matches); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	switch {
	case len(matches) == 0:
		return errors.New("no device matches the given filter(s)")
	case len(matches) > 1:
		ids, err := decodeDeviceIDs(res.Body)
		if err != nil {
			return err
		}
		return errTooManyMatches(ids)
	}

	if c.rawMode {
		_, err := os.Stdout.Write(matches[0])
		return err
	}

	var dev deviceInventory
	if err := json.Unmarshal(matches[0], &dev); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	listInventoryDevice(os.Stdout, dev, c.detailLevel)
	return nil
}
