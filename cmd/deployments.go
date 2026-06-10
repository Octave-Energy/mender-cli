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
	"strings"

	"github.com/spf13/cobra"
)

const (
	argDeploymentID       = "id"
	argDeploymentName     = "name"
	argDeploymentStatus   = "status"
	argDeploymentType     = "type"
	argDeploymentSort     = "sort"
	argDeploymentCreatedB = "created-before"
	argDeploymentCreatedA = "created-after"
	argDeploymentDevice   = "device"
	argSearchGroup        = "group"
)

// deploymentStatuses are the valid deployment status filter values.
var deploymentStatuses = []string{"inprogress", "finished", "pending"}

// deploymentTypes are the valid deployment type filter values.
var deploymentTypes = []string{"software", "configuration"}

// deploymentSortValues are the valid sort directions for the deployment list.
var deploymentSortValues = []string{"asc", "desc"}

// deploymentDeviceStatuses are the valid per-device status values within a
// deployment (union of the API's DeploymentDeviceStatus and
// DeploymentDeviceStatusGet enums).
var deploymentDeviceStatuses = []string{
	"failure", "aborted",
	"pause_before_installing", "pause_before_committing", "pause_before_rebooting",
	"downloading", "installing", "rebooting",
	"pending", "success", "noartifact", "artifact_too_big",
	"already-installed", "decommissioned",
	"pause", "active", "finished",
}

func enumCompletion(values []string) func(
	*cobra.Command, []string, string,
) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}

func validateEnum(flag, value string, allowed []string) error {
	if value == "" {
		return nil
	}
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf(
		"invalid --%s %q, expected one of: %s",
		flag, value, strings.Join(allowed, ", "),
	)
}

var deploymentsCmd = &cobra.Command{
	Use:       "deployments",
	Short:     "Operations on Mender deployments.",
	ValidArgs: []string{"list", "count", "search", "get", "stats", "devices", "log"},
}

func init() {
	deploymentsCmd.AddCommand(deploymentsListCmd)
	deploymentsCmd.AddCommand(deploymentsCountCmd)
	deploymentsCmd.AddCommand(deploymentsSearchCmd)
	deploymentsCmd.AddCommand(deploymentsGetCmd)
	deploymentsCmd.AddCommand(deploymentsStatsCmd)
	deploymentsCmd.AddCommand(deploymentsDevicesCmd)
	deploymentsCmd.AddCommand(deploymentsLogCmd)
}
