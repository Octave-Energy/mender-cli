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

var deploymentsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a single deployment from the Mender server.",
	Long: "Show the details of a single deployment from the Mender server, " +
		"selected by its id with --id.",
	Example: `  mender-cli deployments get --id 00a0c91e6-7dec-11d0-a765-f81d4faebf6
  mender-cli deployments get --id 00a0c91e6-7dec-11d0-a765-f81d4faebf6 --detail 2
  mender-cli deployments get --id 00a0c91e6-7dec-11d0-a765-f81d4faebf6 --raw`,
	Args: cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewDeploymentsGetCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	deploymentsGetCmd.Flags().String(argDeploymentID, "", "deployment id to fetch (verbatim)")
	deploymentsGetCmd.Flags().IntP(argDetailLevel, "d", 0, "deployments detail level [0..3]")
	deploymentsGetCmd.Flags().BoolP(
		argRawMode, "r", false, "output the raw JSON returned by the Mender server")
}

// DeploymentsGetCmd implements `mender-cli deployments get`.
type DeploymentsGetCmd struct {
	server      string
	skipVerify  bool
	token       string
	id          string
	detailLevel int
	rawMode     bool
}

// NewDeploymentsGetCmd validates flags and returns a new DeploymentsGetCmd.
func NewDeploymentsGetCmd(cmd *cobra.Command, args []string) (*DeploymentsGetCmd, error) {
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

	rawMode, err := flags.GetBool(argRawMode)
	if err != nil {
		return nil, err
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	return &DeploymentsGetCmd{
		server:      server,
		token:       token,
		skipVerify:  skipVerify,
		id:          id,
		detailLevel: detailLevel,
		rawMode:     rawMode,
	}, nil
}

func (c *DeploymentsGetCmd) Run() error {
	client := deployments.NewClient(c.server, c.skipVerify)
	return client.GetDeployment(c.token, c.id, c.detailLevel, c.rawMode)
}
