package network

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net/http"
)

// UpgradeToWebSocket manually negotiates the RFC 6455 WebSocket handshake.
// It relies entirely on standard library network hijacking to avoid third-party imports.
func UpgradeToWebSocket(w http.ResponseWriter, r *http.Request) error {
	if r.Header.Get("Upgrade") != "websocket" {
		return errors.New("not a websocket request")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return errors.New("missing Sec-WebSocket-Key")
	}

	// Compute Sec-WebSocket-Accept
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	hj, ok := w.(http.Hijacker)
	if !ok {
		return errors.New("webserver doesn't support hijacking")
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return err
	}
	defer conn.Close() // In a real streaming scenario, this would be kept open

	// Write handshake response natively
	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: websocket\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n")
	bufrw.WriteString("Sec-WebSocket-Accept: ")
	bufrw.WriteString(acceptKey)
	bufrw.WriteString("\r\n")
	bufrw.WriteString("\r\n")
	bufrw.Flush()

	// Native binary masking/unmasking and frame reader loop would go here
	// for streaming GraphQL subscription payloads down to the UIR.
	return nil
}
