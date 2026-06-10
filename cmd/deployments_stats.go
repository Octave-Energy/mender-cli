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

var deploymentsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Get per-status device counts for a deployment.",
	Long: "Show the number of devices in each state for a single deployment, " +
		"selected by its id with --id.",
	Example: `  mender-cli deployments stats --id 00a0c91e6-7dec-11d0-a765-f81d4faebf6
  mender-cli deployments stats --id 00a0c91e6-7dec-11d0-a765-f81d4faebf6 --raw`,
	Args: cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewDeploymentsStatsCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	deploymentsStatsCmd.Flags().String(argDeploymentID, "", "deployment id (verbatim)")
	deploymentsStatsCmd.Flags().BoolP(
		argRawMode, "r", false, "output the raw JSON returned by the Mender server")
}

// DeploymentsStatsCmd implements `mender-cli deployments stats`.
type DeploymentsStatsCmd struct {
	server     string
	skipVerify bool
	token      string
	id         string
	rawMode    bool
}

// NewDeploymentsStatsCmd validates flags and returns a new DeploymentsStatsCmd.
func NewDeploymentsStatsCmd(cmd *cobra.Command, args []string) (*DeploymentsStatsCmd, error) {
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

	rawMode, err := flags.GetBool(argRawMode)
	if err != nil {
		return nil, err
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	return &DeploymentsStatsCmd{
		server:     server,
		token:      token,
		skipVerify: skipVerify,
		id:         id,
		rawMode:    rawMode,
	}, nil
}

func (c *DeploymentsStatsCmd) Run() error {
	client := deployments.NewClient(c.server, c.skipVerify)
	return client.DeploymentStatistics(c.token, c.id, c.rawMode)
}
