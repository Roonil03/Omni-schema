package network

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"
)

// Opcode defines WebSocket frame types.
type Opcode byte

const (
	OpContinuation Opcode = 0x0
	OpText         Opcode = 0x1
	OpBinary       Opcode = 0x2
	OpClose        Opcode = 0x8
	OpPing         Opcode = 0x9
	OpPong         Opcode = 0xA
)

// RFC 6455 close codes.
const (
	CloseNormalClosure           uint16 = 1000
	CloseGoingAway               uint16 = 1001
	CloseProtocolError           uint16 = 1002
	CloseUnsupportedData         uint16 = 1003
	CloseNoStatusRcvd            uint16 = 1005
	CloseAbnormalClosure         uint16 = 1006
	CloseInvalidFramePayloadData uint16 = 1007
	ClosePolicyViolation         uint16 = 1008
	CloseMessageTooBig           uint16 = 1009
	CloseMandatoryExtension      uint16 = 1010
	CloseInternalError           uint16 = 1011
	CloseServiceRestart          uint16 = 1012
	CloseTryAgainLater           uint16 = 1013
	CloseBadGateway              uint16 = 1014
	CloseTLSHandshake            uint16 = 1015
)

const MaxMessageSize = 5 << 20 // 5 MB limit per message

const (
	DefaultReadDeadline  = 60 * time.Second
	DefaultWriteDeadline = 30 * time.Second
)

// ProtocolError is returned when a peer violates RFC 6455 framing rules.
// Code is the WebSocket close code that was (or should be) sent, typically 1002.
type ProtocolError struct {
	Code   uint16
	Reason string
}

func (e *ProtocolError) Error() string {
	if e == nil {
		return "websocket protocol error"
	}
	if e.Reason != "" {
		return fmt.Sprintf("websocket protocol error (%d): %s", e.Code, e.Reason)
	}
	return fmt.Sprintf("websocket protocol error (%d)", e.Code)
}

// UpgradeOptions optionally overrides idle deadlines applied after a successful
// HTTP 101 upgrade. Zero durations select the package defaults (60s read, 30s write).
type UpgradeOptions struct {
	ReadDeadline  time.Duration
	WriteDeadline time.Duration
	PingInterval  time.Duration
}

type Conn struct {
	netConn net.Conn
	rw      *bufio.ReadWriter
	wmu     sync.Mutex

	// client is true when this endpoint is the WebSocket client.
	// Servers (UpgradeToWebSocket) leave this false and therefore require MASK=1
	// on every incoming frame. Client endpoints require MASK=0 (server frames).
	client bool

	ReadDeadline  time.Duration
	WriteDeadline time.Duration
	PingInterval  time.Duration

	closed    bool
	closeSent bool
}

// UpgradeToWebSocket manually negotiates the RFC 6455 WebSocket handshake.
// It relies entirely on standard library network hijacking to avoid third-party imports.
// The returned Conn is a server-side endpoint: ReadMessage requires masked client frames.
func UpgradeToWebSocket(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	return UpgradeToWebSocketWithOptions(w, r, UpgradeOptions{})
}

// UpgradeToWebSocketWithOptions is UpgradeToWebSocket with optional deadline overrides.
func UpgradeToWebSocketWithOptions(w http.ResponseWriter, r *http.Request, opts UpgradeOptions) (*Conn, error) {
	if r.Header.Get("Upgrade") != "websocket" {
		return nil, errors.New("not a websocket request")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("missing Sec-WebSocket-Key")
	}

	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("webserver doesn't support hijacking")
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: websocket\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n")
	bufrw.WriteString("Sec-WebSocket-Accept: ")
	bufrw.WriteString(acceptKey)
	bufrw.WriteString("\r\n\r\n")
	if err := bufrw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}

	c := &Conn{
		netConn:       conn,
		rw:            bufrw,
		client:        false, // server-side: require client masking
		ReadDeadline:  DefaultReadDeadline,
		WriteDeadline: DefaultWriteDeadline,
		PingInterval:  opts.PingInterval,
	}
	if opts.ReadDeadline > 0 {
		c.ReadDeadline = opts.ReadDeadline
	}
	if opts.WriteDeadline > 0 {
		c.WriteDeadline = opts.WriteDeadline
	}
	return c, nil
}

