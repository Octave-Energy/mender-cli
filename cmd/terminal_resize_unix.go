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

//go:build !windows

package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// notifyTerminalResize registers ch to receive terminal-resize (SIGWINCH)
// signals, used to propagate the local terminal size to the remote shell.
// Signal delivery also stops when ctx is canceled. The returned function stops
// the delivery of further signals and releases the associated goroutine.
func notifyTerminalResize(ctx context.Context, ch chan os.Signal) func() {
	signal.Notify(ch, syscall.SIGWINCH)
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			signal.Stop(ch)
		case <-stopped:
		}
	}()
	return func() {
		signal.Stop(ch)
		close(stopped)
	}
}
