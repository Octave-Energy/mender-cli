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
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mendersoftware/mender-cli/log"
)

// CheckErr writes a "FAILURE: <error>" message to standard error and exits the
// process with status 1 when e is non-nil. It is a no-op for a nil error.
func CheckErr(e error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, "FAILURE: %s\n", e.Error())
		os.Exit(1)
	}
}

// resolveServerConfig reads the shared --server and --skip-verify settings from
// the root flags. It returns an error if no server has been configured. It is
// the common preamble for the command constructors.
func resolveServerConfig(cmd *cobra.Command) (server string, skipVerify bool, err error) {
	server = viper.GetString(argRootServer)
	if server == "" {
		return "", false, errors.New("no server configured")
	}
	skipVerify, err = cmd.Flags().GetBool(argRootSkipVerify)
	if err != nil {
		return "", false, err
	}
	return server, skipVerify, nil
}

func migrateAuthToken(oldtoken string, token string) {
	// if needed, migrate token from old to new location
	if _, err := os.Stat(token); !os.IsNotExist(err) {
		// new token exists, no migration
		return
	}

	if _, err := os.Stat(oldtoken); err != nil {
		// old token doesn't exist, no migration
		return
	}

	// Attempt migration, ignore errors (but log them?)
	if err := os.MkdirAll(filepath.Dir(token), 0700); err == nil {
		// log that token was moved?
		_ = os.Rename(oldtoken, token)
	}

	// Cleanup old token directory if empty
	_ = os.Remove(filepath.Dir(oldtoken)) // err on non-empty, ignore.
}

func getDefaultAuthTokenPath() (string, error) {
	cachedir := ""
	userhomedir := ""

	if homeenv := os.Getenv("HOME"); homeenv != "" {
		userhomedir = homeenv
	} else if user, err := user.Current(); err == nil {
		userhomedir = user.HomeDir
	} else {
		return "", errors.New("not able to determine users cache dir")
	}

	if cachehomeenv := os.Getenv("XDG_CACHE_HOME"); cachehomeenv != "" {
		cachedir = cachehomeenv
	} else {
		cachedir = path.Join(userhomedir, ".cache")
	}

	oldtoken := filepath.Join(userhomedir, ".mender", "authtoken")
	token := filepath.Join(cachedir, "mender", "authtoken")

	migrateAuthToken(oldtoken, token)

	return token, nil
}

// writeAuthToken persists the given token bytes to path with 0600 permissions,
// creating any missing parent directories with 0700.
func writeAuthToken(path string, token []byte) error {
	dir := filepath.Dir(path)
	log.Verb("creating directory: " + dir)
	if err := os.MkdirAll(dir, os.ModeDir|0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	if err := os.WriteFile(path, token, 0600); err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	return nil
}

func getAuthToken(cmd *cobra.Command) (string, error) {
	tokenValue, err := cmd.Flags().GetString(argRootTokenValue)
	if err != nil {
		return "", err
	}
	tokenPath, err := cmd.Flags().GetString(argRootToken)
	if err != nil {
		return "", err
	}

	if tokenValue != "" && tokenPath != "" {
		return "", fmt.Errorf("cannot specify both --%s and --%s",
			argRootTokenValue, argRootToken)
	}

	if tokenValue != "" {
		return tokenValue, nil
	}

	if tokenPath == "" {
		tokenPath, err = getDefaultAuthTokenPath()
		if err != nil {
			return "", err
		}
	}

	token, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("please login first: %w", err)
	}
	tokenValue = strings.TrimSpace(string(token))
	return tokenValue, nil
}
