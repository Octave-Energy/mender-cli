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

	"github.com/mendersoftware/mender-cli/client/deployments"
)

var deploymentsLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Get the deployment log of a single device.",
	Long: "Print the deployment log (text) of a single device within a " +
		"deployment. Select the deployment with --id and the device with " +
		"--device.",
	Example: `  mender-cli deployments log --id 00a0c91e6-7dec-11d0-a765-f81d4faebf6 ` +
		`--device 0123456789abcdef0123456789abcdef`,
	Args: cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewDeploymentsLogCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	deploymentsLogCmd.Flags().String(argDeploymentID, "", "deployment id (verbatim)")
	deploymentsLogCmd.Flags().String(argDeploymentDevice, "", "device id (verbatim)")
}

// DeploymentsLogCmd implements `mender-cli deployments log`.
type DeploymentsLogCmd struct {
	server     string
	skipVerify bool
	token      string
	id         string
	deviceID   string
}

// NewDeploymentsLogCmd validates flags and returns a new DeploymentsLogCmd.
func NewDeploymentsLogCmd(cmd *cobra.Command, args []string) (*DeploymentsLogCmd, error) {
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

	deviceID, err := flags.GetString(argDeploymentDevice)
	if err != nil {
		return nil, err
	}
	if deviceID == "" {
		return nil, errors.New("--device is required")
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	return &DeploymentsLogCmd{
		server:     server,
		token:      token,
		skipVerify: skipVerify,
		id:         id,
		deviceID:   deviceID,
	}, nil
}

func (c *DeploymentsLogCmd) Run() error {
	client := deployments.NewClient(c.server, c.skipVerify)
	return client.DeploymentDeviceLog(c.token, c.id, c.deviceID)
}
