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
	"strings"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/deployments"
)

var deploymentsDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List the devices of a deployment and their status.",
	Long: "List the devices that are part of a deployment, together with their " +
		"per-device status. Select the deployment with --id.\n\n" +
		"All matching devices are returned: pagination is handled transparently. " +
		"Use --status to filter by per-device deployment status.",
	Example: `  mender-cli deployments devices --id 00a0c91e6-7dec-11d0-a765-f81d4faebf6
  mender-cli deployments devices --id 00a0c91e6-7dec-11d0-a765-f81d4faebf6 --status failure
  mender-cli deployments devices --id 00a0c91e6-7dec-11d0-a765-f81d4faebf6 --detail 2`,
	Args: cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewDeploymentsDevicesCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	deploymentsDevicesCmd.Flags().String(argDeploymentID, "", "deployment id (verbatim)")
	deploymentsDevicesCmd.Flags().IntP(argDetailLevel, "d", 0, "devices detail level [0..3]")
	deploymentsDevicesCmd.Flags().String(
		argDeploymentStatus,
		"",
		"filter by per-device status: "+strings.Join(deploymentDeviceStatuses, ", "))
	_ = deploymentsDevicesCmd.RegisterFlagCompletionFunc(
		argDeploymentStatus, enumCompletion(deploymentDeviceStatuses))
	deploymentsDevicesCmd.Flags().BoolP(
		argRawMode, "r", false, "output the raw JSON returned by the Mender server")
}

// DeploymentsDevicesCmd implements `mender-cli deployments devices`.
type DeploymentsDevicesCmd struct {
	server      string
	skipVerify  bool
	token       string
	id          string
	detailLevel int
	status      string
	rawMode     bool
}

// NewDeploymentsDevicesCmd validates flags and returns a new DeploymentsDevicesCmd.
func NewDeploymentsDevicesCmd(cmd *cobra.Command, args []string) (*DeploymentsDevicesCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()

	id, err := flags.GetString(argDeploymentID)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, errors.New("--id is required")
	}

	detailLevel, err := flags.GetInt(argDetailLevel)
	if err != nil {
		return nil, err
	}

	status, err := flags.GetString(argDeploymentStatus)
	if err != nil {
		return nil, err
	}
	if err := validateEnum(argDeploymentStatus, status, deploymentDeviceStatuses); err != nil {
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

	return &DeploymentsDevicesCmd{
		server:      server,
		token:       token,
		skipVerify:  skipVerify,
		id:          id,
		detailLevel: detailLevel,
		status:      status,
		rawMode:     rawMode,
	}, nil
}

func (c *DeploymentsDevicesCmd) Run() error {
	client := deployments.NewClient(c.server, c.skipVerify)
	return client.ListDeploymentDevices(c.token, c.id, c.detailLevel, c.status, c.rawMode)
}
