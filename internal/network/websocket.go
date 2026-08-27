package network

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
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

type Conn struct {
	netConn net.Conn
	rw      *bufio.ReadWriter
	wmu     sync.Mutex
}

// UpgradeToWebSocket manually negotiates the RFC 6455 WebSocket handshake.
// It relies entirely on standard library network hijacking to avoid third-party imports.
func UpgradeToWebSocket(w http.ResponseWriter, r *http.Request) (*Conn, error) {
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
	bufrw.Flush()

	return &Conn{netConn: conn, rw: bufrw}, nil
}

// ReadMessage reads a complete WebSocket message, correctly assembling fragmented
// messages per RFC 6455 §5.4. Control frames (ping, pong, close) that arrive between
// data fragments are handled inline without disrupting the assembly buffer.
func (c *Conn) ReadMessage() (Opcode, []byte, error) {
	var (
		messageOpcode Opcode // The opcode from the first frame of the current message.
		messageBuf    []byte // Accumulation buffer for fragmented payloads.
		assembling    bool   // True when we are mid-fragmentation.
	)

	for {
		header := make([]byte, 2)
		_, err := io.ReadFull(c.rw, header)
		if err != nil {
			return 0, nil, err
		}

		fin := (header[0] & 0x80) != 0
		opcode := Opcode(header[0] & 0x0F)
		masked := (header[1] & 0x80) != 0
		payloadLen := uint64(header[1] & 0x7F)

		if payloadLen == 126 {
			ext := make([]byte, 2)
			if _, err := io.ReadFull(c.rw, ext); err != nil {
				return 0, nil, err
			}
			payloadLen = uint64(binary.BigEndian.Uint16(ext))
		} else if payloadLen == 127 {
			ext := make([]byte, 8)
			if _, err := io.ReadFull(c.rw, ext); err != nil {
				return 0, nil, err
			}
			payloadLen = binary.BigEndian.Uint64(ext)
		}

		var maskKey []byte
		if masked {
			maskKey = make([]byte, 4)
			if _, err := io.ReadFull(c.rw, maskKey); err != nil {
				return 0, nil, err
			}
		}

		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(c.rw, payload); err != nil {
			return 0, nil, err
		}

		if masked {
			for i := uint64(0); i < payloadLen; i++ {
				payload[i] ^= maskKey[i%4]
			}
		}

		// RFC 6455 §5.5: Control frames (opcodes >= 0x8) may be injected between
		// data fragments and must be handled immediately.
		if opcode >= 0x8 {
			switch opcode {
			case OpPing:
				c.WriteMessage(OpPong, payload)
			case OpPong:
				// No action required.
			case OpClose:
				c.Close()
				return OpClose, payload, io.EOF
			}
			continue
		}

		// Data frame handling with fragmentation assembly.
		if !assembling {
			// This is the first frame of a new message.
			if fin {
				// Single-frame, unfragmented message — fast path.
				return opcode, payload, nil
			}
			// First frame of a fragmented message (FIN=0, opcode != 0).
			messageOpcode = opcode
			messageBuf = append(messageBuf[:0], payload...)
			assembling = true
		} else {
			// We are mid-fragmentation. Per RFC 6455 §5.4, continuation frames
			// must have opcode 0x0 and the final fragment has FIN=1.
			if opcode != OpContinuation {
				return 0, nil, errors.New("expected continuation frame (opcode 0x0) during fragmented message")
			}
			messageBuf = append(messageBuf, payload...)
			if fin {
				// Final fragment received — return the assembled message.
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
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var header []byte
	header = append(header, 0x80|byte(opcode)) // FIN bit set

	length := len(payload)
	if length < 126 {
		header = append(header, byte(length))
	} else if length <= 65535 {
		header = append(header, 126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(length))
		header = append(header, ext...)
	} else {
		header = append(header, 127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(length))
		header = append(header, ext...)
	}

	if _, err := c.rw.Write(header); err != nil {
		return err
	}
	if _, err := c.rw.Write(payload); err != nil {
		return err
	}
	return c.rw.Flush()
}

func (c *Conn) Close() error {
	return c.netConn.Close()
}
