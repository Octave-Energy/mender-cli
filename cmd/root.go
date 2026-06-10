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

// Package cmd implements the mender-cli command tree built on top of Cobra.
// It wires up the root command and all subcommands (login, artifacts, devices,
// inventory, terminal, port-forward, cp and token) and translates user input
// into calls to the client packages.
package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/mendersoftware/mender-cli/log"
)

const (
	argRootServer     = "server"
	argRootSkipVerify = "skip-verify"
	argRootToken      = "token"
	argRootTokenValue = "token-value"
	argRootVerbose    = "verbose"
	argRootGenerate   = "generate-autocomplete"
)

func init() {
	viper.SetConfigName(".mender-clirc")
	viper.SetConfigType("json")
	viper.AddConfigPath("/etc/mender-cli/")
	viper.AddConfigPath("$HOME/")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Info(fmt.Sprintf("Failed to read config: %s", err))
			os.Exit(1)
		} else {
			log.Info("Configuration file not found. Continuing.")
		}
	} else {
		fmt.Fprintf(os.Stderr, "Using configuration file: %s\n", viper.ConfigFileUsed())
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mender-cli",
	Short: "A general-purpose CLI for the Mender server.",

	//setup global stuff, will run regardless of (sub)command
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		verbose, err := cmd.Flags().GetBool(argRootVerbose)
		CheckErr(err)
		log.Setup(verbose)
		if verbose {
			log.Verb("verbose output is ON")
		}
	},
	ValidArgs: []string{
		"artifacts", "cp", "deployments", "devices", "help", "inventory", "login",
		"port-forward", "releases", "terminal", "token", "version",
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	lflags := rootCmd.LocalFlags()
	lflags.ParseErrorsAllowlist.UnknownFlags = true
	err := lflags.Parse(os.Args)
	if err != nil && !errors.Is(err, pflag.ErrHelp) {
		log.Errf("Failed to parse flags: %s\n", err)
	}
	b, _ := rootCmd.Flags().GetBool(argRootGenerate)
	if b {
		err := rootCmd.GenBashCompletionFile("./autocomplete/autocomplete.sh")
		if err != nil {
			log.Errf("Failed to generate the Bash autocompletion scripts: %s\n", err)
		}
		err = rootCmd.GenZshCompletionFile("./autocomplete/autocomplete.zsh")
		if err != nil {
			log.Errf("Failed to generate the Zsh autocompletion scripts: %s\n", err)
		}
		return
	}
	CheckErr(rootCmd.Execute())
}

func validateConfiguration() {
	server := viper.GetString(argRootServer)
	u, _ := url.Parse(server)
	if u.Scheme == "" {
		viper.Set(argRootServer, "https://"+server)
		log.Info("Protocol is not specified, HTTPS is used by default.")
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().
		StringP(
			argRootServer,
			"",
			"https://hosted.mender.io",
			"root server URL, e.g. 'https://hosted.mender.io'",
		)
	_ = viper.BindPFlag(argRootServer, rootCmd.PersistentFlags().Lookup(argRootServer))
	rootCmd.PersistentFlags().
		BoolP(argRootSkipVerify, "k", false, "skip SSL certificate verification")
	rootCmd.PersistentFlags().StringP(argRootToken, "", "", "JWT token file path")
	rootCmd.PersistentFlags().StringP(argRootTokenValue, "", "", "JWT token value (API key)")
	rootCmd.PersistentFlags().BoolP(argRootVerbose, "v", false, "print verbose output")
	rootCmd.Flags().Bool(argRootGenerate, false, "generate shell completion script")
	_ = rootCmd.Flags().MarkHidden(argRootGenerate)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(artifactsCmd)
	rootCmd.AddCommand(devicesCmd)
	rootCmd.AddCommand(inventoryCmd)
	rootCmd.AddCommand(releasesCmd)
	rootCmd.AddCommand(deploymentsCmd)
	rootCmd.AddCommand(terminalCmd)
	rootCmd.AddCommand(portForwardCmd)
	rootCmd.AddCommand(fileTransferCmd)
	rootCmd.AddCommand(tokenCmd)
	rootCmd.AddCommand(versionCmd)
	validateConfiguration()
}
