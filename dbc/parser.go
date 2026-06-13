// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dbc parses DBC (CAN database) files and decodes signal values
// from CAN frames.
//
// A DBC file describes the messages and signals on a CAN network.
// Each message has an arbitration ID and a set of named signals with
// bit positions, lengths, scaling factors, and units.
//
//fusa:req REQ-DBC-001
//fusa:req REQ-DBC-002
//fusa:req REQ-DBC-003
package dbc

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// ByteOrder indicates the bit-layout order for a signal.
type ByteOrder int

const (
	// LittleEndian (Intel) format — LSB is at start_bit, bits extend upward.
	LittleEndian ByteOrder = iota
	// BigEndian (Motorola) format — MSB is at start_bit, bits extend downward.
	BigEndian
)

// Signal is a named value encoded within a CAN message.
type Signal struct {
	Name      string
	StartBit  int
	Length    int
	ByteOrder ByteOrder
	Signed    bool
	Factor    float64
	Offset    float64
	Min       float64
	Max       float64
	Unit      string
	Receivers []string
}

// Message is a CAN message definition from a DBC file.
type Message struct {
	ID      uint32
	Name    string
	DLC     int
	Sender  string
	Signals map[string]*Signal
}

// DB is a parsed DBC database.
//
//fusa:req REQ-DBC-001
type DB struct {
	Messages map[uint32]*Message
}

// Parse reads a DBC file from r and returns the parsed database.
//
//fusa:req REQ-DBC-002
func Parse(r io.Reader) (*DB, error) {
	db := &DB{Messages: make(map[uint32]*Message)}
	scanner := bufio.NewScanner(r)

	var curMsg *Message
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "BO_ "):
			msg, err := parseMessage(line)
			if err != nil {
				return nil, err
			}
			curMsg = msg
			db.Messages[msg.ID] = msg

		case strings.HasPrefix(line, "SG_ ") && curMsg != nil:
			sig, err := parseSignal(line)
			if err != nil {
				return nil, err
			}
			curMsg.Signals[sig.Name] = sig

		case !strings.HasPrefix(line, "SG_ "):
			curMsg = nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return db, nil
}

// Decode extracts all signal values from data for the given message ID.
// Returns nil if the message ID is not in the database.
//
//fusa:req REQ-DBC-003
func (db *DB) Decode(id uint32, data []byte) map[string]float64 {
	msg, ok := db.Messages[id]
	if !ok {
		return nil
	}
	result := make(map[string]float64, len(msg.Signals))
	for name, sig := range msg.Signals {
		result[name] = sig.Decode(data)
	}
	return result
}

// Decode extracts this signal's physical value from a CAN frame payload.
func (s *Signal) Decode(data []byte) float64 {
	raw := extractRaw(data, s.StartBit, s.Length, s.ByteOrder)
	var phys float64
	if s.Signed {
		signed := int64(raw)
		if raw&(1<<(s.Length-1)) != 0 {
			signed = int64(raw) - int64(1)<<s.Length
		}
		phys = float64(signed)*s.Factor + s.Offset
	} else {
		phys = float64(raw)*s.Factor + s.Offset
	}
	return phys
}

func extractRaw(data []byte, startBit, length int, order ByteOrder) uint64 {
	var raw uint64
	if order == LittleEndian {
		for i := 0; i < length; i++ {
			bit := startBit + i
			byteIdx := bit / 8
			bitIdx := bit % 8
			if byteIdx >= len(data) {
				break
			}
			if data[byteIdx]&(1<<bitIdx) != 0 {
				raw |= 1 << i
			}
		}
	} else {
		// Big endian (Motorola): startBit is MSB position
		byteIdx := startBit / 8
		bitIdx := startBit % 8
		for i := 0; i < length; i++ {
			if byteIdx < len(data) && data[byteIdx]&(1<<bitIdx) != 0 {
				raw |= 1 << (length - 1 - i)
			}
			if bitIdx == 0 {
				bitIdx = 7
				byteIdx++
			} else {
				bitIdx--
			}
		}
	}
	return raw
}