// SetDeadlines updates idle read/write timeouts and the suggested ping interval.
// A duration of 0 disables the corresponding net.Conn deadline (or clears PingInterval).
func (c *Conn) SetDeadlines(read, write, ping time.Duration) {
	c.ReadDeadline = read
	c.WriteDeadline = write
	c.PingInterval = ping
}

func (c *Conn) applyReadDeadline() {
	if c.netConn == nil {
		return
	}
	if c.ReadDeadline > 0 {
		c.netConn.SetReadDeadline(time.Now().Add(c.ReadDeadline))
	} else {
		c.netConn.SetReadDeadline(time.Time{})
	}
}

func (c *Conn) applyWriteDeadline() {
	if c.netConn == nil {
		return
	}
	if c.WriteDeadline > 0 {
		c.netConn.SetWriteDeadline(time.Now().Add(c.WriteDeadline))
	} else {
		c.netConn.SetWriteDeadline(time.Time{})
	}
}

func isKnownOpcode(op Opcode) bool {
	switch op {
	case OpContinuation, OpText, OpBinary, OpClose, OpPing, OpPong:
		return true
	default:
		return false
	}
}

func isControlOpcode(op Opcode) bool {
	return op == OpClose || op == OpPing || op == OpPong
}

// ValidCloseCode reports whether code may appear in a Close frame body.
// Allowed: 1000–1003, 1007–1014, 3000–4999.
func ValidCloseCode(code uint16) bool {
	switch {
	case code >= 1000 && code <= 1003:
		return true
	case code >= 1007 && code <= 1014:
		return true
	case code >= 3000 && code <= 4999:
		return true
	default:
		return false
	}
}

// ValidateClosePayload checks an OpClose payload per RFC 6455 §5.5.1 / §7.4.
// A 0-byte payload is valid. A 1-byte payload is invalid. Two or more bytes
// begin with a 2-byte big-endian close code; any remainder must be valid UTF-8.
func ValidateClosePayload(payload []byte) error {
	switch len(payload) {
	case 0:
		return nil
	case 1:
		return &ProtocolError{Code: CloseProtocolError, Reason: "close payload must not be 1 byte"}
	default:
		code := binary.BigEndian.Uint16(payload[:2])
		if !ValidCloseCode(code) {
			return &ProtocolError{Code: CloseProtocolError, Reason: fmt.Sprintf("invalid close code %d", code)}
		}
		if len(payload) > 2 && !utf8.Valid(payload[2:]) {
			return &ProtocolError{Code: CloseProtocolError, Reason: "close reason is not valid UTF-8"}
		}
		return nil
	}
}

func (c *Conn) failProtocol(reason string) error {
	_ = c.CloseWithCode(int(CloseProtocolError), reason)
	return &ProtocolError{Code: CloseProtocolError, Reason: reason}
}

func (c *Conn) failClose(code uint16, reason string, wrap error) error {
	_ = c.CloseWithCode(int(code), reason)
	if wrap != nil {
		return wrap
	}
	return &ProtocolError{Code: code, Reason: reason}
}

