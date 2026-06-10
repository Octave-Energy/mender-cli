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
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/inventory"
)

// tagsScopeName is the inventory attribute scope that holds device tags.
const tagsScopeName = "tags"

// argTagDescription is the flag used to set a tag's optional description on
// 'add' and 'set'.
const argTagDescription = "description"

var inventoryDeviceTagsCmd = &cobra.Command{
	Use:   "device-tags",
	Short: "Manage the tags set on a device.",
	Long: "Manage device tags (tags-scope inventory attributes) on a single device.\n\n" +
		"Every subcommand targets one device, selected with either --id or a " +
		"--filter expression that matches exactly one device.",
	ValidArgs: []string{"list", "set", "add", "delete"},
}

func init() {
	inventoryDeviceTagsCmd.AddCommand(inventoryDeviceTagsListCmd)
	inventoryDeviceTagsCmd.AddCommand(inventoryDeviceTagsSetCmd)
	inventoryDeviceTagsCmd.AddCommand(inventoryDeviceTagsAddCmd)
	inventoryDeviceTagsCmd.AddCommand(inventoryDeviceTagsDeleteCmd)

	for _, c := range []*cobra.Command{
		inventoryDeviceTagsListCmd,
		inventoryDeviceTagsSetCmd,
		inventoryDeviceTagsAddCmd,
		inventoryDeviceTagsDeleteCmd,
	} {
		addDeviceTargetFlags(c)
	}
	inventoryDeviceTagsListCmd.Flags().BoolP(argRawMode, "r", false, "output the tags as raw JSON")

	inventoryDeviceTagsAddCmd.Flags().String(argTagDescription, "", "optional human-readable tag description")
	inventoryDeviceTagsSetCmd.Flags().String(
		argTagDescription, "",
		"set the tag description (when omitted, the existing description is preserved)",
	)
}

// ----------------------------------------------------------------------------
// list
// ----------------------------------------------------------------------------

var inventoryDeviceTagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags set on a device.",
	Long: "List the tags (tags-scope inventory attributes) currently set on a " +
		"single device, selected with --id or --filter.",
	Example: `  mender-cli inventory device-tags list --id 0123456789abcdef0123456789abcdef
  mender-cli inventory device-tags list -f hostname=my-gateway
  mender-cli inventory device-tags list --id 0123456789abcdef0123456789abcdef --raw`,
	Args: cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		server, skipVerify, token, deviceID, err := resolveTagsTargetDevice(c)
		CheckErr(err)
		raw, err := c.Flags().GetBool(argRawMode)
		CheckErr(err)

		cli := inventory.NewClient(server, skipVerify)
		tags, _, err := fetchDeviceTags(cli, token, deviceID)
		CheckErr(err)
		CheckErr(printDeviceTags(os.Stdout, deviceID, tags, raw))
	},
}

// ----------------------------------------------------------------------------
// add
// ----------------------------------------------------------------------------

var inventoryDeviceTagsAddCmd = &cobra.Command{
	Use:   "add NAME=VALUE",
	Short: "Add a new tag to a device.",
	Long: "Add a new tag to a single device, selected with --id or --filter.\n\n" +
		"Fails if a tag with the same name already exists; use " +
		"'inventory device-tags set' to change an existing tag's value.",
	Example: `  mender-cli inventory device-tags add environment=production --id 0123456789abcdef0123456789abcdef
  mender-cli inventory device-tags add location=eu-west -f hostname=my-gateway
  mender-cli inventory device-tags add owner=ops --description "team that owns the device" --id 0123456789abcdef0123456789abcdef`,
	Args: cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		name, value, err := parseTagAssignment(args[0])
		CheckErr(err)
		description, err := c.Flags().GetString(argTagDescription)
		CheckErr(err)
		server, skipVerify, token, deviceID, err := resolveTagsTargetDevice(c)
		CheckErr(err)

		cli := inventory.NewClient(server, skipVerify)
		existing, etag, err := fetchDeviceTags(cli, token, deviceID)
		CheckErr(err)
		if tagIndex(existing, name) >= 0 {
			CheckErr(fmt.Errorf(
				"tag %q already exists on device %s; "+
					"use 'inventory device-tags set' to change its value",
				name, deviceID,
			))
		}
		CheckErr(cli.UpsertDeviceTags(
			token, deviceID, etag,
			[]inventory.Tag{{Name: name, Value: value, Description: description}},
		))
		fmt.Printf("added tag %q on device %s\n", name, deviceID)
	},
}

