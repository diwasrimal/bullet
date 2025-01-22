package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/diwasrimal/bullet/pkg/proto"
	"github.com/diwasrimal/bullet/pkg/utils"
)

// type Client struct {
// 	conn           net.Conn
// 	id             string
// 	streamingEnded chan struct{} // for senders that wait until a peer gets their file
// }

// type fileEntry struct {
// 	senderAddr net.Addr // connection of one who's trying to send
// 	shareCode  string   // string code used for identifying file
// 	filename   string
// 	filesize   int64
// }

// var fileEntries = make(map[string]fileEntry) // TODO: cleanup of this map
// var fileEntriesMu sync.Mutex

// var senders = make(map[string]Client)
// var sendersMu sync.Mutex

type sender struct {
	conn                net.Conn
	waitTillConsumption chan struct{} // to block senders from closing until someone consumes the file
	shareCode           string        // string code used for identifying file
	filename            string
	filesize            int64
}

// Map of senders trying to send a file
// mapping is done with their file share codes
// TODO: occasionally cleanup stale connections
var senders = make(map[string]sender)
var sendersMu sync.Mutex

var port int

func main() {
	flag.IntVar(&port, "p", 3030, "Server port")
	flag.Parse()

	address := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Error initializing listener: %v\n", err)
	}

	log.Printf("Server running on %v...\n", address)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Error accepting conn: %v\n", err)
			continue
		}
		go handleConn(conn)
	}
}

func readFrameWithLog(conn net.Conn) (opcode proto.Opcode, payload []byte, err error) {
	opcode, payload, err = proto.ReadFrame(conn)
	log.Printf("read frame, opcode=%s, payload=%s, err=%v\n", opcode, payload, err)
	return
}

