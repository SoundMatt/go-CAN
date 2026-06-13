// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package uds implements Unified Diagnostic Services (ISO 14229) over
// ISO-TP (ISO 15765-2). UDS provides a standardised request/response
// protocol for ECU diagnostics in automotive systems.
//
// Supported service IDs:
//   - 0x10 DiagnosticSessionControl
//   - 0x11 ECUReset
//   - 0x22 ReadDataByIdentifier
//   - 0x27 SecurityAccess
//   - 0x2E WriteDataByIdentifier
//   - 0x3E TesterPresent
//
// Usage:
//
//	conn, _ := isotp.New(bus, isotp.Config{TxID: 0x7E0, RxID: 0x7E8})
//	client := uds.NewClient(conn)
//	data, _ := client.ReadDataByIdentifier(ctx, 0xF190)  // VIN
//
//fusa:req REQ-UDS-001
//fusa:req REQ-UDS-002
//fusa:req REQ-UDS-003
//fusa:req REQ-UDS-004
//fusa:req REQ-UDS-005
//fusa:req REQ-UDS-006
//fusa:req REQ-UDS-007
//fusa:req REQ-UDS-008
//fusa:req REQ-UDS-009
package uds

import (
	"context"
	"errors"
	"fmt"

	"github.com/SoundMatt/go-CAN/isotp"
)

// Service IDs (SID).
const (
	SIDDiagnosticSessionControl = 0x10
	SIDECUReset                 = 0x11
	SIDSecurityAccess           = 0x27
	SIDReadDataByIdentifier     = 0x22
	SIDWriteDataByIdentifier    = 0x2E
	SIDTesterPresent            = 0x3E

	positiveResponseOffset = 0x40 // positive response SID = request SID + 0x40
	negativeResponseSID    = 0x7F
)

// SessionType specifies the diagnostic session.
type SessionType byte

const (
	SessionDefault    SessionType = 0x01
	SessionProgramming SessionType = 0x02
	SessionExtended   SessionType = 0x03
)

// ResetType specifies the ECU reset mode.
type ResetType byte

const (
	ResetHard    ResetType = 0x01
	ResetKeyOff  ResetType = 0x02
	ResetSoft    ResetType = 0x03
)

// NRC (Negative Response Code) values from ISO 14229-1 Table A-1.
const (
	NRCGeneralReject                    byte = 0x10
	NRCServiceNotSupported              byte = 0x11
	NRCSubFunctionNotSupported          byte = 0x12
	NRCIncorrectMessageLengthOrFormat   byte = 0x13
	NRCConditionsNotCorrect             byte = 0x22
	NRCRequestSequenceError             byte = 0x24
	NRCRequestOutOfRange                byte = 0x31
	NRCSecurityAccessDenied             byte = 0x33
	NRCUploadDownloadNotAccepted        byte = 0x70
	NRCResponsePending                  byte = 0x78
)

// NegativeResponseError is returned when the ECU sends a negative response.
type NegativeResponseError struct {
	RequestSID byte
	NRC        byte
}

func (e *NegativeResponseError) Error() string {
	return fmt.Sprintf("uds: NRC 0x%02X (%s) for SID 0x%02X", e.NRC, nrcString(e.NRC), e.RequestSID)
}

// Client is a UDS client that communicates over an ISO-TP connection.
//
//fusa:req REQ-UDS-001
//fusa:req REQ-UDS-002
//fusa:req REQ-UDS-003
//fusa:req REQ-UDS-004
//fusa:req REQ-UDS-005
//fusa:req REQ-UDS-006
//fusa:req REQ-UDS-007
//fusa:req REQ-UDS-008
//fusa:req REQ-UDS-009
type Client struct {
	conn *isotp.Conn
}

// NewClient creates a UDS client.
//
//fusa:req REQ-UDS-001
//fusa:req REQ-UDS-009
func NewClient(conn *isotp.Conn) *Client {
	return &Client{conn: conn}
}

// DiagnosticSessionControl switches the ECU to the specified session.
//
//fusa:req REQ-UDS-002
func (c *Client) DiagnosticSessionControl(ctx context.Context, session SessionType) error {
	resp, err := c.request(ctx, []byte{SIDDiagnosticSessionControl, byte(session)})
	if err != nil {
		return err
	}
	if len(resp) < 2 || resp[0] != SIDDiagnosticSessionControl+positiveResponseOffset {
		return fmt.Errorf("uds: unexpected DiagnosticSessionControl response: %X", resp)
	}
	return nil
}

