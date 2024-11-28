package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/diwasrimal/bullet/pkg/handshake"
	"github.com/diwasrimal/bullet/pkg/utils"
)

type Client struct {
	conn           net.Conn
	id             string
	streamingEnded chan struct{} // for senders that wait until a peer gets their file
}

var senders = make(map[string]Client)
var sendersMu sync.Mutex

var port int

func main() {
	flag.IntVar(&port, "p", 3030, "Server port")
	flag.Parse()

	address := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}

	log.Printf("Server running on %v...\n", address)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Error accepting conn: %v\n", err)
			continue
		}
		client := Client{
			conn:           conn,
			id:             utils.RandCode(),
			streamingEnded: make(chan struct{}),
		}
		go handleClient(client)
	}
}

func handleClient(client Client) {
	defer client.conn.Close()
	log.Println("Handling conn for", client.id)

	shakeType, err := handshake.Complete(client.conn)
	if err != nil {
		log.Printf("Error performing handshake: %v\n", err)
		return
	}

	// Client wants to stream a file, in this case we send them the
	// connection id, that sending client can share with others
	if shakeType == handshake.SendRequest {
		log.Printf("Send handshake from %v\n", client.id)
		client.conn.Write([]byte(client.id))
		sendersMu.Lock()
		senders[client.id] = client
		sendersMu.Unlock()

		<-client.streamingEnded
		log.Printf("streaming completed by %s\n", client.id)

	} else if shakeType == handshake.RecvRequest {
		log.Printf("Recv handshake from %v\n", client.id)
		senderIdBuf := make([]byte, 32)
		n, err := client.conn.Read(senderIdBuf)
		if err != nil {
			log.Printf("%T reading conn id: %v\n", err, err)
			return
		}
		senderId := string(senderIdBuf[:n])
		sendersMu.Lock()
		sender, senderExists := senders[senderId]
		sendersMu.Unlock()
		if !senderExists {
			log.Printf("Sender with id %v not found, closing reciever's connection\n", senderId)
			return
		}

		written, err := io.Copy(client.conn, sender.conn)
		if err != nil {
			log.Printf("Error streaming to peer: %v\n", err)
			return
		}
		log.Printf("Streamed %v bytes to peer\n", written)

		sendersMu.Lock()
		delete(senders, senderId)
		close(sender.streamingEnded)
		sendersMu.Unlock()
	}

}