func writeFrameWithLog(conn net.Conn, opcode proto.Opcode, payload []byte) (n int, err error) {
	n, err = proto.WriteFrame(conn, opcode, payload)
	log.Printf("wrote frame, conn=%s opcode=%s, payload=%s\n", conn.RemoteAddr().String(), opcode, payload)
	return
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	defer func() {
		log.Printf("deferred function, conn=%s\n", conn.RemoteAddr().String())
	}()
	addr := conn.RemoteAddr().String()
	log.Printf("new connection, conn=%s\n", addr)

	var opcode proto.Opcode
	var payload []byte
	var err error
	_ = err

	opcode, payload, err = readFrameWithLog(conn)
	if opcode != proto.OpcodeHandshakeRequest {
		return
	}
	writeFrameWithLog(conn, proto.OpcodeHandshakeResponse, nil)

	opcode, payload, err = readFrameWithLog(conn)
	if opcode == proto.OpcodeFileSendRequest {
		req := proto.DecodeJSON[proto.FileSendRequestPayload](payload)

		// If client gave a custom share code, make sure it is not already
		// used. If already used, close the connection.
		// If share code was not given, we generate a unique code ourselves
		shareCode := req.ShareCode
		sendersMu.Lock()
		if shareCode == "" {
			for {
				shareCode = utils.RandCode()
				if _, exists := senders[shareCode]; !exists {
					break
				}
			}
		} else {
			_, exists := senders[shareCode]
			if exists {
				writeFrameWithLog(conn, proto.OpcodeShareCodeNotAvailable, nil)
				return
			}
		}
		// Store the sender details int a global map
		sender := sender{
			conn:                conn,
			waitTillConsumption: make(chan struct{}),
			shareCode:           shareCode,
			filename:            req.Filename,
			filesize:            req.Filesize,
		}
		senders[shareCode] = sender
		sendersMu.Unlock()
		defer func() {
			sendersMu.Lock()
			delete(senders, sender.shareCode)
			sendersMu.Unlock()
		}()

		writeFrameWithLog(
			conn,
			proto.OpcodeFileSendResponse,
			proto.JSONToBytes(
				proto.FileSendResponsePayload{ShareCode: shareCode},
			),
		)

		// Wail till file is consumed by some receiver
		<-sender.waitTillConsumption

	} else if opcode == proto.OpcodeFileRecvRequest {
		req := proto.DecodeJSON[proto.FileRecvRequestPayload](payload)

		// Make sure the share code provided is valid
		sendersMu.Lock()
		sender, exists := senders[req.ShareCode]
		sendersMu.Unlock()
		if !exists {
			writeFrameWithLog(conn, proto.OpcodeShareCodeNotFound, nil)
			return
		}

		// Notify receiver about file's name and size
		fileDetails := proto.FileRecvResponsePayload{
			Filesize: sender.filesize,
			Filename: sender.filename,
		}
		writeFrameWithLog(conn, proto.OpcodeFileRecvResponse, proto.JSONToBytes(fileDetails))

		// Wait until receiver is ready to recieve
		opcode, _, _ := readFrameWithLog(conn)

		if opcode != proto.OpcodeReadyToRecieve {
			log.Printf("Unexpected opcode from receiver while waiting for readiness, have (%d) want (%d)\n", opcode, proto.OpcodeReadyToRecieve)
			return
		}
		// Since receiver is ready now, we notify sender
		// that they can start receiving now
		writeFrameWithLog(sender.conn, proto.OpcodeCanStartSending, nil)

		// Then read from sender's conn and write to reciever's conn
		sent, err := io.CopyN(conn, sender.conn, sender.filesize)
		if err != nil {
			log.Printf("Error streaming data to receiver: %s\n", err)
			return
		}
		// All (or some) data has been sent at this point, so we should unblock the sender
		defer close(sender.waitTillConsumption)
		if sent != sender.filesize {
			log.Printf("Couldn't sent whole file, sent (%d/%d) bytes\n", sent, sender.filesize)
			return
		}
		log.Printf("Sent %d bytes from %s -> %s\n", sent, sender.conn.RemoteAddr().String(), addr)

	}

	return

	for {
		opcode, payload, err := proto.ReadFrame(conn)
		log.Printf("In for loop, opcode=%d, payload=%s, err=%v\n", opcode, payload, err)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading from %s: %v\n", addr, err)
			break
		}
		switch opcode {
		case proto.OpcodeHandshakeRequest:
			proto.WriteFrame(conn, proto.OpcodeHandshakeResponse, nil)

		case proto.OpcodeFileSendRequest:
			log.Printf("%s is trying to send\n", addr)
			req := proto.DecodeJSON[proto.FileSendRequestPayload](payload)

			// If client gave a custom share code, make sure it is not already
			// used. If already used, close the connection.
			// If share code was not given, we generate a unique code ourselves
			shareCode := req.ShareCode
			sendersMu.Lock()
			if shareCode == "" {
				for {
					shareCode = utils.RandCode()
					if _, exists := senders[shareCode]; !exists {
						break
					}
				}
			} else {
				_, exists := senders[shareCode]
				if exists {
					proto.WriteFrame(conn, proto.OpcodeShareCodeNotAvailable, nil)
					return
				}
			}
			// Store the sender details int a global map
			sender := sender{
				conn:                conn,
				waitTillConsumption: make(chan struct{}),
				shareCode:           shareCode,
				filename:            req.Filename,
				filesize:            req.Filesize,
			}
			senders[shareCode] = sender
			sendersMu.Unlock()
			defer func() {
				sendersMu.Lock()
				delete(senders, sender.shareCode)
				sendersMu.Unlock()
			}()

			proto.WriteFrame(
				conn,
				proto.OpcodeFileSendResponse,
				proto.JSONToBytes(
					proto.FileSendResponsePayload{ShareCode: shareCode},
				),
			)

			// Wail till file is consumed by some receiver
			<-sender.waitTillConsumption

		case proto.OpcodeFileRecvRequest:
			log.Printf("%s is trying to recv\n", addr)
			req := proto.DecodeJSON[proto.FileRecvRequestPayload](payload)

			// Make sure the share code provided is valid
			sendersMu.Lock()
			sender, exists := senders[req.ShareCode]
			sendersMu.Unlock()
			if !exists {
				proto.WriteFrame(conn, proto.OpcodeShareCodeNotFound, nil)
				break
			}

			// Notify receiver about file's name and size
			fileDetails := proto.FileRecvResponsePayload{
				Filesize: sender.filesize,
				Filename: sender.filename,
			}
			proto.WriteFrame(conn, proto.OpcodeFileRecvResponse, proto.JSONToBytes(fileDetails))

			// Wait until receiver is ready to recieve
			opcode, _, _ := proto.ReadFrame(conn)
			if opcode != proto.OpcodeReadyToRecieve {
				log.Printf("Unexpected opcode from receiver while waiting for readiness, have (%d) want (%d)\n", opcode, proto.OpcodeReadyToRecieve)
				return
			}
			// Since receiver is ready now, we notify sender
			// that they can start receiving now
			proto.WriteFrame(sender.conn, proto.OpcodeCanStartSending, nil)

			// Then read rom sender's conn and write to reciever's conn
			sent, err := io.CopyN(conn, sender.conn, sender.filesize)
			if err != nil {
				log.Printf("Error streaming data to receiver: %s\n", err)
				return
			}
			// All (or some) data has been sent at this point, so we should unblock the sender
			defer close(sender.waitTillConsumption)
			if sent != sender.filesize {
				log.Printf("Couldn't sent whole file, sent (%d/%d) bytes\n", sent, sender.filesize)
				return
			}
			log.Printf("Sent %d bytes from %s -> %s\n", sent, sender.conn.RemoteAddr().String(), addr)

		// Invalid cases when connections are closed
		// Server only gets requests, not responses.
		case proto.OpcodeHandshakeResponse,
			proto.OpcodeFileSendResponse,
			proto.OpcodeFileRecvResponse:
			log.Printf("Unacceptable opcode (%d) from %s, closing connection....\n", opcode, addr)
			return
		default:
			log.Printf("Unrecognized opcode (%d) from %s, closing connection....\n", opcode, addr)
			return
		}
	}

	return

}

