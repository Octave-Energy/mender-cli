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
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/deployments"
)

var deploymentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Get a list of deployments from the Mender server.",
	Long: "Get a list of deployments from the Mender server.\n\n" +
		"All matching deployments are returned: pagination is handled " +
		"transparently (like 'inventory devices list'). Use --id, --name, " +
		"--status, --type, --created-before/--created-after to narrow the " +
		"results, and --sort to order them by creation date.",
	Example: `  mender-cli deployments list
  mender-cli deployments list --status inprogress
  mender-cli deployments list --name production --type software
  mender-cli deployments list --sort desc --detail 2
  mender-cli deployments list --raw`,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewDeploymentsListCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	deploymentsListCmd.Flags().IntP(argDetailLevel, "d", 0, "deployments list detail level [0..3]")
	deploymentsListCmd.Flags().StringArray(
		argDeploymentID, nil, "filter by deployment id; repeat to match several")
	deploymentsListCmd.Flags().StringArray(
		argDeploymentName, nil, "filter by deployment name; repeat to match several")
	deploymentsListCmd.Flags().String(
		argDeploymentStatus, "", "filter by status: "+strings.Join(deploymentStatuses, ", "))
	deploymentsListCmd.Flags().String(
		argDeploymentType, "", "filter by type: "+strings.Join(deploymentTypes, ", "))
	deploymentsListCmd.Flags().Int64(
		argDeploymentCreatedB, 0, "only deployments created before this Unix timestamp (UTC)")
	deploymentsListCmd.Flags().Int64(
		argDeploymentCreatedA, 0, "only deployments created after this Unix timestamp (UTC)")
	deploymentsListCmd.Flags().String(
		argDeploymentSort, "", "sort by creation date: "+strings.Join(deploymentSortValues, ", "))
	_ = deploymentsListCmd.RegisterFlagCompletionFunc(
		argDeploymentStatus, enumCompletion(deploymentStatuses))
	_ = deploymentsListCmd.RegisterFlagCompletionFunc(
		argDeploymentType, enumCompletion(deploymentTypes))
	_ = deploymentsListCmd.RegisterFlagCompletionFunc(
		argDeploymentSort, enumCompletion(deploymentSortValues))
	deploymentsListCmd.Flags().BoolP(
		argRawMode, "r", false, "output the raw JSON returned by the Mender server")
}

// DeploymentsListCmd implements `mender-cli deployments list`.
type DeploymentsListCmd struct {
	server        string
	skipVerify    bool
	token         string
	detailLevel   int
	rawMode       bool
	ids           []string
	names         []string
	status        string
	depType       string
	sort          string
	createdBefore int64
	createdAfter  int64
}

// NewDeploymentsListCmd validates flags and returns a new DeploymentsListCmd.
func NewDeploymentsListCmd(cmd *cobra.Command, args []string) (*DeploymentsListCmd, error) {
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

	ids, err := flags.GetStringArray(argDeploymentID)
	if err != nil {
		return nil, err
	}

	names, err := flags.GetStringArray(argDeploymentName)
	if err != nil {
		return nil, err
	}

	status, err := flags.GetString(argDeploymentStatus)
	if err != nil {
		return nil, err
	}
	if err := validateEnum(argDeploymentStatus, status, deploymentStatuses); err != nil {
		return nil, err
	}

	depType, err := flags.GetString(argDeploymentType)
	if err != nil {
		return nil, err
	}
	if err := validateEnum(argDeploymentType, depType, deploymentTypes); err != nil {
		return nil, err
	}

	sort, err := flags.GetString(argDeploymentSort)
	if err != nil {
		return nil, err
	}
	if err := validateEnum(argDeploymentSort, sort, deploymentSortValues); err != nil {
		return nil, err
	}

	createdBefore, err := flags.GetInt64(argDeploymentCreatedB)
	if err != nil {
		return nil, err
	}

	createdAfter, err := flags.GetInt64(argDeploymentCreatedA)
	if err != nil {
		return nil, err
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	return &DeploymentsListCmd{
		server:        server,
		token:         token,
		skipVerify:    skipVerify,
		detailLevel:   detailLevel,
		rawMode:       rawMode,
		ids:           ids,
		names:         names,
		status:        status,
		depType:       depType,
		sort:          sort,
		createdBefore: createdBefore,
		createdAfter:  createdAfter,
	}, nil
}

func (c *DeploymentsListCmd) Run() error {
	q := url.Values{}
	for _, id := range c.ids {
		if id != "" {
			q.Add("id", id)
		}
	}
	for _, n := range c.names {
		if n != "" {
			q.Add("name", n)
		}
	}
	if c.status != "" {
		q.Set("status", c.status)
	}
	if c.depType != "" {
		q.Set("type", c.depType)
	}
	if c.sort != "" {
		q.Set("sort", c.sort)
	}
	if c.createdBefore > 0 {
		q.Set("created_before", strconv.FormatInt(c.createdBefore, 10))
	}
	if c.createdAfter > 0 {
		q.Set("created_after", strconv.FormatInt(c.createdAfter, 10))
	}

	client := deployments.NewClient(c.server, c.skipVerify)
	return client.ListDeployments(c.token, c.detailLevel, q, c.rawMode)
}
