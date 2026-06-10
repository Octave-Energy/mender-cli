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

	"github.com/mendersoftware/mender-cli/client/releases"
)

var releasesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a single release from the Mender server.",
	Long: "Show a single release (a group of artifacts sharing a release name) " +
		"from the Mender server, selected by its name with --name.",
	Example: `  mender-cli releases get --name my-app-v1.0.0
  mender-cli releases get --name my-app-v1.0.0 --detail 2
  mender-cli releases get --name my-app-v1.0.0 --raw`,
	Args: cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewReleasesGetCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	releasesGetCmd.Flags().String(argReleaseName, "", "release name to fetch (verbatim)")
	releasesGetCmd.Flags().IntP(argDetailLevel, "d", 0, "releases detail level [0..3]")
	releasesGetCmd.Flags().BoolP(
		argRawMode,
		"r",
		false,
		"output the raw JSON returned by the Mender server",
	)
}

// ReleasesGetCmd implements `mender-cli releases get`.
type ReleasesGetCmd struct {
	server      string
	skipVerify  bool
	token       string
	name        string
	detailLevel int
	rawMode     bool
}

// NewReleasesGetCmd validates flags and returns a new ReleasesGetCmd.
func NewReleasesGetCmd(cmd *cobra.Command, args []string) (*ReleasesGetCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()

	name, err := flags.GetString(argReleaseName)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("--name is required")
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

	return &ReleasesGetCmd{
		server:      server,
		token:       token,
		skipVerify:  skipVerify,
		name:        name,
		detailLevel: detailLevel,
		rawMode:     rawMode,
	}, nil
}

func (c *ReleasesGetCmd) Run() error {
	client := releases.NewClient(c.server, c.skipVerify)
	return client.GetRelease(c.token, c.name, c.detailLevel, c.rawMode)
}
