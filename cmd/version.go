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
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// Build-time version information. These variables are populated by the linker
// via -ldflags "-X github.com/mendersoftware/mender-cli/cmd.<Var>=<value>" (see
// the Makefile). When built without those flags they fall back to "unknown".
var (
	// Version is the mender-cli release version (a git tag or short commit).
	Version string
	// Revision is the git commit the binary was built from.
	Revision string
	// Branch is the git branch the binary was built from.
	Branch string
	// BuildDate is the build timestamp.
	BuildDate string
	// BuildUser is the user@host that produced the build.
	BuildUser string
	// Tags is the set of build tags the binary was compiled with.
	Tags string
)

const argVersionShort = "short"

// versionInfoUnknown is the placeholder used for build-time fields that were
// not injected at link time.
const versionInfoUnknown = "unknown"

// orUnknown returns s, or versionInfoUnknown when s is empty.
func orUnknown(s string) string {
	if s == "" {
		return versionInfoUnknown
	}
	return s
}

// versionSummary returns the single-line version string, e.g.
// "mender-cli, version 1.2.3 (branch: master, revision: abc1234)".
func versionSummary() string {
	return fmt.Sprintf(
		"mender-cli, version %s (branch: %s, revision: %s)",
		orUnknown(Version),
		orUnknown(Branch),
		orUnknown(Revision),
	)
}

// versionDetails returns the full, multi-line version report including build
// metadata and the runtime Go version and platform.
func versionDetails() string {
	var b strings.Builder
	b.WriteString(versionSummary())
	b.WriteByte('\n')
	fields := []struct{ name, value string }{
		{"build user", orUnknown(BuildUser)},
		{"build date", orUnknown(BuildDate)},
		{"go version", runtime.Version()},
		{"platform", runtime.GOOS + "/" + runtime.GOARCH},
		{"tags", orUnknown(Tags)},
	}
	for _, f := range fields {
		fmt.Fprintf(&b, "  %-16s%s\n", f.name+":", f.value)
	}
	return strings.TrimRight(b.String(), "\n")
}

// versionCmd prints the version and build information.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and build information.",
	Long: "Print the mender-cli version together with build metadata: " +
		"git branch and revision, build user and date, the Go version and " +
		"platform it was built for, and the build tags used.",
	Example: "  mender-cli version\n  mender-cli version --short",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		short, _ := cmd.Flags().GetBool(argVersionShort)
		if short {
			fmt.Println(versionSummary())
			return
		}
		fmt.Println(versionDetails())
	},
}

func init() {
	versionCmd.Flags().Bool(argVersionShort, false, "print only the one-line version summary")
}