// ECUReset requests an ECU reset.
//
//fusa:req REQ-UDS-003
func (c *Client) ECUReset(ctx context.Context, resetType ResetType) error {
	resp, err := c.request(ctx, []byte{SIDECUReset, byte(resetType)})
	if err != nil {
		return err
	}
	if len(resp) < 2 || resp[0] != SIDECUReset+positiveResponseOffset {
		return fmt.Errorf("uds: unexpected ECUReset response: %X", resp)
	}
	return nil
}

// ReadDataByIdentifier reads one or more data identifiers from the ECU.
// Returns the raw response data (excluding the positive response SID).
//
//fusa:req REQ-UDS-005
//fusa:req REQ-UDS-006
func (c *Client) ReadDataByIdentifier(ctx context.Context, did uint16) ([]byte, error) {
	req := []byte{
		SIDReadDataByIdentifier,
		byte(did >> 8),
		byte(did),
	}
	resp, err := c.request(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp) < 3 || resp[0] != SIDReadDataByIdentifier+positiveResponseOffset {
		return nil, fmt.Errorf("uds: unexpected ReadDataByIdentifier response: %X", resp)
	}
	// resp[0] = positive SID, resp[1:2] = echoed DID, resp[3:] = data
	if len(resp) < 3 {
		return nil, errors.New("uds: ReadDataByIdentifier response too short")
	}
	return resp[3:], nil
}

// WriteDataByIdentifier writes data to a data identifier.
//
//fusa:req REQ-UDS-007
func (c *Client) WriteDataByIdentifier(ctx context.Context, did uint16, data []byte) error {
	req := make([]byte, 3+len(data))
	req[0] = SIDWriteDataByIdentifier
	req[1] = byte(did >> 8)
	req[2] = byte(did)
	copy(req[3:], data)

	resp, err := c.request(ctx, req)
	if err != nil {
		return err
	}
	if len(resp) < 3 || resp[0] != SIDWriteDataByIdentifier+positiveResponseOffset {
		return fmt.Errorf("uds: unexpected WriteDataByIdentifier response: %X", resp)
	}
	return nil
}

// TesterPresent sends a keep-alive to prevent the ECU from timing out the session.
//
//fusa:req REQ-UDS-004
func (c *Client) TesterPresent(ctx context.Context) error {
	resp, err := c.request(ctx, []byte{SIDTesterPresent, 0x00})
	if err != nil {
		return err
	}
	if len(resp) < 1 || resp[0] != SIDTesterPresent+positiveResponseOffset {
		return fmt.Errorf("uds: unexpected TesterPresent response: %X", resp)
	}
	return nil
}

// request sends a UDS request and returns the raw response payload.
//
//fusa:req REQ-UDS-008
//fusa:req REQ-UDS-009
func (c *Client) request(ctx context.Context, req []byte) ([]byte, error) {
	if err := c.conn.Send(ctx, req); err != nil {
		return nil, fmt.Errorf("uds: send: %w", err)
	}
	for {
		resp, err := c.conn.Recv(ctx)
		if err != nil {
			return nil, fmt.Errorf("uds: recv: %w", err)
		}
		if len(resp) == 0 {
			return nil, errors.New("uds: empty response")
		}
		// Handle response pending (NRC 0x78)
		if resp[0] == negativeResponseSID && len(resp) >= 3 && resp[2] == NRCResponsePending {
			continue
		}
		if resp[0] == negativeResponseSID {
			nrc := byte(0)
			if len(resp) >= 3 {
				nrc = resp[2]
			}
			return nil, &NegativeResponseError{RequestSID: req[0], NRC: nrc}
		}
		return resp, nil
	}
}

func nrcString(nrc byte) string {
	switch nrc {
	case NRCGeneralReject:
		return "generalReject"
	case NRCServiceNotSupported:
		return "serviceNotSupported"
	case NRCSubFunctionNotSupported:
		return "subFunctionNotSupported"
	case NRCIncorrectMessageLengthOrFormat:
		return "incorrectMessageLength"
	case NRCConditionsNotCorrect:
		return "conditionsNotCorrect"
	case NRCRequestSequenceError:
		return "requestSequenceError"
	case NRCRequestOutOfRange:
		return "requestOutOfRange"
	case NRCSecurityAccessDenied:
		return "securityAccessDenied"
	case NRCUploadDownloadNotAccepted:
		return "uploadDownloadNotAccepted"
	case NRCResponsePending:
		return "responsePending"
	default:
		return "unknown"
	}
}
