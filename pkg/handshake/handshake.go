package handshake

import (
	"fmt"
	"net"
)

type HandshakeType = byte

const (
	OK HandshakeType = iota
	RecvRequest
	SendRequest
	Invalid
)

func Perform(shakeType HandshakeType, conn net.Conn) error {
	_, err := conn.Write([]byte{shakeType})
	if err != nil {
		return err
	}

	resp := make([]byte, 1)
	_, err = conn.Read(resp)
	if err != nil {
		return err
	}

	if resp[0] == OK {
		return nil
	} else {
		return fmt.Errorf("handshake not ok")
	}
}

func Complete(src net.Conn) (shakeType HandshakeType, err error) {
	buf := make([]byte, 1)
	_, err = src.Read(buf)
	if err != nil {
		return Invalid, err
	}

	shakeType = buf[0]
	if shakeType != SendRequest && shakeType != RecvRequest {
		return Invalid, fmt.Errorf("unrecognized handshake request type, code=%d", shakeType)
	}

	// Handshake was valid
	_, err = src.Write([]byte{OK})
	if err != nil {
		return Invalid, err
	}

	return
}

// func performSendHandshake(conn net.Conn) error {

// }

// func performRecvHandshake(conn net.Conn) error {
// 	_, err := conn.Write([]byte{HandshakeRecv})

// 	if err != nil {
// 		return err
// 	}

// 	resp := make([]byte, 1)
// 	_, err = conn.Read(resp)
// 	if err != nil {
// 		return err
// 	}

// 	if resp[0] == HandshakeOK {
// 		return nil
// 	} else {
// 		return fmt.Errorf("handshake not ok")
// 	}
// }
