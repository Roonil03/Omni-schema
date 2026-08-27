package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

func main() {
	u, _ := url.Parse("ws://localhost:8080/graphql/subscriptions?schema=transaction")
	log.Printf("Connecting to %s", u.String())

	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		log.Fatalf("Dial error: %v", err)
	}
	defer conn.Close()

	// Generate a Sec-WebSocket-Key
	keyBytes := make([]byte, 16)
	rand.Read(keyBytes)
	key := base64.StdEncoding.EncodeToString(keyBytes)

	// Send HTTP Upgrade Request
	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", key)
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Write(conn)

	// Read HTTP Response
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		log.Fatalf("Handshake error: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		log.Fatalf("Expected 101 Switching Protocols, got %d", resp.StatusCode)
	}
	log.Println("Handshake complete. WebSocket connected!")

	// Start a goroutine to read frames from the server
	go func() {
		for {
			header := make([]byte, 2)
			_, err := io.ReadFull(br, header)
			if err != nil {
				log.Printf("Read error: %v", err)
				return
			}
			
			opcode := header[0] & 0x0F
			payloadLen := uint64(header[1] & 0x7F)

			if payloadLen == 126 {
				ext := make([]byte, 2)
				io.ReadFull(br, ext)
				payloadLen = uint64(binary.BigEndian.Uint16(ext))
			} else if payloadLen == 127 {
				ext := make([]byte, 8)
				io.ReadFull(br, ext)
				payloadLen = binary.BigEndian.Uint64(ext)
			}

			payload := make([]byte, payloadLen)
			io.ReadFull(br, payload)

			if opcode == 1 { // Text frame
				fmt.Printf("\n[Event Received] %s\n", string(payload))
			}
		}
	}()

	// Send Connection Init
	log.Println("Sending connection_init...")
	writeFrame(conn, 1, []byte(`{"type":"connection_init"}`))
	
	time.Sleep(500 * time.Millisecond)

	// Send Subscribe
	log.Println("Sending subscribe...")
	writeFrame(conn, 1, []byte(`{"id":"sub_1","type":"subscribe","payload":{"query":"subscription { ... }"}}`))

	log.Println("Listening for events... (Press Ctrl+C to exit)")
	select {} // Block forever
}

// writeFrame manually constructs and masks a WebSocket frame (client -> server must be masked)
func writeFrame(conn net.Conn, opcode byte, payload []byte) {
	var header []byte
	header = append(header, 0x80|opcode) // FIN set

	length := len(payload)
	// Mask bit set (0x80)
	if length < 126 {
		header = append(header, 0x80|byte(length))
	} else if length <= 65535 {
		header = append(header, 0x80|126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(length))
		header = append(header, ext...)
	}

	// Generate 4-byte masking key
	maskKey := make([]byte, 4)
	rand.Read(maskKey)
	header = append(header, maskKey...)

	// Mask the payload
	maskedPayload := make([]byte, length)
	for i := 0; i < length; i++ {
		maskedPayload[i] = payload[i] ^ maskKey[i%4]
	}

	conn.Write(header)
	conn.Write(maskedPayload)
}
