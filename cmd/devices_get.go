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
	"errors"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/devices"
)

var devicesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show a single device by id or filter from the Mender server.",
	Long: "Show a single device from the Mender server's device authentication " +
		"service.\n\n" +
		"Specify the target with either --id or a --filter expression. A filter " +
		"is resolved via inventory and must match exactly one device; if " +
		"multiple devices match, their ids are listed so you can refine the query.",
	Example: `  mender-cli devices get --id 0123456789abcdef0123456789abcdef
  mender-cli devices get --id 0123456789abcdef0123456789abcdef --detail 3
  mender-cli devices get -f hostname=my-gateway
  mender-cli devices get -f inventory/mac=00:11:22:33:44:55 -f tags/env=prod`,
	Args: cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewDevicesGetCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	devicesGetCmd.Flags().IntP(argDetailLevel, "d", 0, "devices detail level [0..3]")
	devicesGetCmd.Flags().BoolP(
		argRawMode,
		"r",
		false,
		"output the raw JSON returned by the Mender server",
	)
	addDeviceTargetFlags(devicesGetCmd)
}

// DevicesGetCmd implements `mender-cli devices get`.
type DevicesGetCmd struct {
	server      string
	skipVerify  bool
	token       string
	deviceID    string
	filters     []string
	detailLevel int
	rawMode     bool
}

// NewDevicesGetCmd validates flags and returns a new DevicesGetCmd.
func NewDevicesGetCmd(cmd *cobra.Command, args []string) (*DevicesGetCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()

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

	detailLevel, err := flags.GetInt(argDetailLevel)
	if err != nil {
		return nil, err
	}

	rawMode, err := flags.GetBool(argRawMode)
	if err != nil {
		return nil, err
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	return &DevicesGetCmd{
		server:      server,
		token:       token,
		skipVerify:  skipVerify,
		deviceID:    deviceID,
		filters:     filters,
		detailLevel: detailLevel,
		rawMode:     rawMode,
	}, nil
}

func (c *DevicesGetCmd) Run() error {
	deviceID, err := resolveDeviceID(c.server, c.token, c.skipVerify, c.deviceID, c.filters)
	if err != nil {
		return err
	}

	client := devices.NewClient(c.server, c.skipVerify)
	return client.GetDevice(c.token, deviceID, c.detailLevel, c.rawMode)
}