// ReadMessage reads a complete WebSocket message, correctly assembling fragmented
// messages per RFC 6455 §5.4. Control frames (ping, pong, close) that arrive between
// data fragments are handled inline without disrupting the assembly buffer.
//
// On OpClose, ReadMessage returns (OpClose, payload, nil) and does not TCP-close.
// Callers MUST invoke HandleCloseFrame (or CloseHandshake) with that payload to
// complete the close handshake, then close the TCP connection.
func (c *Conn) ReadMessage() (Opcode, []byte, error) {
	var (
		messageOpcode Opcode // The opcode from the first frame of the current message.
		messageBuf    []byte // Accumulation buffer for fragmented payloads.
		assembling    bool   // True when we are mid-fragmentation.
	)

	for {
		c.applyReadDeadline()
		header := make([]byte, 2)
		_, err := io.ReadFull(c.rw, header)
		if err != nil {
			return 0, nil, err
		}

		fin := (header[0] & 0x80) != 0
		rsv := header[0] & 0x70
		opcode := Opcode(header[0] & 0x0F)
		masked := (header[1] & 0x80) != 0
		payloadLen7 := header[1] & 0x7F
		payloadLen := uint64(payloadLen7)

		// No extensions are negotiated; RSV bits must be 0 (RFC 6455 §5.2).
		if rsv != 0 {
			return 0, nil, c.failProtocol("RSV bits must be 0 unless an extension is negotiated")
		}
		if !isKnownOpcode(opcode) {
			return 0, nil, c.failProtocol(fmt.Sprintf("unknown opcode 0x%X", opcode))
		}

		// RFC 6455 §5.1: client-to-server frames MUST be masked; server-to-client MUST NOT.
		if !c.client && !masked {
			return 0, nil, c.failProtocol("client frames must be masked")
		}
		if c.client && masked {
			return 0, nil, c.failProtocol("server frames must not be masked")
		}

		if isControlOpcode(opcode) {
			// RFC 6455 §5.5: control frames MUST NOT be fragmented and payload ≤ 125.
			if !fin {
				return 0, nil, c.failProtocol("control frames must not be fragmented")
			}
			if payloadLen7 == 126 || payloadLen7 == 127 {
				return 0, nil, c.failProtocol("control frame payload must be <= 125 bytes")
			}
		}

		if payloadLen7 == 126 {
			c.applyReadDeadline()
			ext := make([]byte, 2)
			if _, err := io.ReadFull(c.rw, ext); err != nil {
				return 0, nil, err
			}
			payloadLen = uint64(binary.BigEndian.Uint16(ext))
		} else if payloadLen7 == 127 {
			c.applyReadDeadline()
			ext := make([]byte, 8)
			if _, err := io.ReadFull(c.rw, ext); err != nil {
				return 0, nil, err
			}
			payloadLen = binary.BigEndian.Uint64(ext)
			// Highest bit of 64-bit length must be 0 (RFC 6455 §5.2).
			if ext[0]&0x80 != 0 {
				return 0, nil, c.failProtocol("invalid 64-bit payload length")
			}
		}

		var maskKey []byte
		if masked {
			c.applyReadDeadline()
			maskKey = make([]byte, 4)
			if _, err := io.ReadFull(c.rw, maskKey); err != nil {
				return 0, nil, err
			}
		}

		if isControlOpcode(opcode) && payloadLen > 125 {
			return 0, nil, c.failProtocol("control frame payload must be <= 125 bytes")
		}

		if payloadLen > MaxMessageSize {
			return 0, nil, c.failClose(CloseMessageTooBig, "message too big", errors.New("websocket message exceeds max size"))
		}
		if assembling && uint64(len(messageBuf))+payloadLen > MaxMessageSize {
			return 0, nil, c.failClose(CloseMessageTooBig, "message too big", errors.New("websocket assembled message exceeds max size"))
		}

		c.applyReadDeadline()
		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(c.rw, payload); err != nil {
			return 0, nil, err
		}

		if masked {
			for i := uint64(0); i < payloadLen; i++ {
				payload[i] ^= maskKey[i%4]
			}
		}

		// RFC 6455 §5.5: Control frames may be injected between data fragments
		// and must be handled immediately.
		if isControlOpcode(opcode) {
			switch opcode {
			case OpPing:
				if err := c.WriteMessage(OpPong, payload); err != nil {
					return 0, nil, err
				}
			case OpPong:
				// No action required.
			case OpClose:
				// Do not TCP-close here. Caller should HandleCloseFrame(payload).
				return OpClose, payload, nil
			}
			continue
		}

		// Data frame handling with fragmentation assembly.
		if !assembling {
			if opcode == OpContinuation {
				return 0, nil, c.failProtocol("unexpected continuation frame")
			}
			if fin {
				return opcode, payload, nil
			}
			messageOpcode = opcode
			messageBuf = append(messageBuf[:0], payload...)
			assembling = true
		} else {
			// Per RFC 6455 §5.4, continuation frames must have opcode 0x0
			// and the final fragment has FIN=1.
			if opcode != OpContinuation {
				return 0, nil, c.failProtocol("expected continuation frame (opcode 0x0) during fragmented message")
			}
			messageBuf = append(messageBuf, payload...)
			if fin {
				assembling = false
				result := make([]byte, len(messageBuf))
				copy(result, messageBuf)
				messageBuf = messageBuf[:0]
				return messageOpcode, result, nil
			}
		}
	}
}

