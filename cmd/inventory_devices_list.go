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
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/inventory"
	"github.com/mendersoftware/mender-cli/log"
)

const (
	argInventorySort     = "sort"
	argInventoryGroup    = "group"
	argInventoryHasGroup = "has-group"
	argInventoryFilter   = "filter"
	// Example is shown once by Cobra under "Examples:" (do not duplicate in Long).
	inventoryDevicesListExamples = `  mender-cli inventory devices list
  mender-cli inventory devices list --raw
  mender-cli inventory devices list -f hostname=my-gateway -d 1
  mender-cli inventory devices list -f inventory/mac=00:11:22:33:44:55 -f tags/env=prod
  mender-cli inventory devices list --group production --has-group=true`
)

var inventoryDevicesListCmd = &cobra.Command{
	Use:   "list",
	Short: "Get devices and reported inventory (attributes, tags) from the Mender server.",
	Long: "Get devices and their reported inventory (attributes, tags) from the " +
		"Mender server.\n\n" +
		"All matching devices are returned. Use --filter to narrow the results.",
	Example: inventoryDevicesListExamples,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewInventoryDevicesListCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	inventoryDevicesListCmd.Flags().IntP(argDetailLevel, "d", 0, "inventory detail level [0..3]")
	inventoryDevicesListCmd.Flags().BoolP(
		argRawMode,
		"r",
		false,
		"output the raw JSON returned by the Mender server",
	)
	inventoryDevicesListCmd.Flags().String(argInventorySort, "", "sort by attributes, e.g. attr1:asc,attr2:desc")
	inventoryDevicesListCmd.Flags().String(argInventoryGroup, "", "only devices in this group")
	inventoryDevicesListCmd.Flags().Bool(
		argInventoryHasGroup,
		false,
		"only devices in a group (true) or not in any group (false); omit for both",
	)
	inventoryDevicesListCmd.Flags().StringSliceP(
		argInventoryFilter,
		"f",
		nil,
		"filter by attribute: name=value or scope/name=value; repeat -f for multiple",
	)
}

// InventoryDevicesListCmd implements `mender-cli inventory devices list`.
type InventoryDevicesListCmd struct {
	server      string
	skipVerify  bool
	token       string
	detailLevel int
	rawMode     bool
	sort        string
	group       string
	hasGroup    bool
	hasGroupSet bool
	filters     []string
}

// NewInventoryDevicesListCmd validates flags and returns a new InventoryDevicesListCmd.
func NewInventoryDevicesListCmd(cmd *cobra.Command, args []string) (*InventoryDevicesListCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()

	detailLevel, err := flags.GetInt(argDetailLevel)
	if err != nil {
		return nil, err
	}

	rawMode, err := flags.GetBool(argRawMode)
	if err != nil {
		return nil, err
	}

	sort, err := flags.GetString(argInventorySort)
	if err != nil {
		return nil, err
	}

	group, err := flags.GetString(argInventoryGroup)
	if err != nil {
		return nil, err
	}

	hasGroup, err := flags.GetBool(argInventoryHasGroup)
	if err != nil {
		return nil, err
	}
	hasGroupSet := flags.Changed(argInventoryHasGroup)

	filters, err := flags.GetStringSlice(argInventoryFilter)
	if err != nil {
		return nil, err
	}

	if err := validateInventoryFilters(filters); err != nil {
		return nil, err
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	return &InventoryDevicesListCmd{
		server:      server,
		token:       token,
		skipVerify:  skipVerify,
		detailLevel: detailLevel,
		rawMode:     rawMode,
		sort:        sort,
		group:       group,
		hasGroup:    hasGroup,
		hasGroupSet: hasGroupSet,
		filters:     filters,
	}, nil
}

func validateInventoryFilters(filters []string) error {
	for _, f := range filters {
		if _, _, err := inventoryFilterKeyValue(f); err != nil {
			return err
		}
	}
	return nil
}

func addInventoryFilters(q url.Values, filters []string) error {
	for _, f := range filters {
		key, val, err := inventoryFilterKeyValue(f)
		if err != nil {
			return err
		}
		q.Add(key, val)
	}
	return nil
}

func inventoryFilterKeyValue(f string) (key, val string, err error) {
	f = strings.TrimSpace(f)
	if f == "" {
		return "", "", errors.New("empty --filter value")
	}
	idx := strings.Index(f, "=")
	if idx <= 0 {
		return "", "", fmt.Errorf("invalid --filter %q (expected KEY=VALUE)", f)
	}
	key = f[:idx]
	val = f[idx+1:]
	if key == "" {
		return "", "", fmt.Errorf("invalid --filter %q (empty key)", f)
	}
	return key, val, nil
}

func (c *InventoryDevicesListCmd) Run() error {
	q := url.Values{}
	if c.sort != "" {
		q.Set("sort", c.sort)
	}
	if c.group != "" {
		q.Set("group", c.group)
	}
	if c.hasGroupSet {
		q.Set("has_group", strconv.FormatBool(c.hasGroup))
	}
	if err := addInventoryFilters(q, c.filters); err != nil {
		return err
	}

	client := inventory.NewClient(c.server, c.skipVerify)
	res, err := client.ListDeviceInventories(c.token, q)
	if err != nil {
		return err
	}

	if res.TotalCount != "" || res.Link != "" {
		log.Verb(fmt.Sprintf("X-Total-Count: %s", res.TotalCount))
		log.Verb(fmt.Sprintf("Link: %s", res.Link))
	}

	if c.rawMode {
		if _, err := os.Stdout.Write(res.Body); err != nil {
			return fmt.Errorf("error writing response body: %w", err)
		}
		return nil
	}

	var list []deviceInventory
	if err := json.Unmarshal(res.Body, &list); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	out := os.Stdout
	for _, dev := range list {
		listInventoryDevice(out, dev, c.detailLevel)
	}
	return nil
}