// func handleClientOld(client Client) {
// 	defer client.conn.Close()
// 	log.Println("Handling conn for", client.id)

// 	shakeType, err := handshake.Complete(client.conn)
// 	if err != nil {
// 		log.Printf("Error performing handshake: %v\n", err)
// 		return
// 	}

// 	// Client wants to stream a file, in this case we send them the
// 	// connection id, that sending client can share with others
// 	if shakeType == handshake.SendRequest {
// 		log.Printf("Send handshake from %v\n", client.id)
// 		client.conn.Write([]byte(client.id))
// 		sendersMu.Lock()
// 		senders[client.id] = client
// 		sendersMu.Unlock()

// 		<-client.streamingEnded
// 		log.Printf("streaming completed by %s\n", client.id)

// 	} else if shakeType == handshake.RecvRequest {
// 		log.Printf("Recv handshake from %v\n", client.id)
// 		senderIdBuf := make([]byte, 32)
// 		n, err := client.conn.Read(senderIdBuf)
// 		if err != nil {
// 			log.Printf("%T reading conn id: %v\n", err, err)
// 			return
// 		}
// 		senderId := string(senderIdBuf[:n])
// 		sendersMu.Lock()
// 		sender, senderExists := senders[senderId]
// 		sendersMu.Unlock()
// 		if !senderExists {
// 			log.Printf("Sender with id %v not found, closing reciever's connection\n", senderId)
// 			return
// 		}

// 		written, err := io.Copy(client.conn, sender.conn)
// 		if err != nil {
// 			log.Printf("Error streaming to peer: %v\n", err)
// 			return
// 		}
// 		log.Printf("Streamed %v bytes to peer\n", written)

// 		sendersMu.Lock()
// 		delete(senders, senderId)
// 		close(sender.streamingEnded)
// 		sendersMu.Unlock()
// 	}

// }

// var buf [2048]byte
// 	for {
// 		// TODO: what if data is >= len(buf) and takes more than one reads?
// 		n, err := conn.Read(buf[:])
// 		if err != nil {
// 			if err == io.EOF {
// 				break
// 			}
// 			log.Printf("Error reading from %s: %v\n", addr, err)
// 			break
// 		}
// 		utils.Assert(n > 0, "read n <= 0 bytes")

// 		// TODO: handle conn.Write() in each cases
// 		opcode, data := buf[0], buf[1:n]
// 		switch opcode {
// 		case proto.OpcodeHandshakeRequest:
// 			conn.Write([]byte{proto.OpcodeHandshakeResponse})

