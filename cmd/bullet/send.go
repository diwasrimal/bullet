package main

import (
	"flag"
	"io"
	"net"
	"os"

	"github.com/diwasrimal/bullet/pkg/proto"
	"github.com/diwasrimal/bullet/pkg/utils"
)

type sendCmdOpts struct {
	flags struct {
		shareCode string
		relayAddr string
	}
	args struct {
		filepath string
	}
}

func send(opts sendCmdOpts) {
	// Open file
	srcfile, err := os.Open(opts.args.filepath)
	if err != nil {
		eprintf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer srcfile.Close()
	fileInfo, err := srcfile.Stat()
	if err != nil {
		eprintf("Error getting info of file: %s\n", srcfile.Name())
		return // TODO: these returns should be os.Exit(1) ?
	}

	// Create a TCP connection and perform handshake
	conn, err := net.Dial("tcp", opts.flags.relayAddr)
	if err != nil {
		eprintf("Error connecting with server: %v\n", err)
		return
	}
	defer conn.Close()
	_, err = proto.WriteFrame(conn, proto.OpcodeHandshakeRequest, nil)
	if err != nil {
		eprintf("Error during handshake: %v\n", err)
		return
	}
	dbgprintf("Handshake complete!\n")
	opcode, _, err := proto.ReadFrame(conn)
	if err != nil {
		eprintf("Error reading handshake resposne: %v\n", err)
		return
	}
	if opcode != proto.OpcodeHandshakeResponse {
		eprintf("Couldn't complete handshake\n")
		return
	}

	// Perform send file request
	_, err = proto.WriteFrame(
		conn,
		proto.OpcodeFileSendRequest,
		proto.JSONToBytes(proto.FileSendRequestPayload{
			ShareCode: opts.flags.shareCode,
			Filesize:  fileInfo.Size(),
			Filename:  fileInfo.Name(),
		}),
	)
	if err != nil {
		eprintf("Error during send file request: %v\n", err)
		return
	}
	opcode, payload, err := proto.ReadFrame(conn)
	if err != nil {
		eprintf("Error reading send file response: %v\n", err)
		return
	}
	if opcode != proto.OpcodeFileSendResponse {
		if opcode == proto.OpcodeShareCodeNotAvailable {
			eprintf("Share code is unavailable, use another or omit for a random code\n")
		} else {
			eprintf("Unexpected opcode from server, got (%d) want (%d), closing connection....\n", opcode, proto.OpcodeFileSendResponse)
		}
		return
	}
	fileSendResp := proto.DecodeJSON[proto.FileSendResponsePayload](payload)
	eprintf("Share code: %s\n", fileSendResp.ShareCode)

	// Wait for server notification to start sending, then
	// stream the file
	eprintf("Sending %q (%s), waiting for receiver...\n", srcfile.Name(), readableSize(fileInfo.Size()))
	opcode, payload, err = proto.ReadFrame(conn)
	if opcode != proto.OpcodeCanStartSending {
		eprintf("Unexpected opcode from server, got (%d) want (%d), closing connection....\n", opcode, proto.OpcodeCanStartSending)
		return
	}
	sent, err := io.Copy(conn, srcfile)
	if err != nil {
		eprintf("Error sending file: %v\n", err)
		return
	}
	if sent != fileInfo.Size() {
		eprintf("Couldn't send whole file, sent (%d/%d) bytes\n", sent, fileInfo.Size())
		return
	}
	eprintf("Sent %d bytes of data!\n", sent)

	// NOW STREAM ITTTT!!!!
	return

	// for {
	// 	n, err := conn.Read(buf[:]) // TODO: what if 2048 is not enough
	// 	if err != nil {
	// 		eprintf("Error reading from connection: %v\n", err)
	// 		return
	// 	}
	// 	utils.Assert(n > 0, "read n <= 0 bytes")

	// 	opcode, data := buf[0], buf[1:n]
	// 	switch opcode {
	// 	case proto.OpcodeHandshakeResponse:
	// 		eprintf("Handshake complete!\n")

	// 	case proto.OpcodeFileSendResponse:
	// 		var details proto.FileSendResponsePayload
	// 		err := json.Unmarshal(data, &details)
	// 		if err != nil {
	// 			eprintf("Couldn't decode server's message, closing...\n")
	// 			return
	// 		}
	// 		eprintf("File share code: %s\n", details.ShareCode)
	// 		eprintf("On other machine run:\n  ./bullet recv %s\n", details.ShareCode)

	// 	// Invalid cases, clients don't receive requests from server
	// 	case proto.OpcodeFileRecvResponse: // we are sending not recieving
	// 	case proto.OpcodeHandshakeRequest,
	// 		proto.OpcodeFileRecvRequest,
	// 		proto.OpcodeFileSendRequest:
	// 		eprintf("Undesired opcode (%d) returned, closing connection....\n", opcode)
	// 		return
	// 	default:
	// 		eprintf("Unrecognized opcode (%d) returned, closing connection....\n", opcode)
	// 		return
	// 	}
	// }
}

func mustParseSendCmd(args []string) sendCmdOpts {
	var opts sendCmdOpts

	cmd := flag.NewFlagSet("send", flag.ExitOnError)
	cmd.StringVar(&opts.flags.relayAddr, "relay", "", "Relay server address")
	cmd.StringVar(&opts.flags.shareCode, "code", "", "Custom share code for file, randomly generated if not provided")
	cmd.Usage = func() {
		eprintf("Usage: %s send [FLAGS] FILE\n\n", os.Args[0])
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
	if opts.flags.shareCode == "" {
		opts.flags.shareCode = utils.RandCode()
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		os.Exit(1)
	}
	opts.args.filepath = cmd.Arg(0)

	return opts
}
