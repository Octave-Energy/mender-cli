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

var devicesCmd = &cobra.Command{
	Use:       "devices",
	Short:     "Operations on mender devices.",
	ValidArgs: []string{"list", "get", "count"},
}

// deviceAuthStatuses are the valid device authentication status values accepted
// by the device authentication API's status filter.
var deviceAuthStatuses = []string{
	"pending", "accepted", "rejected", "preauthorized", "noauth",
}

// deviceStatusCompletion provides shell completion for the --status flag.
func deviceStatusCompletion(
	_ *cobra.Command, _ []string, _ string,
) ([]string, cobra.ShellCompDirective) {
	return deviceAuthStatuses, cobra.ShellCompDirectiveNoFileComp
}

// validateDeviceStatus ensures status is empty or one of deviceAuthStatuses.
func validateDeviceStatus(status string) error {
	if status == "" {
		return nil
	}
	for _, s := range deviceAuthStatuses {
		if status == s {
			return nil
		}
	}
	return fmt.Errorf(
		"invalid --status %q, expected one of: %s",
		status, strings.Join(deviceAuthStatuses, ", "),
	)
}

func init() {
	devicesCmd.AddCommand(devicesListCmd)
	devicesCmd.AddCommand(devicesGetCmd)
	devicesCmd.AddCommand(devicesCountCmd)
}