// 		case proto.OpcodeFileSendRequest:
// 			var details proto.FileSendRequestPayload
// 			err := json.Unmarshal(data, &details)
// 			if err != nil {
// 				log.Printf("Error unmarshaling file send request: %v\n", err)
// 				return
// 			}
// 			log.Printf("%s is trying to send\n", addr)

// 			// If client gave a custom share code, make sure it is not already
// 			// used. If already used, close the connection.
// 			// If share code was not given, we generate a unique code ourselves
// 			shareCode := details.ShareCode
// 			sendersMu.Lock()
// 			if shareCode == "" {
// 				for {
// 					shareCode = utils.RandCode()
// 					if _, exists := senders[shareCode]; !exists {
// 						break
// 					}
// 				}
// 			} else {
// 				_, exists := senders[shareCode]
// 				if exists {
// 					conn.Write([]byte{proto.OpcodeShareCodeNotAvailable})
// 					return
// 				}
// 			}
// 			// Store the sender details int a global map
// 			sender := sender{
// 				conn:                conn,
// 				waitTillConsumption: make(chan struct{}),
// 				shareCode:           shareCode,
// 				filename:            details.Filename,
// 				filesize:            details.Filesize,
// 			}
// 			senders[shareCode] = sender
// 			sendersMu.Unlock()
// 			defer func() {
// 				sendersMu.Lock()
// 				delete(senders, sender.shareCode)
// 				sendersMu.Unlock()
// 			}()

// 			conn.Write(proto.EncodeJSONFrame(
// 				proto.OpcodeFileSendResponse,
// 				proto.FileSendResponsePayload{ShareCode: shareCode},
// 			))

// 			// Wail till file is consumed by some receiver
// 			<-sender.waitTillConsumption

// 		case proto.OpcodeFileRecvRequest:
// 			var details proto.FileRecvRequestPayload
// 			err := json.Unmarshal(data, &details)
// 			if err != nil {
// 				log.Printf("Error unmarshaling file recv request from %s: %v\n", addr, err)
// 				return
// 			}
// 			log.Printf("%s is trying to recv\n", addr)

// 			// Make sure the share code provided is valid
// 			sendersMu.Lock()
// 			sender, exists := senders[details.ShareCode]
// 			sendersMu.Unlock()
// 			if !exists {
// 				conn.Write([]byte{proto.OpcodeShareCodeNotFound})
// 				break
// 			}

// 			// Notify receiver about file's name and size
// 			fileDetails := proto.FileRecvResponsePayload{
// 				Filesize: sender.filesize,
// 				Filename: sender.filename,
// 			}
// 			n, err = conn.Write(proto.EncodeJSONFrame(proto.OpcodeFileRecvResponse, fileDetails))
// 			log.Printf("proto.FileRecvResponsePayload sent, n=%d, err=%v\n", n, err)
// 			if err != nil {
// 				log.Println(err)
// 			}

// 			// Notify sender that they can start streaming the file now
// 			// Notify reciever that they can start receiving now
// 			// sender.conn.Write([]byte{proto.OpcodeCanStartSending})
// 			// conn.Write([]byte{proto.OpcodeCanStartRecving})

// 			// Then read rom sender's conn and write to reciever's conn
// 			sent, err := io.CopyN(conn, sender.conn, sender.filesize)
// 			if err != nil {
// 				log.Printf("Error streaming data to receiver: %s\n", err)
// 				return
// 			}

// 			// All (or some) data has been sent at this point, so we should unblock the sender
// 			defer close(sender.waitTillConsumption)

// 			if sent != sender.filesize {
// 				log.Printf("Couldn't sent whole file, sent (%d/%d) bytes\n", sent, sender.filesize)
// 				return
// 			}
// 			log.Printf("Sent %d bytes from %s -> %s\n", sent, sender.conn.RemoteAddr().String(), addr)

// 		// Invalid cases when connections are closed
// 		// Server only gets requests, not responses.
// 		case proto.OpcodeHandshakeResponse,
// 			proto.OpcodeFileSendResponse,
// 			proto.OpcodeFileRecvResponse:
// 			log.Printf("Unacceptable opcode (%d) from %s, closing connection....\n", opcode, addr)
// 			return
// 		default:
// 			log.Printf("Unrecognized opcode (%d) from %s, closing connection....\n", opcode, addr)
// 			return
// 		}
// 	}
