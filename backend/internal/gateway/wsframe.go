package gateway

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
)

const (
	wsGUID        = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	wsOpcodeClose = 0x8
	wsOpcodeText  = 0x1
)

func WebsocketAcceptKey(key string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func marshalEvent(event TerminalEvent) ([]byte, error) {
	return json.Marshal(event)
}

func writeTextFrame(conn net.Conn, payload []byte) error {
	header := []byte{0x80 | wsOpcodeText}
	payloadLen := len(payload)

	switch {
	case payloadLen <= 125:
		header = append(header, byte(payloadLen))
	case payloadLen <= 65535:
		header = append(header, 126, 0, 0)
		binary.BigEndian.PutUint16(header[len(header)-2:], uint16(payloadLen))
	default:
		header = append(header, 127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[len(header)-8:], uint64(payloadLen))
	}

	frame := make([]byte, 0, len(header)+len(payload))
	frame = append(frame, header...)
	frame = append(frame, payload...)

	_, err := conn.Write(frame)
	return err
}

func readFrame(conn net.Conn) (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, nil, err
	}

	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	payloadLen := int(header[1] & 0x7F)

	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return 0, nil, err
		}
		length := binary.BigEndian.Uint64(ext)
		if length > 1<<20 {
			return 0, nil, errors.New("payload too large")
		}
		payloadLen = int(length)
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(conn, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, err
		}
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	return opcode, payload, nil
}
