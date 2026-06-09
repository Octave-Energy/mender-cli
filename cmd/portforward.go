// Copyright 2022 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package cmd

import (
	"errors"
	"strconv"
	"strings"

	"github.com/mendersoftware/go-lib-micro/ws"
	wspf "github.com/mendersoftware/go-lib-micro/ws/portforward"
	"github.com/spf13/cobra"
)

const (
	argBindHost    = "bind"
	readBuffLength = 4096
	localhost      = "127.0.0.1"
)

var portForwardCmd = &cobra.Command{
	Use: "port-forward DEVICE_ID [tcp|udp/]LOCAL_PORT[:REMOTE_PORT]" +
		" [[tcp|udp/]LOCAL_PORT[:REMOTE_PORT]...]",
	Short: "Forward one or more local ports to remote port(s) on the device.",
	Long: "This command supports both TCP and UDP port-forwarding.\n\n" +
		"The port specification can be prefixed with \"tcp/\" or \"udp/\".\n" +
		"If no prefix is specified, TCP is the default.\n\n" +
		"REMOTE_PORT can also be specified in the form REMOTE_HOST:REMOTE_PORT, making\n" +
		"it possible to port-forward to third hosts running in the device's network.\n" +
		"In this case, the specification will be LOCAL_PORT:REMOTE_HOST:REMOTE_PORT.\n\n" +
		"You can specify multiple port mapping specifications.",
	Example: "  mender-cli port-forward DEVICE_ID 8000:8000\n" +
		"  mender-cli port-forward DEVICE_ID udp/8000:8000\n" +
		"  mender-cli port-forward DEVICE_ID tcp/8000:192.168.1.1:8000\n" +
		"  mender-cli port-forward --id DEVICE_ID 8000:8000\n" +
		"  mender-cli port-forward -f hostname=my-gateway 8000:8000",
	Args: portForwardArgs,
	Run: func(c *cobra.Command, args []string) {
		cmd, err := NewPortForwardCmd(c, args)
		CheckErr(err)
		CheckErr(cmd.Run())
	},
}

var portForwardMaxDuration = sessionMaxDuration

var errPortForwardNotImplemented = errors.New(
	"port forward not implemented or enabled on the device",
)
var errRestart = errors.New("restart")

func init() {
	portForwardCmd.Flags().StringP(argBindHost, "", localhost, "binding host")
	addDeviceTargetFlags(portForwardCmd)
}

// portForwardArgs validates positional args, accounting for whether the device
// is provided via --id/--filter (then all positionals are port mappings) or as
// the leading positional argument (legacy form).
func portForwardArgs(cmd *cobra.Command, args []string) error {
	hasDeviceFlag, err := deviceTargetProvided(cmd)
	if err != nil {
		return err
	}
	if hasDeviceFlag {
		if len(args) < 1 {
			return errors.New("requires at least one port mapping")
		}
		return nil
	}
	if len(args) < 2 {
		return errors.New(
			"requires DEVICE_ID and at least one port mapping (or use --id/--filter)",
		)
	}
	return nil
}

const (
	protocolTCP = "tcp"
	protocolUDP = "udp"
)

type portMapping struct {
	Protocol   string
	LocalPort  uint16
	RemoteHost string
	RemotePort uint16
}

// PortForwardCmd handles the port-forward command
type PortForwardCmd struct {
	proto        ws.ProtoType
	server       string
	token        string
	skipVerify   bool
	deviceID     string
	sessionID    string
	bindingHost  string
	portMappings []portMapping
	recvChans    map[string]chan *ws.ProtoMsg
	running      bool
	stop         chan struct{}
	err          error
}

func getPortMappings(args []string) ([]portMapping, error) {
	var err error
	portMappings := []portMapping{}
	for _, arg := range args {
		remoteHost := localhost
		protocol := wspf.PortForwardProtocolTCP
		if strings.Contains(arg, "/") {
			parts := strings.SplitN(arg, "/", 2)
			switch parts[0] {
			case protocolTCP:
				protocol = protocolTCP
			case protocolUDP:
				protocol = protocolUDP
			default:
				return nil, errors.New("unknown protocol: " + parts[0])
			}
			arg = parts[1]
		}
		var localPort, remotePort int
		if strings.Contains(arg, ":") {
			parts := strings.SplitN(arg, ":", 3)
			if len(parts) == 3 {
				remoteHost = parts[1]
				parts = []string{parts[0], parts[2]}
			}
			localPort, err = strconv.Atoi(parts[0])
			if err != nil || localPort < 0 || localPort > 65535 {
				return nil, errors.New("invalid port number: " + parts[0])
			}
			remotePort, err = strconv.Atoi(parts[1])
			if err != nil || remotePort < 0 || remotePort > 65535 {
				return nil, errors.New("invalid port number: " + parts[1])
			}
		} else {
			port, err := strconv.Atoi(arg)
			if err != nil || port < 0 || port > 65535 {
				return nil, errors.New("invalid port number: " + arg)
			}
			localPort = port
			remotePort = port
		}
		portMappings = append(portMappings, portMapping{
			Protocol:   protocol,
			LocalPort:  uint16(localPort),
			RemoteHost: remoteHost,
			RemotePort: uint16(remotePort),
		})
	}
	return portMappings, nil
}

// NewPortForwardCmd returns a new PortForwardCmd
func NewPortForwardCmd(cmd *cobra.Command, args []string) (*PortForwardCmd, error) {
	server, skipVerify, err := resolveServerConfig(cmd)
	if err != nil {
		return nil, err
	}

	bindingHost, err := cmd.Flags().GetString(argBindHost)
	if err != nil {
		return nil, err
	}

	token, err := getAuthToken(cmd)
	if err != nil {
		return nil, err
	}

	idFlag, err := cmd.Flags().GetString(argDeviceID)
	if err != nil {
		return nil, err
	}
	filters, err := cmd.Flags().GetStringSlice(argInventoryFilter)
	if err != nil {
		return nil, err
	}

	var deviceID string
	var portArgs []string
	if idFlag != "" || len(filters) > 0 {
		if idFlag != "" && len(filters) > 0 {
			return nil, errors.New("specify only one of --id or --filter")
		}
		if err := validateInventoryFilters(filters); err != nil {
			return nil, err
		}
		deviceID, err = resolveDeviceID(server, token, skipVerify, idFlag, filters)
		if err != nil {
			return nil, err
		}
		portArgs = args
	} else {
		deviceID = args[0]
		portArgs = args[1:]
	}

	portMappings, err := getPortMappings(portArgs)
	if err != nil {
		return nil, err
	}

	return &PortForwardCmd{
		server:       server,
		token:        token,
		skipVerify:   skipVerify,
		deviceID:     deviceID,
		bindingHost:  bindingHost,
		portMappings: portMappings,
		recvChans:    make(map[string]chan *ws.ProtoMsg),
		stop:         make(chan struct{}),
	}, nil
}
