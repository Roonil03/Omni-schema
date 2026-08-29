package network

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
	"unicode/utf8"
)

// mockConn is a minimal net.Conn implementation backed by bytes.Buffers for testing.
type mockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
}

func (m *mockConn) Read(b []byte) (n int, err error) { return m.readBuf.Read(b) }
func (m *mockConn) Write(b []byte) (n int, err error) {
	if m.closed {
		return 0, net.ErrClosed
	}
	return m.writeBuf.Write(b)
}
func (m *mockConn) Close() error {
	m.closed = true
	return nil
}
func (m *mockConn) LocalAddr() net.Addr           { return nil }
func (m *mockConn) RemoteAddr() net.Addr          { return nil }
func (m *mockConn) SetDeadline(t time.Time) error { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}
func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func newServerConn(readBuf, writeBuf *bytes.Buffer) (*Conn, *mockConn) {
	mc := &mockConn{readBuf: readBuf, writeBuf: writeBuf}
	return &Conn{
		rw: bufio.NewReadWriter(
			bufio.NewReader(readBuf),
			bufio.NewWriter(writeBuf),
		),
		netConn:       mc,
		client:        false,
		ReadDeadline:  DefaultReadDeadline,
		WriteDeadline: DefaultWriteDeadline,
	}, mc
}

// writeRawFrame writes a raw WebSocket frame into the buffer for test purposes.
// masked=true for client→server frames (as per RFC 6455).
func writeRawFrame(buf *bytes.Buffer, fin bool, opcode byte, payload []byte, masked bool) {
	writeRawFrameEx(buf, fin, opcode, 0, payload, masked)
}

func writeRawFrameEx(buf *bytes.Buffer, fin bool, opcode byte, rsv byte, payload []byte, masked bool) {
	var b0 byte
	if fin {
		b0 = 0x80
	}
	b0 |= rsv | opcode
	buf.WriteByte(b0)

	length := len(payload)
	var b1 byte
	if masked {
		b1 = 0x80
	}
	if length < 126 {
		b1 |= byte(length)
		buf.WriteByte(b1)
	} else if length <= 65535 {
		b1 |= 126
		buf.WriteByte(b1)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(length))
		buf.Write(ext)
	} else {
		b1 |= 127
		buf.WriteByte(b1)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(length))
		buf.Write(ext)
	}

	if masked {
		maskKey := []byte{0x01, 0x02, 0x03, 0x04}
		buf.Write(maskKey)
		maskedPayload := make([]byte, length)
		for i := 0; i < length; i++ {
			maskedPayload[i] = payload[i] ^ maskKey[i%4]
		}
		buf.Write(maskedPayload)
	} else {
		buf.Write(payload)
	}
}

func requireProtocolError(t *testing.T, err error) *ProtocolError {
	t.Helper()
	var pe *ProtocolError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProtocolError, got %v", err)
	}
	if pe.Code != CloseProtocolError {
		t.Fatalf("expected close code 1002, got %d (%v)", pe.Code, err)
	}
	return pe
}

func TestSingleFrameMessage(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, true, 0x01, []byte("hello"), true)
	conn, _ := newServerConn(readBuf, writeBuf)

	op, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op != OpText {
		t.Errorf("expected OpText, got %v", op)
	}
	if string(payload) != "hello" {
		t.Errorf("expected 'hello', got '%s'", string(payload))
	}
}

func TestContinuationFrameAssembly(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, false, 0x01, []byte("hello"), true)
	writeRawFrame(readBuf, false, 0x00, []byte(" wor"), true)
	writeRawFrame(readBuf, true, 0x00, []byte("ld"), true)
	conn, _ := newServerConn(readBuf, writeBuf)

	op, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op != OpText {
		t.Errorf("expected OpText (from first fragment), got %v", op)
	}
	if string(payload) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(payload))
	}
}

func TestControlFrameDuringFragmentation(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, false, 0x01, []byte("foo"), true)
	writeRawFrame(readBuf, true, 0x09, []byte("ping!"), true)
	writeRawFrame(readBuf, true, 0x00, []byte("bar"), true)
	conn, _ := newServerConn(readBuf, writeBuf)

	op, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op != OpText {
		t.Errorf("expected OpText, got %v", op)
	}
	if string(payload) != "foobar" {
		t.Errorf("expected 'foobar', got '%s'", string(payload))
	}
	if writeBuf.Len() == 0 {
		t.Error("expected pong response to be written")
	}
	if writeBuf.Bytes()[0]&0x0F != byte(OpPong) {
		t.Errorf("expected pong opcode, got 0x%X", writeBuf.Bytes()[0])
	}
	if writeBuf.Bytes()[1]&0x80 != 0 {
		t.Error("server pong must be unmasked")
	}
}

