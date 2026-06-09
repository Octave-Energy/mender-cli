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

// Flag names shared across multiple commands. Keeping them in one place avoids
// drift between the commands that declare and read the same flag.
const (
	// argRawMode prints the raw JSON returned by the server.
	argRawMode = "raw"
	// argDetailLevel selects how much detail a list/get command prints [0..3].
	argDetailLevel = "detail"
	// argPage and argPerPage control pagination on the paginated list commands.
	argPage    = "page"
	argPerPage = "per-page"
	// argWithoutProgress disables the progress bar on transfer commands.
	argWithoutProgress = "no-progress"
)
