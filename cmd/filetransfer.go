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

	"github.com/spf13/cobra"

	"github.com/mendersoftware/mender-cli/client/deviceconnect"
	"github.com/mendersoftware/mender-cli/log"
)

const (
	argSourcePath = "source-path"
	argDestPath   = "dest-path"
	argUpload     = "upload"
	argDownload   = "download"

	fileTransferExamples = `  mender-cli cp --id DEVICE_ID --upload --source-path ./app.conf --dest-path /etc/app.conf
  mender-cli cp --id DEVICE_ID --download --source-path /var/log/syslog --dest-path ./syslog
  mender-cli cp --filter hostname=my-gateway --download --source-path /etc/hosts --dest-path ./hosts`
)

var fileTransferCmd = &cobra.Command{
	Use:     "cp",
	Short:   "Transfer files to or from a device.",
	Long:    "A CLI interface for copying files to or from connected devices in your setup.",
	Example: fileTransferExamples,
	Args:    cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewFileTransfer(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

func init() {
	addDeviceTargetFlags(fileTransferCmd)
	fileTransferCmd.Flags().String(argSourcePath, "", "source file path")
	fileTransferCmd.Flags().String(argDestPath, "", "destination file path")
	fileTransferCmd.Flags().Bool(argUpload, false, "copy from the local host to the device")
	fileTransferCmd.Flags().Bool(argDownload, false, "copy from the device to the local host")
}

// FileTransferCmd implements `mender-cli cp` (device file upload/download).
type FileTransferCmd struct {
	server      string
	skipVerify  bool
	deviceID    string
	source      string
	destination string
	upload      bool
	token       string
}

// NewFileTransfer validates flags and returns a new FileTransferCmd.
func NewFileTransfer(cmd *cobra.Command, args []string) (*FileTransferCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	idFlag, err := flags.GetString(argDeviceID)
	if err != nil {
		return nil, err
	}
	filters, err := flags.GetStringSlice(argInventoryFilter)
	if err != nil {
		return nil, err
	}
	if idFlag == "" && len(filters) == 0 {
		return nil, errors.New("one of --id or --filter is required")
	}
	if idFlag != "" && len(filters) > 0 {
		return nil, errors.New("only one of --id or --filter may be used")
	}
	if err := validateInventoryFilters(filters); err != nil {
		return nil, err
	}

	source, err := flags.GetString(argSourcePath)
	if err != nil {
		return nil, err
	}
	destination, err := flags.GetString(argDestPath)
	if err != nil {
		return nil, err
	}
	if source == "" || destination == "" {
		return nil, errors.New("both --source-path and --dest-path are required")
	}

	upload, err := flags.GetBool(argUpload)
	if err != nil {
		return nil, err
	}
	download, err := flags.GetBool(argDownload)
	if err != nil {
		return nil, err
	}
	if upload == download {
		return nil, errors.New("exactly one of --upload or --download is required")
	}

	deviceID, err := resolveDeviceID(server, token, skipVerify, idFlag, filters)
	if err != nil {
		return nil, err
	}

	return &FileTransferCmd{
		server:      server,
		skipVerify:  skipVerify,
		token:       token,
		deviceID:    deviceID,
		source:      source,
		destination: destination,
		upload:      upload,
	}, nil
}

func (c *FileTransferCmd) Run() error {
	if c.upload {
		return c.uploadFile()
	}
	return c.downloadFile()
}

func (c *FileTransferCmd) checkDevice(deviceID string) error {
	client := deviceconnect.NewClient(c.server, c.token, c.skipVerify)
	return ensureDeviceConnected(client, deviceID)
}

func (c *FileTransferCmd) uploadFile() error {
	if err := c.checkDevice(c.deviceID); err != nil {
		return err
	}
	d := &deviceconnect.DeviceSpec{DeviceID: c.deviceID, DevicePath: c.destination}
	client := deviceconnect.NewFileTransferClient(c.server, c.token, c.skipVerify)
	if err := client.Upload(c.source, d); err != nil {
		return err
	}
	log.Infof("Successfully uploaded the file %q to device %q at location %q\n",
		c.source, d.DeviceID, d.DevicePath)
	return nil
}

func (c *FileTransferCmd) downloadFile() error {
	if err := c.checkDevice(c.deviceID); err != nil {
		return err
	}
	d := &deviceconnect.DeviceSpec{DeviceID: c.deviceID, DevicePath: c.source}
	client := deviceconnect.NewFileTransferClient(c.server, c.token, c.skipVerify)
	if err := client.Download(d, c.destination); err != nil {
		return err
	}
	log.Infof("Successfully downloaded the file: %q from device %q to %q\n",
		d.DevicePath, d.DeviceID, c.destination)
	return nil
}
