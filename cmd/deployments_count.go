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
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/deployments"
)

var deploymentsCountCmd = &cobra.Command{
	Use:   "count",
	Short: "Count deployments, optionally filtered.",
	Long: "Count deployments from the Mender server.\n\n" +
		"This efficiently returns only the total number of deployments " +
		"without listing them. Use the same filters as 'deployments list' " +
		"(--id, --name, --status, --type, --created-before/--created-after) " +
		"to count a subset.",
	Example: `  mender-cli deployments count
  mender-cli deployments count --status inprogress
  mender-cli deployments count --type software`,
	Args: cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewDeploymentsCountCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	deploymentsCountCmd.Flags().StringArray(
		argDeploymentID, nil, "filter by deployment id; repeat to match several")
	deploymentsCountCmd.Flags().StringArray(
		argDeploymentName, nil, "filter by deployment name; repeat to match several")
	deploymentsCountCmd.Flags().String(
		argDeploymentStatus, "", "filter by status: "+strings.Join(deploymentStatuses, ", "))
	deploymentsCountCmd.Flags().String(
		argDeploymentType, "", "filter by type: "+strings.Join(deploymentTypes, ", "))
	deploymentsCountCmd.Flags().Int64(
		argDeploymentCreatedB, 0, "only deployments created before this Unix timestamp (UTC)")
	deploymentsCountCmd.Flags().Int64(
		argDeploymentCreatedA, 0, "only deployments created after this Unix timestamp (UTC)")
	_ = deploymentsCountCmd.RegisterFlagCompletionFunc(
		argDeploymentStatus, enumCompletion(deploymentStatuses))
	_ = deploymentsCountCmd.RegisterFlagCompletionFunc(
		argDeploymentType, enumCompletion(deploymentTypes))
}

// DeploymentsCountCmd implements `mender-cli deployments count`.
type DeploymentsCountCmd struct {
	server        string
	skipVerify    bool
	token         string
	ids           []string
	names         []string
	status        string
	depType       string
	createdBefore int64
	createdAfter  int64
}

// NewDeploymentsCountCmd validates flags and returns a new DeploymentsCountCmd.
func NewDeploymentsCountCmd(cmd *cobra.Command, args []string) (*DeploymentsCountCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()

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

	return &DeploymentsCountCmd{
		server:        server,
		token:         token,
		skipVerify:    skipVerify,
		ids:           ids,
		names:         names,
		status:        status,
		depType:       depType,
		createdBefore: createdBefore,
		createdAfter:  createdAfter,
	}, nil
}

func (c *DeploymentsCountCmd) Run() error {
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
	if c.createdBefore > 0 {
		q.Set("created_before", strconv.FormatInt(c.createdBefore, 10))
	}
	if c.createdAfter > 0 {
		q.Set("created_after", strconv.FormatInt(c.createdAfter, 10))
	}

	client := deployments.NewClient(c.server, c.skipVerify)
	count, err := client.CountDeployments(c.token, q)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, count)
	return nil
}
