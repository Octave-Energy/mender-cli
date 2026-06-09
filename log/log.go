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

// Package log provides a thin leveled logging wrapper around the standard
// library slog package used throughout mender-cli. Output is written to
// standard error and the verbosity can be raised with Setup.
package log

import (
	"fmt"
	"log/slog"
	"os"
)

var (
	logOpts = slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	logger = slog.New(
		slog.NewTextHandler(os.Stderr, &logOpts),
	)
)

// Setup configures the global logger. When verb is true, debug-level
// (verbose) messages are emitted in addition to the default info level.
func Setup(verb bool) {
	if verb {
		logOpts.Level = slog.LevelDebug
		logger = slog.New(slog.NewTextHandler(os.Stderr, &logOpts))
	}
}

// Err logs msg at the error level.
func Err(msg string) {
	logger.Error(msg)
}

// Errf logs a printf-style formatted message at the error level.
func Errf(msg string, args ...any) {
	logger.Error(fmt.Sprintf(msg, args...))
}

// Verb logs msg at the debug (verbose) level.
func Verb(msg string) {
	logger.Debug(msg)
}

// Verbf logs a printf-style formatted message at the debug (verbose) level.
func Verbf(msg string, args ...any) {
	logger.Debug(fmt.Sprintf(msg, args...))
}

// Info logs msg at the info level.
func Info(msg string) {
	logger.Info(msg)
}

// Infof logs a printf-style formatted message at the info level.
func Infof(msg string, args ...any) {
	logger.Info(fmt.Sprintf(msg, args...))
}