// ----------------------------------------------------------------------------
// set
// ----------------------------------------------------------------------------

var inventoryDeviceTagsSetCmd = &cobra.Command{
	Use:   "set NAME=VALUE",
	Short: "Change the value of an existing tag on a device.",
	Long: "Change the value of an existing tag on a single device, selected " +
		"with --id or --filter.\n\n" +
		"Fails if the tag does not exist; use 'inventory device-tags add' to " +
		"create it. Any existing tag description is preserved.",
	Example: `  mender-cli inventory device-tags set environment=staging --id 0123456789abcdef0123456789abcdef
  mender-cli inventory device-tags set location=eu-central -f hostname=my-gateway
  mender-cli inventory device-tags set owner=sre --description "new owning team" --id 0123456789abcdef0123456789abcdef`,
	Args: cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		name, value, err := parseTagAssignment(args[0])
		CheckErr(err)
		server, skipVerify, token, deviceID, err := resolveTagsTargetDevice(c)
		CheckErr(err)

		cli := inventory.NewClient(server, skipVerify)
		existing, etag, err := fetchDeviceTags(cli, token, deviceID)
		CheckErr(err)
		idx := tagIndex(existing, name)
		if idx < 0 {
			CheckErr(fmt.Errorf(
				"tag %q does not exist on device %s; "+
					"use 'inventory device-tags add' to create it",
				name, deviceID,
			))
		}
		// Preserve the existing description unless --description is given.
		description := existing[idx].Description
		if c.Flags().Changed(argTagDescription) {
			description, err = c.Flags().GetString(argTagDescription)
			CheckErr(err)
		}
		updated := inventory.Tag{
			Name:        name,
			Value:       value,
			Description: description,
		}
		CheckErr(cli.UpsertDeviceTags(token, deviceID, etag, []inventory.Tag{updated}))
		fmt.Printf("updated tag %q on device %s\n", name, deviceID)
	},
}

// ----------------------------------------------------------------------------
// delete
// ----------------------------------------------------------------------------

var inventoryDeviceTagsDeleteCmd = &cobra.Command{
	Use:   "delete NAME",
	Short: "Delete a tag from a device.",
	Long: "Delete a tag from a single device, selected with --id or --filter.\n\n" +
		"Fails if the tag is not set on the device.",
	Example: `  mender-cli inventory device-tags delete environment --id 0123456789abcdef0123456789abcdef
  mender-cli inventory device-tags delete location -f hostname=my-gateway`,
	Args: cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		name := strings.TrimSpace(args[0])
		if name == "" {
			CheckErr(errors.New("a tag name is required"))
		}
		server, skipVerify, token, deviceID, err := resolveTagsTargetDevice(c)
		CheckErr(err)

		cli := inventory.NewClient(server, skipVerify)
		existing, etag, err := fetchDeviceTags(cli, token, deviceID)
		CheckErr(err)
		idx := tagIndex(existing, name)
		if idx < 0 {
			CheckErr(fmt.Errorf("tag %q is not set on device %s", name, deviceID))
		}
		// Deletion is a full replace of the remaining tags (the API has no
		// per-tag delete endpoint).
		remaining := make([]inventory.Tag, 0, len(existing)-1)
		remaining = append(remaining, existing[:idx]...)
		remaining = append(remaining, existing[idx+1:]...)
		CheckErr(cli.ReplaceDeviceTags(token, deviceID, etag, remaining))
		fmt.Printf("deleted tag %q from device %s\n", name, deviceID)
	},
}

