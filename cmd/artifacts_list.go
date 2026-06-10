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
	"strings"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/deployments"
)

const (
	argArtifactName       = "name"
	argArtifactDeviceType = "device-type"
)

var artifactsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Get a list of artifacts from the Mender server.",
	Long: "Get a list of artifacts from the Mender server.\n\n" +
		"All matching artifacts are returned: pagination is handled " +
		"transparently (like 'inventory devices list'). Use --name, " +
		"--description and --device-type to narrow the results. The name, " +
		"description and device-type filters support prefix matching by " +
		"appending '*' (e.g. --name 'my-app*'); --name may be repeated to match " +
		"several exact names (but cannot be combined with a prefix match).",
	Example: `  mender-cli artifacts list
  mender-cli artifacts list --detail 3
  mender-cli artifacts list --name my-app --device-type raspberrypi4
  mender-cli artifacts list --name 'release-*'
  mender-cli artifacts list --name app-a --name app-b
  mender-cli artifacts list --raw`,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewArtifactsListCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	artifactsListCmd.Flags().IntP(argDetailLevel, "d", 0, "artifacts list detail level [0..3]")
	artifactsListCmd.Flags().StringArray(
		argArtifactName,
		nil,
		"filter by artifact name; append '*' for prefix matching; repeat to match several names",
	)
	artifactsListCmd.Flags().String(
		argArtifactDescription,
		"",
		"filter by artifact description; append '*' for prefix matching",
	)
	artifactsListCmd.Flags().String(
		argArtifactDeviceType,
		"",
		"filter by compatible device type; append '*' for prefix matching",
	)
	artifactsListCmd.Flags().BoolP(
		argRawMode,
		"r",
		false,
		"output the raw JSON returned by the Mender server")
}

// ArtifactsListCmd implements `mender-cli artifacts list`.
type ArtifactsListCmd struct {
	server      string
	skipVerify  bool
	token       string
	detailLevel int
	rawMode     bool
	names       []string
	description string
	deviceType  string
}

// NewArtifactsListCmd validates flags and returns a new ArtifactsListCmd.
func NewArtifactsListCmd(cmd *cobra.Command, args []string) (*ArtifactsListCmd, error) {
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

	names, err := flags.GetStringArray(argArtifactName)
	if err != nil {
		return nil, err
	}
	if err := validateArtifactNames(names); err != nil {
		return nil, err
	}

	description, err := flags.GetString(argArtifactDescription)
	if err != nil {
		return nil, err
	}

	deviceType, err := flags.GetString(argArtifactDeviceType)
	if err != nil {
		return nil, err
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
		names:       names,
		description: description,
		deviceType:  deviceType,
	}, nil
}

// validateArtifactNames enforces the v2 API constraint that prefix matching
// (a trailing '*') cannot be combined with multiple --name values.
func validateArtifactNames(names []string) error {
	if len(names) <= 1 {
		return nil
	}
	for _, n := range names {
		if strings.HasSuffix(n, "*") {
			return fmt.Errorf(
				"prefix matching (--name %q) cannot be combined with multiple --name values",
				n,
			)
		}
	}
	return nil
}

func (c *ArtifactsListCmd) Run() error {
	q := url.Values{}
	for _, n := range c.names {
		if n != "" {
			q.Add("name", n)
		}
	}
	if c.description != "" {
		q.Set("description", c.description)
	}
	if c.deviceType != "" {
		q.Set("device_type", c.deviceType)
	}

	client := deployments.NewClient(c.server, c.skipVerify)
	return client.ListArtifacts(c.token, c.detailLevel, q, c.rawMode)
}
