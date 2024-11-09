package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/diwasrimal/bullet/pkg/handshake"
)

var serverUrl = "localhost:3030"

func printUsage() {
	fmt.Printf("Usage: %s {send|recv} {<filepath>|<peer_id>}\n\n", os.Args[0])
	fmt.Println("    send <filepath>: Streams the file to the server.")
	fmt.Println("    recv <peer_id>:  Receives file sent by the peer, <peer_id>")
	fmt.Println("                     must be shared by peer who is sending the file.")
}

func main() {
	if len(os.Args) != 3 {
		printUsage()
		os.Exit(1)
	}

	subcmd := os.Args[1]
	switch subcmd {
	case "send":
		sendFile(os.Args[2])
	case "recv":
		recvFile(os.Args[2])
	default:
		printUsage()
		os.Exit(1)
	}
}

func sendFile(filePath string) {
	file, err := os.Open(filePath)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	conn, err := net.Dial("tcp", serverUrl)
	if err != nil {
		fmt.Printf("Error connecting with server: %v\n", err)
		return
	}
	defer conn.Close()

	if err := handshake.Perform(handshake.SendRequest, conn); err != nil {
		fmt.Printf("Error performing send handshake: %v\n", err)
		return
	}

	// Print connection id sent by server, this id is shared to peer for receiving sent file
	idBuf := make([]byte, 32)
	n, err := conn.Read(idBuf)
	if err != nil {
		fmt.Printf("%T reading conn id: %v\n", err, err)
		return
	}
	fmt.Printf("Use id %s to share the file\n", idBuf[:n])

	// Stream the file
	sent, err := io.Copy(conn, file)
	if err != nil {
		fmt.Printf("Error sending file: %v\n", err)
		return
	}
	fmt.Printf("Sent %v bytes of data\n", sent)
}

func recvFile(peerId string) {
	conn, err := net.Dial("tcp", serverUrl)
	if err != nil {
		fmt.Printf("Error connecting with server: %v\n", err)
		return
	}

	// Complete recieve file request handshake and then send connection
	// id shared by peer to recieve file from
	if err := handshake.Perform(handshake.RecvRequest, conn); err != nil {
		fmt.Printf("Error performing recv handshake: %v\n", err)
		return
	}
	_, err = conn.Write([]byte(peerId))
	if err != nil {
		fmt.Printf("Error sending connection id to server: %v\n", err)
		return
	}

	n, err := io.Copy(os.Stdout, conn)
	if err != nil {
		fmt.Printf("Error receiving data from server: %v\n", err)
		return
	}

	_ = n

	// fmt.Printf("Received %d bytes of data\n", n)

	defer conn.Close()
}
