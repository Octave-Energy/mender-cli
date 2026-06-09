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

//go:build windows

package cmd

import (
	"context"
	"os"
	"syscall"
	"time"
)

// terminalResizePollInterval is how often the local terminal size is sampled on
// Windows, which has no SIGWINCH signal to detect resizes.
const terminalResizePollInterval = 500 * time.Millisecond

// notifyTerminalResize starts a goroutine that periodically nudges ch so the
// resize loop re-reads the terminal size and propagates any change to the
// remote shell. This emulates SIGWINCH on Windows via polling. The returned
// function stops the poller.
func notifyTerminalResize(ctx context.Context, ch chan os.Signal) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(terminalResizePollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				// Non-blocking nudge: the resize loop does the actual
				// size comparison, so dropping a tick is harmless.
				select {
				case ch <- syscall.Signal(0):
				default:
				}
			}
		}
	}()
	return func() { close(done) }
}
