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

	"github.com/mendersoftware/mender-cli/client/releases"
)

const (
	argReleaseName       = "name"
	argReleaseTag        = "tag"
	argReleaseUpdateType = "update-type"
	argReleaseSort       = "sort"
)

// releaseSortValues are the sort options accepted by the release list API.
var releaseSortValues = []string{
	"name:asc", "name:desc",
	"modified:asc", "modified:desc",
	"artifacts_count:asc", "artifacts_count:desc",
	"tags:asc", "tags:desc",
}

func releaseSortCompletion(
	_ *cobra.Command, _ []string, _ string,
) ([]string, cobra.ShellCompDirective) {
	return releaseSortValues, cobra.ShellCompDirectiveNoFileComp
}

func validateReleaseSort(sort string) error {
	if sort == "" {
		return nil
	}
	for _, s := range releaseSortValues {
		if sort == s {
			return nil
		}
	}
	return fmt.Errorf(
		"invalid --sort %q, expected one of: %s",
		sort, strings.Join(releaseSortValues, ", "),
	)
}

var releasesListCmd = &cobra.Command{
	Use:   "list",
	Short: "Get a list of releases from the Mender server.",
	Long: "Get a list of releases (groups of artifacts sharing a release name) " +
		"from the Mender server.\n\n" +
		"All matching releases are returned: pagination is handled transparently " +
		"(like 'inventory devices list'). Use --name, --tag and --update-type to " +
		"narrow the results, and --sort to order them.",
	Example: `  mender-cli releases list
  mender-cli releases list --detail 2
  mender-cli releases list --name my-app
  mender-cli releases list --tag stable --tag qa
  mender-cli releases list --update-type rootfs-image --sort modified:desc
  mender-cli releases list --raw`,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewReleasesListCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	releasesListCmd.Flags().IntP(argDetailLevel, "d", 0, "releases list detail level [0..3]")
	releasesListCmd.Flags().String(argReleaseName, "", "filter by release name")
	releasesListCmd.Flags().StringArray(
		argReleaseTag,
		nil,
		"filter by release tag; repeat to match any of several tags",
	)
	releasesListCmd.Flags().String(argReleaseUpdateType, "", "filter by update type")
	releasesListCmd.Flags().String(
		argReleaseSort,
		"",
		"sort results: "+strings.Join(releaseSortValues, ", "),
	)
	_ = releasesListCmd.RegisterFlagCompletionFunc(argReleaseSort, releaseSortCompletion)
	releasesListCmd.Flags().BoolP(
		argRawMode,
		"r",
		false,
		"output the raw JSON returned by the Mender server")
}

// ReleasesListCmd implements `mender-cli releases list`.
type ReleasesListCmd struct {
	server      string
	skipVerify  bool
	token       string
	detailLevel int
	rawMode     bool
	name        string
	tags        []string
	updateType  string
	sort        string
}

// NewReleasesListCmd validates flags and returns a new ReleasesListCmd.
func NewReleasesListCmd(cmd *cobra.Command, args []string) (*ReleasesListCmd, error) {
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

	name, err := flags.GetString(argReleaseName)
	if err != nil {
		return nil, err
	}

	tags, err := flags.GetStringArray(argReleaseTag)
	if err != nil {
		return nil, err
	}

	updateType, err := flags.GetString(argReleaseUpdateType)
	if err != nil {
		return nil, err
	}

	sort, err := flags.GetString(argReleaseSort)
	if err != nil {
		return nil, err
	}
	if err := validateReleaseSort(sort); err != nil {
		return nil, err
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	return &ReleasesListCmd{
		server:      server,
		token:       token,
		skipVerify:  skipVerify,
		detailLevel: detailLevel,
		rawMode:     rawMode,
		name:        name,
		tags:        tags,
		updateType:  updateType,
		sort:        sort,
	}, nil
}

func (c *ReleasesListCmd) Run() error {
	q := url.Values{}
	if c.name != "" {
		q.Set("name", c.name)
	}
	for _, t := range c.tags {
		if t != "" {
			q.Add("tag", t)
		}
	}
	if c.updateType != "" {
		q.Set("update_type", c.updateType)
	}
	if c.sort != "" {
		q.Set("sort", c.sort)
	}

	client := releases.NewClient(c.server, c.skipVerify)
	return client.ListReleases(c.token, c.detailLevel, q, c.rawMode)
}
