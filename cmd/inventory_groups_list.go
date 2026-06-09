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
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/inventory"
	"github.com/mendersoftware/mender-cli/log"
)

const (
	argGroupsListStatus         = "status"
	inventoryGroupsListExamples = `  mender-cli inventory groups list
  mender-cli inventory groups list --raw
  mender-cli inventory groups list --status accepted --raw`
)

var inventoryGroupsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List inventory static group names.",
	Example: inventoryGroupsListExamples,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewInventoryGroupsListCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	inventoryGroupsListCmd.Flags().BoolP(argRawMode, "r", false, "output the raw JSON returned by the Mender server")
	inventoryGroupsListCmd.Flags().String(argGroupsListStatus, "", "only groups for devices with this auth set status")
}

// InventoryGroupsListCmd implements `mender-cli inventory groups list`.
type InventoryGroupsListCmd struct {
	server     string
	skipVerify bool
	token      string
	rawMode    bool
	status     string
}

// NewInventoryGroupsListCmd validates flags and returns a new InventoryGroupsListCmd.
func NewInventoryGroupsListCmd(cmd *cobra.Command, args []string) (*InventoryGroupsListCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()
	rawMode, err := flags.GetBool(argRawMode)
	if err != nil {
		return nil, err
	}
	status, err := flags.GetString(argGroupsListStatus)
	if err != nil {
		return nil, err
	}
	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}
	return &InventoryGroupsListCmd{
		server:     server,
		skipVerify: skipVerify,
		token:      token,
		rawMode:    rawMode,
		status:     status,
	}, nil
}

func (c *InventoryGroupsListCmd) Run() error {
	cli := inventory.NewClient(c.server, c.skipVerify)
	res, err := cli.ListGroups(c.token, c.status)
	if err != nil {
		return err
	}
	if res.TotalCount != "" || res.Link != "" {
		log.Verb(fmt.Sprintf("X-Total-Count: %s", res.TotalCount))
		log.Verb(fmt.Sprintf("Link: %s", res.Link))
	}
	if c.rawMode {
		_, err := os.Stdout.Write(res.Body)
		return err
	}
	var names []string
	if err := json.Unmarshal(res.Body, &names); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	out := os.Stdout
	for _, n := range names {
		fmt.Fprintln(out, n)
	}
	return nil
}
