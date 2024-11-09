package main

import (
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/diwasrimal/bullet/pkg/handshake"
)

type Client struct {
	conn           net.Conn
	id             string
	streamingEnded chan struct{} // for senders that wait until a peer gets their file
}

var senders = make(map[string]Client)
var sendersMu sync.Mutex

const address = ":3030"

func main() {
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
			id:             randId(),
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

	// if true {
	// 	buf := make([]byte, 2048)
	// 	for {
	// 		n, err := client.conn.Read(buf)
	// 		if err != nil {
	// 			if err == io.EOF {
	// 				log.Printf("Received EOF, breaking read loop")
	// 				break
	// 			}
	// 			log.Printf("%T reading from %v: %v\n", err, client.id, err)
	// 			continue
	// 		}
	// 		// data := buf[:n]
	// 		log.Printf("Read %v bytes from %v\n", n, client.id)
	// 	}
	// }
}

// func getHandshakeReqType(conn net.Conn) (byte, error) {
// 	buf := make([]byte, 1)
// 	_, err := conn.Read(buf)
// 	if err != nil {
// 		return 255, err
// 	}
// 	handshakeType := buf[0]
// 	if handshakeType != HandshakeSend && handshakeType != HandshakeRecv {
// 		return 255, fmt.Errorf("unrecognized handshake request type, code=%d", handshakeType)
// 	}

// 	// Handshake was valid
// 	_, err = conn.Write([]byte{HandshakeOK})
// 	if err != nil {
// 		return 255, err
// 	}
// 	return handshakeType, nil
// }

func randId() string {
	const randchars = "1234567890qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM"
	buf := make([]byte, 8)
	for i := range len(buf) {
		buf[i] = randchars[rand.Intn(len(randchars))]
	}
	return string(buf)
}

func dummyClient() {
	time.Sleep(2 * time.Second)
	log.Println("client: sending some request")
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("client: %T dialing: %v\n", err, err)
		return
	}

	connIdBuf := make([]byte, 20)
	n, err := conn.Read(connIdBuf)
	connId := connIdBuf[:n]
	log.Printf("client: receive file at link %s/files/%s\n", address, connId)

	data := make([]byte, 2500)
	for i := range len(data) {
		data[i] = byte(rand.Intn(255))
	}
	n, err = conn.Write(data)
	if err != nil {
		log.Printf("client: %T writing: %v\n", err, err)
		return
	}
	log.Printf("client: wrote %v bytes...\n", n)
}
