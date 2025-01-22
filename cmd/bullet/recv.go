package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/diwasrimal/bullet/pkg/proto"
)

type recvCmdOpts struct {
	flags struct {
		relayAddr   string
		outFilepath string
	}
	args struct {
		shareCode string
	}
}

func recv(opts recvCmdOpts) {
	// Create a TCP connection and perform handshake
	conn, err := net.Dial("tcp", opts.flags.relayAddr)
	if err != nil {
		eprintf("Error connecting with server: %v\n", err)
		return
	}
	defer conn.Close()

	// Perform handshake with server
	_, err = proto.WriteFrame(conn, proto.OpcodeHandshakeRequest, nil)
	if err != nil {
		eprintf("Error during handshake: %v\n", err)
		return
	}
	opcode, _, err := proto.ReadFrame(conn)
	if err != nil {
		eprintf("Error reading handshake resposne: %v\n", err)
		return
	}
	if opcode != proto.OpcodeHandshakeResponse {
		eprintf("Couldn't complete handshake\n")
		return
	}

	// Do file recv request
	_, err = proto.WriteFrame(
		conn,
		proto.OpcodeFileRecvRequest,
		proto.JSONToBytes(proto.FileRecvRequestPayload{
			ShareCode: opts.args.shareCode,
		}),
	)
	if err != nil {
		eprintf("Error during recv file request: %v\n", err)
		return
	}
	opcode, payload, err := proto.ReadFrame(conn)
	if opcode != proto.OpcodeFileRecvResponse {
		if opcode == proto.OpcodeShareCodeNotFound {
			eprintf("Share code %q not found!\n", opts.args.shareCode)
		} else {
			eprintf("Unexpected opcode, have (%d) want (%d), closing connection....\n", opcode, proto.OpcodeFileRecvResponse)
		}
		return
	}
	fileRecvResp := proto.DecodeJSON[proto.FileRecvResponsePayload](payload)
	eprintf("Detected sender's file: %q (%s)\n", fileRecvResp.Filename, readableSize(fileRecvResp.Filesize))

	// Determine output file path
	// If filepath is provided by user though the cli, we'll write data there,
	// else we'll use the receiving file's name
	// If "-" is provided, we write to stdout
	outFilepath := fileRecvResp.Filename
	if opts.flags.outFilepath != "" {
		outFilepath = opts.flags.outFilepath
	}

	var dstfile *os.File
	if outFilepath == "-" {
		dstfile = os.Stdout
	} else {
		// Get confirmation to overwrite
		_, err = os.Stat(outFilepath)
		fileExists := !errors.Is(err, os.ErrNotExist) // TODO: maybe just err == nil is enough
		if fileExists {
			eprintf("%q already exists, overwrite? (Y/n): ", outFilepath)
			var resp string
			fmt.Scanln(&resp)
			if resp == "n" {
				eprintf("Closing connection...\n")
				return
			}
		}
		dstfile, err = os.Create(outFilepath)
		if err != nil {
			eprintf("Error opening %q for writing: %s\n", outFilepath, err)
			return
		}
	}

	// Now notify server that we are ready to receive the file
	// And receive the file into destination
	proto.WriteFrame(conn, proto.OpcodeReadyToRecieve, nil)
	nc, err := io.CopyN(dstfile, conn, fileRecvResp.Filesize)
	if err != nil {
		eprintf("Error receiving file: %s\n", err)
		return
	}
	if int64(nc) != fileRecvResp.Filesize {
		eprintf("Didn't receive whole file, got (%d/%d) bytes\n", nc, fileRecvResp.Filesize)
		return
	}
	eprintf("Received %d bytes of data at %q.\n", nc, dstfile.Name())

	return // -- prev code cut here --
}

func mustParseRecvCmd(args []string) recvCmdOpts {
	var opts recvCmdOpts

	cmd := flag.NewFlagSet("recv", flag.ExitOnError)
	cmd.StringVar(&opts.flags.relayAddr, "relay", "", "Relay server address")
	cmd.StringVar(&opts.flags.outFilepath, "o", "", "Output file name")
	cmd.Usage = func() {
		eprintf("Usage: %s recv [FLAGS] SHARE_CODE\n\n", os.Args[0])
		eprintf("FLAGS:\n")
		cmd.PrintDefaults()
	}
	if err := cmd.Parse(args); err != nil {
		cmd.Usage()
		os.Exit(1)
	}
	if opts.flags.relayAddr == "" {
		opts.flags.relayAddr = defaultRelayAddr
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		os.Exit(1)
	}
	opts.args.shareCode = cmd.Arg(0)

	return opts
}
