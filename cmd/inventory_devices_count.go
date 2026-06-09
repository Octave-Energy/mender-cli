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
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/inventory"
)

const inventoryDevicesCountExamples = `  mender-cli inventory devices count
  mender-cli inventory devices count -f hostname=my-gateway
  mender-cli inventory devices count -f inventory/mac=00:11:22:33:44:55 -f tags/env=prod
  mender-cli inventory devices count --group production --has-group=true`

var inventoryDevicesCountCmd = &cobra.Command{
	Use:   "count",
	Short: "Count devices matching the given inventory filters.",
	Long: "Count devices matching the given inventory filters.\n\n" +
		"This efficiently returns only the total number of matching devices " +
		"(read from the server's X-Total-Count header) without listing them. Use " +
		"--filter to narrow the results.",
	Example: inventoryDevicesCountExamples,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewInventoryDevicesCountCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	inventoryDevicesCountCmd.Flags().String(argInventoryGroup, "", "only devices in this group")
	inventoryDevicesCountCmd.Flags().Bool(
		argInventoryHasGroup,
		false,
		"only devices in a group (true) or not in any group (false); omit for both",
	)
	inventoryDevicesCountCmd.Flags().StringSliceP(
		argInventoryFilter,
		"f",
		nil,
		"filter by attribute: name=value or scope/name=value; repeat -f for multiple",
	)
}

// InventoryDevicesCountCmd implements `mender-cli inventory devices count`.
type InventoryDevicesCountCmd struct {
	server      string
	skipVerify  bool
	token       string
	group       string
	hasGroup    bool
	hasGroupSet bool
	filters     []string
}

// NewInventoryDevicesCountCmd validates flags and returns a new InventoryDevicesCountCmd.
func NewInventoryDevicesCountCmd(cmd *cobra.Command, args []string) (*InventoryDevicesCountCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()

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

	return &InventoryDevicesCountCmd{
		server:      server,
		token:       token,
		skipVerify:  skipVerify,
		group:       group,
		hasGroup:    hasGroup,
		hasGroupSet: hasGroupSet,
		filters:     filters,
	}, nil
}

func (c *InventoryDevicesCountCmd) Run() error {
	q := url.Values{}
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
	count, err := client.CountDeviceInventories(c.token, q)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, count)
	return nil
}
