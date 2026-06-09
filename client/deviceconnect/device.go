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

package deviceconnect

// Device is a device as reported by the deviceconnect API, together with its
// current connection status.
type Device struct {
	ID     string
	Status string
}

// StatusConnected is the Device.Status value indicating that the device is
// currently connected and reachable for terminal, port-forward and file
// transfer operations.
const StatusConnected = "connected"