func TestCloseFrameDoesNotTCPClose(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, true, 0x08, []byte{0x03, 0xE8}, true) // close with code 1000
	conn, mc := newServerConn(readBuf, writeBuf)

	op, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected nil error on OpClose, got %v", err)
	}
	if op != OpClose {
		t.Errorf("expected OpClose, got %v", op)
	}
	if mc.closed {
		t.Fatal("ReadMessage must not TCP-close on OpClose")
	}
	if err := conn.WriteMessage(OpText, []byte("still-open")); err != nil {
		t.Fatalf("TCP should still be open: %v", err)
	}

	if err := conn.HandleCloseFrame(payload); err != nil {
		t.Fatalf("HandleCloseFrame: %v", err)
	}
	if !mc.closed {
		t.Fatal("HandleCloseFrame should TCP-close")
	}
}

func TestRejectUnmaskedClientFrame(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, true, 0x01, []byte("hello"), false)
	conn, _ := newServerConn(readBuf, writeBuf)

	_, _, err := conn.ReadMessage()
	requireProtocolError(t, err)
}

func TestRSVBitsRejected(t *testing.T) {
	for _, rsv := range []byte{0x40, 0x20, 0x10, 0x70} {
		readBuf := new(bytes.Buffer)
		writeBuf := new(bytes.Buffer)
		writeRawFrameEx(readBuf, true, 0x01, rsv, []byte("x"), true)
		conn, _ := newServerConn(readBuf, writeBuf)
		_, _, err := conn.ReadMessage()
		requireProtocolError(t, err)
	}
}

func TestUnknownOpcodesRejected(t *testing.T) {
	for _, op := range []byte{0x3, 0x4, 0x5, 0x6, 0x7, 0xB, 0xC, 0xD, 0xE, 0xF} {
		readBuf := new(bytes.Buffer)
		writeBuf := new(bytes.Buffer)
		writeRawFrame(readBuf, true, op, nil, true)
		conn, _ := newServerConn(readBuf, writeBuf)
		_, _, err := conn.ReadMessage()
		requireProtocolError(t, err)
	}
}

func TestControlFrameMustBeFIN(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, false, 0x09, []byte("x"), true)
	conn, _ := newServerConn(readBuf, writeBuf)
	_, _, err := conn.ReadMessage()
	requireProtocolError(t, err)
}

func TestControlFramePayloadTooLong(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, true, 0x09, bytes.Repeat([]byte{'a'}, 126), true)
	conn, _ := newServerConn(readBuf, writeBuf)
	_, _, err := conn.ReadMessage()
	requireProtocolError(t, err)
}

func TestValidateClosePayload(t *testing.T) {
	if err := ValidateClosePayload(nil); err != nil {
		t.Fatalf("empty payload should be OK: %v", err)
	}
	if err := ValidateClosePayload([]byte{}); err != nil {
		t.Fatalf("0-byte payload should be OK: %v", err)
	}
	if err := ValidateClosePayload([]byte{0x03}); err == nil {
		t.Fatal("1-byte payload should be invalid")
	}

	validCodes := []uint16{1000, 1001, 1002, 1003, 1007, 1014, 3000, 4999}
	for _, code := range validCodes {
		p := make([]byte, 2)
		binary.BigEndian.PutUint16(p, code)
		if err := ValidateClosePayload(p); err != nil {
			t.Errorf("code %d should be valid: %v", code, err)
		}
		p = append(p, []byte("ok")...)
		if err := ValidateClosePayload(p); err != nil {
			t.Errorf("code %d with UTF-8 reason should be valid: %v", code, err)
		}
	}

	invalidCodes := []uint16{0, 999, 1004, 1005, 1006, 1015, 1016, 2999, 5000}
	for _, code := range invalidCodes {
		p := make([]byte, 2)
		binary.BigEndian.PutUint16(p, code)
		if err := ValidateClosePayload(p); err == nil {
			t.Errorf("code %d should be invalid", code)
		}
	}

	p := []byte{0x03, 0xE8, 0xff, 0xfe, 0xfd}
	if utf8.Valid(p[2:]) {
		t.Fatal("test fixture should be invalid UTF-8")
	}
	if err := ValidateClosePayload(p); err == nil {
		t.Fatal("non-UTF-8 reason should be invalid")
	}
}

