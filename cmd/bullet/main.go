package main

import (
	"os"
)

const debugEnabled = false

var defaultRelayAddr = "0.0.0.0:3030"

const usage = `Usage: %[1]s COMMAND

Commands:
  send         Send a file
  recv         Receive a file

Use %[1]s COMMAND --help for usage of specific command.
`

func printUsage() {
	eprintf(usage, os.Args[0])
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "send":
		opts := mustParseSendCmd(os.Args[2:])
		send(opts)
	case "recv":
		opts := mustParseRecvCmd(os.Args[2:])
		recv(opts)
	default:
		printUsage()
		os.Exit(1)
	}
}