// ----------------------------------------------------------------------------
// shared helpers
// ----------------------------------------------------------------------------

// resolveTagsTargetDevice validates the shared --id/--filter targeting flags
// and resolves them to exactly one device id.
func resolveTagsTargetDevice(
	cmd *cobra.Command,
) (server string, skipVerify bool, token, deviceID string, err error) {
	server, skipVerify, err = resolveServerConfig(cmd)
	if err != nil {
		return
	}
	flags := cmd.Flags()
	id, err := flags.GetString(argDeviceID)
	if err != nil {
		return
	}
	filters, err := flags.GetStringSlice(argInventoryFilter)
	if err != nil {
		return
	}
	if id == "" && len(filters) == 0 {
		err = errors.New("one of --id or --filter is required")
		return
	}
	if id != "" && len(filters) > 0 {
		err = errors.New("only one of --id or --filter may be used")
		return
	}
	if err = validateInventoryFilters(filters); err != nil {
		return
	}
	token, err = getAuthToken(cmd)
	if err != nil {
		return
	}
	deviceID, err = resolveDeviceID(server, token, skipVerify, id, filters)
	return
}

// fetchDeviceTags returns the device's tags (attributes in the "tags" scope)
// and the current ETag for optimistic-concurrency writes.
func fetchDeviceTags(
	cli *inventory.Client, token, deviceID string,
) ([]inventory.Tag, string, error) {
	res, err := cli.GetDeviceInventory(token, deviceID)
	if err != nil {
		return nil, "", err
	}
	var dev deviceInventory
	if err := json.Unmarshal(res.Body, &dev); err != nil {
		return nil, "", fmt.Errorf("decode response: %w", err)
	}
	tags := make([]inventory.Tag, 0)
	for _, a := range dev.Attributes {
		if a.Scope != tagsScopeName {
			continue
		}
		tags = append(tags, inventory.Tag{
			Name:        a.Name,
			Value:       tagValueToString(a.Value),
			Description: a.Description,
		})
	}
	return tags, res.ETag, nil
}

// printDeviceTags renders the tags either as raw JSON or a human-readable list.
func printDeviceTags(out io.Writer, deviceID string, tags []inventory.Tag, raw bool) error {
	if raw {
		b, err := json.MarshalIndent(tags, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(b))
		return err
	}
	fmt.Fprintf(out, "Device: %s\n", deviceID)
	fmt.Fprintf(out, "Tags: %d\n", len(tags))
	for _, t := range tags {
		line := fmt.Sprintf("  %s = %s", t.Name, t.Value)
		if t.Description != "" {
			line += fmt.Sprintf(" (%s)", t.Description)
		}
		fmt.Fprintln(out, line)
	}
	return nil
}

// parseTagAssignment splits a NAME=VALUE argument. The value may contain '='.
func parseTagAssignment(s string) (name, value string, err error) {
	i := strings.Index(s, "=")
	if i < 0 {
		return "", "", fmt.Errorf("invalid tag %q, expected NAME=VALUE", s)
	}
	name = strings.TrimSpace(s[:i])
	value = s[i+1:]
	if name == "" {
		return "", "", fmt.Errorf("invalid tag %q, name must not be empty", s)
	}
	if value == "" {
		return "", "", fmt.Errorf("invalid tag %q, value must not be empty", s)
	}
	return name, value, nil
}

// tagIndex returns the index of the tag with the given name, or -1 if absent.
func tagIndex(tags []inventory.Tag, name string) int {
	for i, t := range tags {
		if t.Name == name {
			return i
		}
	}
	return -1
}

// tagValueToString renders an inventory attribute value as a tag string. Tag
// values are strings per the API, but fall back to a JSON encoding otherwise.
func tagValueToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