// WriteMessage sends a WebSocket frame with the given opcode and payload.
// Server-side connections (client == false) send unmasked frames.
// Client-side connections mask the payload as required by RFC 6455 §5.1.
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.writeMessageLocked(opcode, payload)
}

// StartPinger writes OpPing frames until stop is closed.
func (c *Conn) StartPinger(stop <-chan struct{}) {
	if c == nil || c.PingInterval <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(c.PingInterval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if err := c.WriteMessage(OpPing, nil); err != nil {
					return
				}
			}
		}
	}()
}

func (c *Conn) writeMessageLocked(opcode Opcode, payload []byte) error {
	if c.closed {
		return net.ErrClosed
	}
	if isControlOpcode(opcode) && len(payload) > 125 {
		return errors.New("control frame payload must be <= 125 bytes")
	}

	c.applyWriteDeadline()

	var header []byte
	header = append(header, 0x80|byte(opcode)) // FIN bit set; RSV=0

	length := len(payload)
	maskBit := byte(0)
	if c.client {
		maskBit = 0x80
	}
	if length < 126 {
		header = append(header, maskBit|byte(length))
	} else if length <= 65535 {
		header = append(header, maskBit|126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(length))
		header = append(header, ext...)
	} else {
		header = append(header, maskBit|127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(length))
		header = append(header, ext...)
	}

	var maskKey [4]byte
	outPayload := payload
	if c.client {
		if _, err := rand.Read(maskKey[:]); err != nil {
			return err
		}
		header = append(header, maskKey[:]...)
		outPayload = make([]byte, length)
		for i := 0; i < length; i++ {
			outPayload[i] = payload[i] ^ maskKey[i%4]
		}
	}

	if _, err := c.rw.Write(header); err != nil {
		return err
	}
	if len(outPayload) > 0 {
		if _, err := c.rw.Write(outPayload); err != nil {
			return err
		}
	}
	return c.rw.Flush()
}

// HandleCloseFrame completes the RFC 6455 close handshake: it validates the
// peer's close payload, sends a Close frame (echoing the code, or 1000 if none),
// then closes the TCP connection.
func (c *Conn) HandleCloseFrame(payload []byte) error {
	if err := ValidateClosePayload(payload); err != nil {
		reason := "invalid close payload"
		if pe, ok := err.(*ProtocolError); ok && pe.Reason != "" {
			reason = pe.Reason
		}
		return c.CloseWithCode(int(CloseProtocolError), reason)
	}
	code := int(CloseNormalClosure)
	reason := ""
	if len(payload) >= 2 {
		code = int(binary.BigEndian.Uint16(payload[:2]))
		reason = string(payload[2:])
	}
	return c.CloseWithCode(code, reason)
}

// CloseHandshake is an alias for HandleCloseFrame.
func (c *Conn) CloseHandshake(payload []byte) error {
	return c.HandleCloseFrame(payload)
}

// CloseWithCode sends a Close frame with the given code and UTF-8 reason, then
// closes the underlying TCP connection.
func (c *Conn) CloseWithCode(code int, reason string) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	if c.closed {
		return net.ErrClosed
	}

	var writeErr error
	if !c.closeSent {
		c.closeSent = true
		payload := make([]byte, 2+len(reason))
		binary.BigEndian.PutUint16(payload[:2], uint16(code))
		copy(payload[2:], reason)
		writeErr = c.writeMessageLocked(Opcode(OpClose), payload)
	}

	c.closed = true
	closeErr := c.netConn.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (c *Conn) Close() error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	c.closed = true
	return c.netConn.Close()
}
