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
	"io"
)

type inventoryAttribute struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	Value       any    `json:"value"`
	Description string `json:"description,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

type deviceInventory struct {
	ID         string               `json:"id"`
	UpdatedTs  string               `json:"updated_ts"`
	Attributes []inventoryAttribute `json:"attributes"`
}

func listInventoryDevice(out io.Writer, dev deviceInventory, detailLevel int) {
	nAttrs := len(dev.Attributes)
	fmt.Fprintf(out, "ID: %s\n", dev.ID)
	fmt.Fprintf(out, "UpdatedTs: %s\n", dev.UpdatedTs)
	fmt.Fprintf(out, "Attributes: %d\n", nAttrs)
	if detailLevel >= 1 {
		for _, a := range dev.Attributes {
			val := formatInventoryAttributeValue(a.Value)
			line := fmt.Sprintf("  [%s] %s = %s", a.Scope, a.Name, val)
			if a.Description != "" {
				line += fmt.Sprintf(" (%s)", a.Description)
			}
			fmt.Fprintln(out, line)
			if a.Timestamp != "" {
				fmt.Fprintf(out, "    timestamp: %s\n", a.Timestamp)
			}
		}
	}
	fmt.Fprintf(out, "--------------------------------------------------------------------------------\n")
}

func formatInventoryAttributeValue(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