func TestHandleCloseFrameInvalidOneByte(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, true, 0x08, []byte{0x03}, true)
	conn, mc := newServerConn(readBuf, writeBuf)

	op, payload, err := conn.ReadMessage()
	if err != nil || op != OpClose {
		t.Fatalf("op=%v err=%v", op, err)
	}
	if err := conn.HandleCloseFrame(payload); err != nil && !errors.Is(err, net.ErrClosed) {
		// mock Close succeeds; write of 1002 should succeed
		t.Fatalf("HandleCloseFrame: %v", err)
	}
	if !mc.closed {
		t.Fatal("expected TCP close")
	}
	if writeBuf.Len() < 2 {
		t.Fatal("expected close frame to be written")
	}
	if writeBuf.Bytes()[0]&0x0F != byte(OpClose) {
		t.Fatalf("expected close opcode, got 0x%X", writeBuf.Bytes()[0])
	}
	// unmasked close, 2-byte length + code 1002
	plen := int(writeBuf.Bytes()[1] & 0x7F)
	body := writeBuf.Bytes()[2 : 2+plen]
	got := binary.BigEndian.Uint16(body[:2])
	if got != CloseProtocolError {
		t.Fatalf("expected echo/send 1002, got %d", got)
	}
}

func TestHandleCloseFrameEchoesCode(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	payload := []byte{0x03, 0xE8} // 1000
	payload = append(payload, []byte("bye")...)
	writeRawFrame(readBuf, true, 0x08, payload, true)
	conn, _ := newServerConn(readBuf, writeBuf)

	op, got, err := conn.ReadMessage()
	if err != nil || op != OpClose {
		t.Fatalf("op=%v err=%v", op, err)
	}
	if err := conn.HandleCloseFrame(got); err != nil {
		t.Fatal(err)
	}
	plen := int(writeBuf.Bytes()[1] & 0x7F)
	body := writeBuf.Bytes()[2 : 2+plen]
	if binary.BigEndian.Uint16(body[:2]) != 1000 {
		t.Fatalf("expected echoed code 1000, got %d", binary.BigEndian.Uint16(body[:2]))
	}
	if string(body[2:]) != "bye" {
		t.Fatalf("expected echoed reason, got %q", body[2:])
	}
}

func TestCloseWithCodeEmptyPayloadUses1000(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, true, 0x08, nil, true)
	conn, _ := newServerConn(readBuf, writeBuf)
	op, payload, err := conn.ReadMessage()
	if err != nil || op != OpClose {
		t.Fatalf("op=%v err=%v", op, err)
	}
	if err := conn.HandleCloseFrame(payload); err != nil {
		t.Fatal(err)
	}
	plen := int(writeBuf.Bytes()[1] & 0x7F)
	body := writeBuf.Bytes()[2 : 2+plen]
	if binary.BigEndian.Uint16(body[:2]) != CloseNormalClosure {
		t.Fatalf("expected 1000, got %d", binary.BigEndian.Uint16(body[:2]))
	}
}

func TestWriteMessageServerUnmasked(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	conn, _ := newServerConn(readBuf, writeBuf)
	if err := conn.WriteMessage(OpText, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	b := writeBuf.Bytes()
	if b[0]&0x0F != byte(OpText) {
		t.Fatalf("opcode: 0x%X", b[0])
	}
	if b[1]&0x80 != 0 {
		t.Fatal("server frames must be unmasked")
	}
	if string(b[2:]) != "hi" {
		t.Fatalf("payload %q", b[2:])
	}
}

func TestSetDeadlines(t *testing.T) {
	conn, _ := newServerConn(new(bytes.Buffer), new(bytes.Buffer))
	conn.SetDeadlines(10*time.Second, 5*time.Second, 15*time.Second)
	if conn.ReadDeadline != 10*time.Second || conn.WriteDeadline != 5*time.Second || conn.PingInterval != 15*time.Second {
		t.Fatalf("deadlines not stored: read=%v write=%v ping=%v", conn.ReadDeadline, conn.WriteDeadline, conn.PingInterval)
	}
}

func TestUnexpectedContinuation(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, true, 0x00, []byte("x"), true)
	conn, _ := newServerConn(readBuf, writeBuf)
	_, _, err := conn.ReadMessage()
	requireProtocolError(t, err)
}

func TestNonContinuationDuringFragment(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, false, 0x01, []byte("a"), true)
	writeRawFrame(readBuf, true, 0x01, []byte("b"), true)
	conn, _ := newServerConn(readBuf, writeBuf)
	_, _, err := conn.ReadMessage()
	requireProtocolError(t, err)
}

