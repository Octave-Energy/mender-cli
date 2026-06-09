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
	"compress/gzip"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mendersoftware/mender-cli/log"
)

const (
	deviceIDMaxLength     = 64
	terminalTypeMaxLength = 32
	terminalTypeDefault   = "xterm-256color"
)

// TerminalRecordingHeader is the fixed-size header written at the start of a
// terminal session recording.
type TerminalRecordingHeader struct {
	Version        uint8
	DeviceID       [deviceIDMaxLength]byte
	TerminalType   [terminalTypeMaxLength]byte
	TerminalWidth  int16
	TerminalHeight int16
	Timestamp      int64
}

const (
	terminalRecordingVersion = 1
)

// TerminalRecordingType identifies the kind of a recorded data chunk.
type TerminalRecordingType int8

// TerminalRecordingData is a single recorded chunk of terminal data.
type TerminalRecordingData struct {
	Type TerminalRecordingType
	Data []byte
}

const (
	terminalRecordingOutput TerminalRecordingType = iota
)

// record streams terminal output to the gzip-compressed recording file until
// the session stops.
func (c *TerminalCmd) record() {
	f, err := os.Create(c.recordFile)
	if err != nil {
		log.Err(fmt.Sprintf("Can't create recording file: %s: %s", c.recordFile, err.Error()))
	}
	defer f.Close()

	fz := gzip.NewWriter(f)
	defer fz.Close()

	data := TerminalRecordingHeader{
		Version:        terminalRecordingVersion,
		Timestamp:      time.Now().Unix(),
		TerminalWidth:  defaultTermWidth,
		TerminalHeight: defaultTermHeight,
	}
	copy(data.DeviceID[:], []byte(c.deviceID))
	copy(data.TerminalType[:], []byte(terminalTypeDefault))
	err = binary.Write(fz, binary.LittleEndian, data)
	if err != nil {
		log.Err(fmt.Sprintf("Header write failed: %s", err.Error()))
	}
	err = fz.Flush()
	if err != nil {
		log.Err(fmt.Sprintf("Header flush failed: %s", err.Error()))
	}

	log.Info(fmt.Sprintf("Recording to file: %s", c.recordFile))

	e := gob.NewEncoder(fz)
	for {
		select {
		case <-c.stopRecording:
			return
		case terminalOutput := <-c.terminalOutputChan:
			o := TerminalRecordingData{
				Type: terminalRecordingOutput,
				Data: terminalOutput,
			}
			err = e.Encode(o)
			fz.Flush()
			if err != nil {
				log.Err(fmt.Sprintf("Error encoding %q: %s", string(terminalOutput), err.Error()))
				return
			}
		}
	}
}

// playback replays a previously recorded session to w, reproducing the original
// pacing between chunks.
func (c *TerminalCmd) playback(w io.Writer) error {
	f, err := os.Open(c.playbackFile)
	if err != nil {
		log.Err(fmt.Sprintf("Can't open %s: %s", c.playbackFile, err.Error()))
		return err
	}
	defer f.Close()

	fz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer fz.Close()

	var header TerminalRecordingHeader
	err = binary.Read(fz, binary.LittleEndian, &header)
	if err != nil {
		log.Err(fmt.Sprintf("Can't read header: %s", err.Error()))
		return err
	}

	dateTime := time.Unix(header.Timestamp, 0)

	log.Info(fmt.Sprintf("Playing back from file: %s", c.playbackFile))
	log.Info(fmt.Sprintf("Device ID: %s", string(header.DeviceID[:])))
	log.Info(fmt.Sprintf("Terminal type: %s", string(header.TerminalType[:])))
	log.Info(fmt.Sprintf("Terminal size: %dx%d", header.TerminalWidth, header.TerminalHeight))
	log.Info(fmt.Sprintf("Timestamp: %s", dateTime.Format(time.UnixDate)))
	log.Info("")

	d := gob.NewDecoder(fz)
	for {
		var o TerminalRecordingData
		err = d.Decode(&o)
		if err != nil {
			if err != io.EOF {
				log.Err(fmt.Sprintf("Decoding error: %s", err.Error()))
				return err
			}
			break
		}
		if o.Type == terminalRecordingOutput {
			_, err = w.Write(o.Data)
			if err != nil {
				log.Err(fmt.Sprintf("Writting error: %s", err.Error()))
				return err
			}
		}
		time.Sleep(playbackSleep)
	}
	log.Info("\r")
	return nil
}