// parseMessage parses a line like: BO_ 256 EngineData: 8 ECU
func parseMessage(line string) (*Message, error) {
	// BO_ <id> <name>: <dlc> <sender>
	parts := strings.Fields(line)
	if len(parts) < 5 {
		return nil, fmt.Errorf("dbc: malformed BO_ line: %q", line)
	}
	id, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("dbc: invalid message ID %q: %w", parts[1], err)
	}
	if !strings.HasSuffix(parts[2], ":") {
		return nil, fmt.Errorf("dbc: malformed BO_ line (missing colon after name): %q", line)
	}
	name := strings.TrimSuffix(parts[2], ":")
	dlc, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, fmt.Errorf("dbc: invalid DLC %q: %w", parts[3], err)
	}
	return &Message{
		ID:      uint32(id),
		Name:    name,
		DLC:     dlc,
		Sender:  parts[4],
		Signals: make(map[string]*Signal),
	}, nil
}

// parseSignal parses a line like:
// SG_ EngineSpeed : 0|16@1+ (0.25,0) [0|16383.75] "rpm" Vector__XXX
func parseSignal(line string) (*Signal, error) {
	line = strings.TrimSpace(line)
	// Remove "SG_ " prefix
	line = strings.TrimPrefix(line, "SG_ ")
	// Name ends at " : "
	colonIdx := strings.Index(line, " : ")
	if colonIdx < 0 {
		return nil, fmt.Errorf("dbc: malformed SG_ line: %q", line)
	}
	name := strings.TrimSpace(line[:colonIdx])
	rest := strings.TrimSpace(line[colonIdx+3:])

	// rest: <start>|<len>@<order><sign> (<factor>,<offset>) [<min>|<max>] "<unit>" <receivers>
	fields := strings.Fields(rest)
	if len(fields) < 5 {
		return nil, fmt.Errorf("dbc: malformed SG_ body: %q", rest)
	}

	// parse <start>|<len>@<order><sign>
	bitDef := fields[0]
	pipeIdx := strings.Index(bitDef, "|")
	atIdx := strings.Index(bitDef, "@")
	if pipeIdx < 0 || atIdx < 0 {
		return nil, fmt.Errorf("dbc: malformed bit definition: %q", bitDef)
	}
	startBit, err := strconv.Atoi(bitDef[:pipeIdx])
	if err != nil {
		return nil, fmt.Errorf("dbc: invalid start bit: %w", err)
	}
	length, err := strconv.Atoi(bitDef[pipeIdx+1 : atIdx])
	if err != nil {
		return nil, fmt.Errorf("dbc: invalid length: %w", err)
	}
	orderSign := bitDef[atIdx+1:]
	if len(orderSign) < 2 {
		return nil, fmt.Errorf("dbc: malformed order+sign: %q", orderSign)
	}
	order := LittleEndian
	if orderSign[0] == '0' {
		order = BigEndian
	}
	signed := orderSign[1] == '-'

	// parse (<factor>,<offset>)
	factorOffsetStr := fields[1]
	factorOffsetStr = strings.Trim(factorOffsetStr, "()")
	foparts := strings.Split(factorOffsetStr, ",")
	if len(foparts) != 2 {
		return nil, fmt.Errorf("dbc: malformed factor/offset: %q", factorOffsetStr)
	}
	factor, err := strconv.ParseFloat(foparts[0], 64)
	if err != nil {
		return nil, fmt.Errorf("dbc: invalid factor: %w", err)
	}
	offset, err := strconv.ParseFloat(foparts[1], 64)
	if err != nil {
		return nil, fmt.Errorf("dbc: invalid offset: %w", err)
	}

	// parse [<min>|<max>]
	minMaxStr := fields[2]
	minMaxStr = strings.Trim(minMaxStr, "[]")
	mmparts := strings.Split(minMaxStr, "|")
	var minVal, maxVal float64
	if len(mmparts) == 2 {
		minVal, _ = strconv.ParseFloat(mmparts[0], 64)
		maxVal, _ = strconv.ParseFloat(mmparts[1], 64)
	} else {
		minVal = math.NaN()
		maxVal = math.NaN()
	}

	// parse "<unit>"
	unit := strings.Trim(fields[3], "\"")

	// parse receivers (rest of fields)
	var receivers []string
	if len(fields) > 4 {
		receivers = strings.Split(fields[4], ",")
	}

	return &Signal{
		Name:      name,
		StartBit:  startBit,
		Length:    length,
		ByteOrder: order,
		Signed:    signed,
		Factor:    factor,
		Offset:    offset,
		Min:       minVal,
		Max:       maxVal,
		Unit:      unit,
		Receivers: receivers,
	}, nil
}
