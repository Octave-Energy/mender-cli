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
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/deployments"
)

var deploymentsSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Find deployments by the group or device they target.",
	Long: "Find deployments by who they target. Provide exactly one of:\n\n" +
		"  --group   a static group name\n" +
		"  --device  a device id (verbatim)\n" +
		"  -f/--filter  inventory attributes that must resolve to exactly one " +
		"device (e.g. mac=00:11:.., scope/name=value)\n\n" +
		"Matching is done against each deployment's declared targeting " +
		"(its groups and inventory filter terms); a device targeted only " +
		"implicitly through a group deployment is not matched by --device. " +
		"Narrow the scan further with --status/--type.",
	Example: `  mender-cli deployments search --group edgebox-PROD
  mender-cli deployments search --device 2bf0f1ab-cd52-4b74-af6e-ce46e8b9f4a4
  mender-cli deployments search --filter mac=00:11:22:33:44:55
  mender-cli deployments search --group edgebox-PROD --status inprogress --detail 2`,
	Args: cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewDeploymentsSearchCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	deploymentsSearchCmd.Flags().String(argSearchGroup, "", "group name to search for")
	deploymentsSearchCmd.Flags().String(
		argDeploymentDevice, "", "device id to search for (verbatim)")
	deploymentsSearchCmd.Flags().StringSliceP(
		argInventoryFilter,
		"f",
		nil,
		"find the target device by attribute: name=value or scope/name=value "+
			"(scope defaults to inventory); must match exactly one device; repeat -f for multiple",
	)
	deploymentsSearchCmd.Flags().IntP(argDetailLevel, "d", 0, "deployments detail level [0..3]")
	deploymentsSearchCmd.Flags().String(
		argDeploymentStatus, "", "narrow by status: "+strings.Join(deploymentStatuses, ", "))
	deploymentsSearchCmd.Flags().String(
		argDeploymentType, "", "narrow by type: "+strings.Join(deploymentTypes, ", "))
	_ = deploymentsSearchCmd.RegisterFlagCompletionFunc(
		argDeploymentStatus, enumCompletion(deploymentStatuses))
	_ = deploymentsSearchCmd.RegisterFlagCompletionFunc(
		argDeploymentType, enumCompletion(deploymentTypes))
	deploymentsSearchCmd.Flags().BoolP(
		argRawMode, "r", false, "output the raw JSON returned by the Mender server")
}

// DeploymentsSearchCmd implements `mender-cli deployments search`.
type DeploymentsSearchCmd struct {
	server      string
	skipVerify  bool
	token       string
	detailLevel int
	rawMode     bool
	group       string
	device      string
	filters     []string
	status      string
	depType     string
}

// NewDeploymentsSearchCmd validates flags and returns a new DeploymentsSearchCmd.
func NewDeploymentsSearchCmd(cmd *cobra.Command, args []string) (*DeploymentsSearchCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()

	group, err := flags.GetString(argSearchGroup)
	if err != nil {
		return nil, err
	}

	device, err := flags.GetString(argDeploymentDevice)
	if err != nil {
		return nil, err
	}

	filters, err := flags.GetStringSlice(argInventoryFilter)
	if err != nil {
		return nil, err
	}

	modes := 0
	if group != "" {
		modes++
	}
	if device != "" {
		modes++
	}
	if len(filters) > 0 {
		modes++
	}
	if modes != 1 {
		return nil, errors.New(
			"provide exactly one of --group, --device or --filter to search by")
	}

	detailLevel, err := flags.GetInt(argDetailLevel)
	if err != nil {
		return nil, err
	}

	rawMode, err := flags.GetBool(argRawMode)
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

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	return &DeploymentsSearchCmd{
		server:      server,
		token:       token,
		skipVerify:  skipVerify,
		detailLevel: detailLevel,
		rawMode:     rawMode,
		group:       group,
		device:      device,
		filters:     filters,
		status:      status,
		depType:     depType,
	}, nil
}

func (c *DeploymentsSearchCmd) Run() error {
	group := c.group
	deviceID := c.device
	if len(c.filters) > 0 {
		resolved, err := resolveDeviceID(c.server, c.token, c.skipVerify, "", c.filters)
		if err != nil {
			return err
		}
		deviceID = resolved
	}

	q := url.Values{}
	if c.status != "" {
		q.Set("status", c.status)
	}
	if c.depType != "" {
		q.Set("type", c.depType)
	}

	client := deployments.NewClient(c.server, c.skipVerify)
	return client.SearchDeployments(c.token, c.detailLevel, q, group, deviceID, c.rawMode)
}
