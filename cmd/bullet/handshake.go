package main

import (
	"net"
	"os"

	"github.com/diwasrimal/bullet/pkg/proto"
)

func mustCompleteHandshake(conn net.Conn) {
	_, err := conn.Write([]byte{byte(proto.OpcodeHandshakeRequest)})
	if err != nil {
		eprintf("Error during handshake: %v\n", err)
		os.Exit(1)
	}
	var handshakeResp [1]byte
	_, err = conn.Read(handshakeResp[:])
	if err != nil {
		eprintf("Error during handshake: %v\n", err)
		os.Exit(1)
	}
	if handshakeResp[0] != byte(proto.OpcodeHandshakeResponse) {
		eprintf("Couldn't complete handshake\n")
		os.Exit(1)
	}
	eprintf("Handshake successful!")
}
