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

var artifactsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Get a list of artifacts from the Mender server.",
	Long: "Get a list of artifacts from the Mender server.\n\n" +
		"Results are paginated: use --page and --per-page to navigate. (This " +
		"differs from 'inventory devices list', which transparently returns all " +
		"matching devices.)",
	Example: `  mender-cli artifacts list
  mender-cli artifacts list --detail 3
  mender-cli artifacts list --page 2 --per-page 50
  mender-cli artifacts list --raw`,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewArtifactsListCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	artifactsListCmd.Flags().IntP(argDetailLevel, "d", 0, "artifacts list detail level [0..3]")
	artifactsListCmd.Flags().IntP(argPerPage, "N", 20, "number of results to display")
	artifactsListCmd.Flags().IntP(argPage, "P", 1, "page number to return")
	artifactsListCmd.Flags().BoolP(
		argRawMode,
		"r",
		false,
		"artifacts list raw mode (json from mender server)")
}

// ArtifactsListCmd implements `mender-cli artifacts list`.
type ArtifactsListCmd struct {
	server        string
	skipVerify    bool
	token         string
	detailLevel   int
	rawMode       bool
	page, perPage int
}

// NewArtifactsListCmd validates flags and returns a new ArtifactsListCmd.
func NewArtifactsListCmd(cmd *cobra.Command, args []string) (*ArtifactsListCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()

	detailLevel, err := cmd.Flags().GetInt(argDetailLevel)
	if err != nil {
		return nil, err
	}

	rawMode, err := flags.GetBool(argRawMode)
	if err != nil {
		return nil, err
	}

	perPage, err := flags.GetInt(argPerPage)
	if err != nil {
		return nil, err
	}

	page, err := flags.GetInt(argPage)
	if err != nil {
		return nil, err
	}

	if page <= 0 || perPage <= 0 {
		return nil, errors.New("page and per-page arguments must be larger than 0")
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	return &ArtifactsListCmd{
		server:      server,
		token:       token,
		skipVerify:  skipVerify,
		detailLevel: detailLevel,
		rawMode:     rawMode,
		perPage:     perPage,
		page:        page,
	}, nil
}

func (c *ArtifactsListCmd) Run() error {

	client := deployments.NewClient(c.server, c.skipVerify)
	return client.ListArtifacts(c.token, c.detailLevel, c.perPage, c.page, c.rawMode)
}
