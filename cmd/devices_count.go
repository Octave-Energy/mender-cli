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
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/devices"
)

var devicesCountCmd = &cobra.Command{
	Use:   "count",
	Short: "Count devices, optionally filtered by authentication status.",
	Long: "Count devices from the Mender server's device authentication " +
		"service.\n\n" +
		"This efficiently returns only the total number of devices without " +
		"listing them. Use --status to count only devices with a given " +
		"authentication status.",
	Example: `  mender-cli devices count
  mender-cli devices count --status pending`,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewDevicesCountCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	devicesCountCmd.Flags().String(
		argDeviceStatus,
		"",
		"only devices with this auth status: "+strings.Join(deviceAuthStatuses, ", "),
	)
	_ = devicesCountCmd.RegisterFlagCompletionFunc(argDeviceStatus, deviceStatusCompletion)
}

// DevicesCountCmd implements `mender-cli devices count`.
type DevicesCountCmd struct {
	server     string
	skipVerify bool
	token      string
	status     string
}

// NewDevicesCountCmd validates flags and returns a new DevicesCountCmd.
func NewDevicesCountCmd(cmd *cobra.Command, args []string) (*DevicesCountCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()

	status, err := flags.GetString(argDeviceStatus)
	if err != nil {
		return nil, err
	}
	if err := validateDeviceStatus(status); err != nil {
		return nil, err
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	return &DevicesCountCmd{
		server:     server,
		token:      token,
		skipVerify: skipVerify,
		status:     status,
	}, nil
}

func (c *DevicesCountCmd) Run() error {
	client := devices.NewClient(c.server, c.skipVerify)
	count, err := client.CountDevices(c.token, c.status)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, count)
	return nil
}
