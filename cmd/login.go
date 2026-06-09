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
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/howeyc/gopass"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mendersoftware/mender-cli/client/useradm"
	"github.com/mendersoftware/mender-cli/log"
)

const (
	argLoginUsername = "username"
	argLoginPassword = "password"
	argLoginToken    = "2fa-code"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to the Mender server (required before other operations).",
	Example: `  mender-cli --server https://hosted.mender.io login --username me@example.com
  mender-cli login --username me@example.com --password secret
  mender-cli login --username me@example.com --2fa-code 123456`,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewLoginCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	loginCmd.Flags().
		StringP(argLoginUsername, "", "", "username, format: email (will prompt if not provided)")
	loginCmd.Flags().StringP(argLoginPassword, "", "", "password (will prompt if not provided)")
	loginCmd.Flags().StringP(argLoginToken, "", "", "two-factor authentication token")
	_ = viper.BindPFlag(argLoginUsername, loginCmd.Flags().Lookup(argLoginUsername))
	_ = viper.BindPFlag(argLoginPassword, loginCmd.Flags().Lookup(argLoginPassword))
}

// LoginCmd implements `mender-cli login`.
type LoginCmd struct {
	server     string
	skipVerify bool
	username   string
	password   string
	token      string
	tokenPath  string
}

// NewLoginCmd validates flags and returns a new LoginCmd.
func NewLoginCmd(cmd *cobra.Command, args []string) (*LoginCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	username := viper.GetString(argLoginUsername)
	password := viper.GetString(argLoginPassword)

	tfaToken, err := cmd.Flags().GetString(argLoginToken)
	if err != nil {
		return nil, err
	}

	token, err := cmd.Flags().GetString(argRootToken)
	if err != nil {
		return nil, err
	}

	if token == "" {
		token, err = getDefaultAuthTokenPath()
		if err != nil {
			return nil, err
		}
	}

	return &LoginCmd{
		server:     server,
		username:   username,
		password:   password,
		token:      tfaToken,
		tokenPath:  token,
		skipVerify: skipVerify,
	}, nil
}

func (c *LoginCmd) Run() error {
	err := c.maybeGetUsername()
	if err != nil {
		return err
	}

	err = c.maybeGetPassword()
	if err != nil {
		return err
	}
	client := useradm.NewClient(c.server, c.skipVerify)
	res, err := client.Login(c.username, c.password, c.token)
	if err != nil {
		return err
	}

	err = c.saveToken(res)
	if err != nil {
		return err
	}

	return nil
}

func (c *LoginCmd) maybeGetUsername() error {
	if c.username == "" {
		fmt.Printf("Username: ")
		reader := bufio.NewReader(os.Stdin)
		str, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		c.username = strings.TrimSuffix(str, "\n")
	}

	return nil
}

func (c *LoginCmd) maybeGetPassword() error {
	if c.password == "" {
		fmt.Printf("Password: ")

		p, err := gopass.GetPasswdMasked()
		if err != nil {
			return err
		}

		c.password = string(p)
	}

	return nil
}

func (c *LoginCmd) saveToken(t []byte) error {
	if err := writeAuthToken(c.tokenPath, t); err != nil {
		return err
	}

	log.Verb("saved token to: " + c.tokenPath)
	log.Info("login successful")

	return nil
}