func TestCloseHandshakeAlias(t *testing.T) {
	readBuf := new(bytes.Buffer)
	writeBuf := new(bytes.Buffer)
	writeRawFrame(readBuf, true, 0x08, nil, true)
	conn, mc := newServerConn(readBuf, writeBuf)
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.CloseHandshake(payload); err != nil {
		t.Fatal(err)
	}
	if !mc.closed {
		t.Fatal("CloseHandshake should TCP-close")
	}
}

func pipePair(t *testing.T) (*Conn, net.Conn) {
	t.Helper()
	srv, cli := net.Pipe()
	t.Cleanup(func() {
		srv.Close()
		cli.Close()
	})
	c := &Conn{
		netConn:       srv,
		rw:            bufio.NewReadWriter(bufio.NewReader(srv), bufio.NewWriter(srv)),
		client:        false,
		ReadDeadline:  2 * time.Second,
		WriteDeadline: 2 * time.Second,
	}
	return c, cli
}

func TestPipeMaskedTextRoundTrip(t *testing.T) {
	c, cli := pipePair(t)
	var frame bytes.Buffer
	writeRawFrame(&frame, true, 0x01, []byte("ping-body"), true)

	errCh := make(chan error, 1)
	go func() {
		op, payload, err := c.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		if op != OpText || string(payload) != "ping-body" {
			errCh <- errors.New("unexpected message")
			return
		}
		errCh <- c.WriteMessage(OpText, []byte("pong-body"))
	}()

	if _, err := cli.Write(frame.Bytes()); err != nil {
		t.Fatal(err)
	}
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(cli, hdr); err != nil {
		t.Fatal(err)
	}
	if hdr[0]&0x0F != byte(OpText) {
		t.Fatalf("opcode 0x%X", hdr[0])
	}
	if hdr[1]&0x80 != 0 {
		t.Fatal("server reply must be unmasked")
	}
	body := make([]byte, int(hdr[1]&0x7F))
	if _, err := io.ReadFull(cli, body); err != nil {
		t.Fatal(err)
	}
	if string(body) != "pong-body" {
		t.Fatalf("got %q", body)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func TestPipeRejectUnmasked(t *testing.T) {
	c, cli := pipePair(t)
	var frame bytes.Buffer
	writeRawFrame(&frame, true, 0x01, []byte("no-mask"), false)

	errCh := make(chan error, 1)
	go func() {
		_, _, err := c.ReadMessage()
		errCh <- err
	}()
	go io.Copy(io.Discard, cli)

	if _, err := cli.Write(frame.Bytes()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errCh:
		requireProtocolError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for protocol error")
	}
}

func TestPipeCloseHandshake(t *testing.T) {
	c, cli := pipePair(t)
	var frame bytes.Buffer
	writeRawFrame(&frame, true, 0x08, []byte{0x03, 0xE9}, true) // 1001

	type result struct {
		op      Opcode
		payload []byte
		err     error
	}
	readCh := make(chan result, 1)
	go func() {
		op, payload, err := c.ReadMessage()
		readCh <- result{op, payload, err}
	}()
	if _, err := cli.Write(frame.Bytes()); err != nil {
		t.Fatal(err)
	}
	got := <-readCh
	if got.err != nil || got.op != OpClose {
		t.Fatalf("op=%v err=%v", got.op, got.err)
	}

	respCh := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := cli.Read(buf)
		if err != nil {
			respCh <- nil
			return
		}
		respCh <- buf[:n]
	}()
	if err := c.HandleCloseFrame(got.payload); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) && err.Error() != "io: read/write on closed pipe" {
		// net.Pipe Close can surface as "io: read/write on closed pipe" on the write
		// if the peer already went away; the close frame should still have been sent.
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("HandleCloseFrame: %v", err)
		}
	}
	select {
	case resp := <-respCh:
		if len(resp) < 4 {
			t.Fatalf("short close frame %x", resp)
		}
		if resp[0]&0x0F != byte(OpClose) {
			t.Fatalf("expected close, got 0x%X", resp[0])
		}
		if resp[1]&0x80 != 0 {
			t.Fatal("server close must be unmasked")
		}
		plen := int(resp[1] & 0x7F)
		code := binary.BigEndian.Uint16(resp[2 : 2+2])
		if code != 1001 {
			t.Fatalf("expected echoed 1001, got %d (payload len %d)", code, plen)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout reading close echo")
	}
}

func TestPipeReadDeadline(t *testing.T) {
	c, _ := pipePair(t)
	c.SetDeadlines(80*time.Millisecond, 80*time.Millisecond, 0)
	start := time.Now()
	_, _, err := c.ReadMessage()
	if err == nil {
		t.Fatal("expected deadline error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("read deadline was not applied")
	}
}
