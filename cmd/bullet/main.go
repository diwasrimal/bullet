package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/diwasrimal/bullet/pkg/handshake"
	"github.com/diwasrimal/bullet/pkg/utils"
)

var defaultRelayAddr = "0.0.0.0:3030"
var relayAddr string

func printUsage() {
	fmt.Printf("Usage: %[1]s COMMAND\n\n", os.Args[0])
	fmt.Printf("Commands:\n")
	fmt.Printf("  send    Send a file\n")
	fmt.Printf("  recv    Receive a file\n\n")
	fmt.Printf("Use %[1]s COMMAND --help for usage of specific command.\n", os.Args[0])
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "send":
		sendCmd(args)
	case "recv":
		recvCmd(args)
	default:
		printUsage()
		os.Exit(1)
	}
}

func sendCmd(args []string) {
	var shareCode string

	cmd := flag.NewFlagSet("send", flag.ExitOnError)
	cmd.StringVar(&relayAddr, "relay", "", "Relay server address")
	cmd.StringVar(&shareCode, "code", "", "Custom share code for file, randomly generated if not provided")
	cmd.Usage = func() {
		fmt.Printf("Usage: %s send [OPTIONS] FILE\n\n", os.Args[0])
		fmt.Printf("OPTIONS:\n")
		cmd.PrintDefaults()
	}
	if err := cmd.Parse(args); err != nil {
		cmd.Usage()
		os.Exit(1)
	}
	if relayAddr == "" {
		relayAddr = defaultRelayAddr
	}
	if shareCode == "" {
		shareCode = utils.RandCode()
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		os.Exit(1)
	}

	filePath := cmd.Arg(0)
	sendFile(filePath)
}

func recvCmd(args []string) {
	var outFilepath string

	cmd := flag.NewFlagSet("recv", flag.ExitOnError)
	cmd.StringVar(&relayAddr, "relay", "", "Relay server address")
	cmd.StringVar(&outFilepath, "o", "", "Output file name")
	cmd.Usage = func() {
		fmt.Printf("Usage: %s recv [OPTIONS] SHARE_CODE\n\n", os.Args[0])
		fmt.Printf("OPTIONS:\n")
		cmd.PrintDefaults()
	}
	if err := cmd.Parse(args); err != nil {
		cmd.Usage()
		os.Exit(1)
	}
	if relayAddr == "" {
		relayAddr = defaultRelayAddr
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		os.Exit(1)
	}

	shareCode := cmd.Arg(0)
	recvFile(shareCode, outFilepath)
}

func sendFile(srcpath string) {
	file, err := os.Open(srcpath)
	if errors.Is(err, os.ErrNotExist) {
		eprintf("error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	conn, err := net.Dial("tcp", relayAddr)
	if err != nil {
		eprintf("Error connecting with server: %v\n", err)
		return
	}
	defer conn.Close()

	if err := handshake.Perform(handshake.SendRequest, conn); err != nil {
		eprintf("Error performing send handshake: %v\n", err)
		return
	}

	// Print connection id sent by server, this id is shared to peer for receiving sent file
	idBuf := make([]byte, 32)
	n, err := conn.Read(idBuf)
	if err != nil {
		eprintf("%T reading conn id: %v\n", err, err)
		return
	}
	eprintf("Use id %s to share the file\n", idBuf[:n])

	// Stream the file
	sent, err := io.Copy(conn, file)
	if err != nil {
		eprintf("Error sending file: %v\n", err)
		return
	}
	eprintf("Sent %v bytes of data\n", sent)
}

func recvFile(shareCode string, dstpath string) {
	conn, err := net.Dial("tcp", relayAddr)
	if err != nil {
		eprintf("Error connecting with server: %v\n", err)
		return
	}

	// Complete recieve file request handshake and then send connection
	// id shared by peer to recieve file from
	if err := handshake.Perform(handshake.RecvRequest, conn); err != nil {
		eprintf("Error performing recv handshake: %v\n", err)
		return
	}
	_, err = conn.Write([]byte(shareCode))
	if err != nil {
		eprintf("Error sending connection id to server: %v\n", err)
		return
	}

	var dstFile *os.File
	if dstpath == "" {
		dstFile = os.Stdout
	} else {
		dstFile, err = os.Open(dstpath)
		if err != nil {
			eprintf("Error opening file %q for writing: %s\n", dstpath, err)
			return
		}
	}
	n, err := io.Copy(dstFile, conn)
	if err != nil {
		eprintf("Error receiving data from server: %v\n", err)
		return
	}
	eprintf("Received %d bytes of data\n", n)

	defer conn.Close()
}

func eprintf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
}
