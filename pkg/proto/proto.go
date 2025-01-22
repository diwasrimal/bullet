package proto

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/diwasrimal/bullet/pkg/utils"
)

type Opcode byte

const (
	// Handshake codes, used for establishing client-server connection
	OpcodeHandshakeRequest Opcode = iota + 48 // can use netcat :)
	OpcodeHandshakeResponse

	// Data transfer codes, a payload is passed alongside them
	OpcodeFileSendRequest
	OpcodeFileSendResponse
	OpcodeFileRecvRequest
	OpcodeFileRecvResponse
	OpcodeTextMsg

	// Notification codes
	OpcodeShareCodeNotAvailable
	OpcodeShareCodeNotFound
	OpcodeReadyToRecieve  // reciver sends to notify they are ready to accept file
	OpcodeCanStartSending // server notifies sender that they can start sending their file
	// OpcodeCanStartRecving

	OpcodeInvalid
)

func (c Opcode) String() string {
	switch c {
	case OpcodeHandshakeRequest:
		return "OpcodeHandshakeRequest"
	case OpcodeHandshakeResponse:
		return "OpcodeHandshakeResponse"
	case OpcodeFileSendRequest:
		return "OpcodeFileSendRequest"
	case OpcodeFileSendResponse:
		return "OpcodeFileSendResponse"
	case OpcodeFileRecvRequest:
		return "OpcodeFileRecvRequest"
	case OpcodeFileRecvResponse:
		return "OpcodeFileRecvResponse"
	case OpcodeTextMsg:
		return "OpcodeTextMsg"
	case OpcodeShareCodeNotAvailable:
		return "OpcodeShareCodeNotAvailable"
	case OpcodeShareCodeNotFound:
		return "OpcodeShareCodeNotFound"
	case OpcodeReadyToRecieve:
		return "OpcodeReadyToRecieve"
	case OpcodeCanStartSending:
		return "OpcodeCanStartSending"
	default:
		return "OpcodeInvalid"
	}
}

type Payload interface {
	FileSendRequestPayload |
		FileSendResponsePayload |
		FileRecvRequestPayload |
		FileRecvResponsePayload
}

type FileSendRequestPayload struct {
	ShareCode string `json:"share_code"` // Custom file share code requested by sender
	Filesize  int64  `json:"filesize"`
	Filename  string `json:"filename"`
}

type FileSendResponsePayload struct {
	ShareCode string `json:"share_code"`
}

type FileRecvRequestPayload struct {
	ShareCode string `json:"share_code"`
}

type FileRecvResponsePayload struct {
	Filesize int64  `json:"filesize"`
	Filename string `json:"filename"`
}

func JSONToBytes[T Payload](data T) []byte {
	var marshaled []byte
	marshaled, err := json.Marshal(data)
	utils.Assert(err == nil, "json.Marshal shouldn't have errored")
	return marshaled
}

func EncodeJSONFrame[T Payload](opcode Opcode, data T) []byte {
	marshaled, err := json.Marshal(data)
	utils.Assert(err == nil, "json.Marshal shouldn't have errored")

	frame := new(bytes.Buffer)
	frame.WriteByte(byte(opcode))                            // first byte is opcode
	binary.Write(frame, binary.LittleEndian, len(marshaled)) // second byte is payload length
	frame.Write(marshaled)                                   // rest is payload

	return frame.Bytes()
}

func WriteFrame(conn net.Conn, opcode Opcode, payload []byte) (n int, err error) {
	payloadLen := uint16(len(payload))
	frame := new(bytes.Buffer)
	frame.WriteByte(byte(opcode))                              // first byte is opcode
	err = binary.Write(frame, binary.LittleEndian, payloadLen) // second byte is payload length
	frame.Write(payload)                                       // rest is payload
	return conn.Write(frame.Bytes())
}

func ReadFrame(conn net.Conn) (opcode Opcode, payload []byte, err error) {
	var opcodeBuf [1]byte
	_, err = io.ReadFull(conn, opcodeBuf[:])
	if err != nil {
		return OpcodeInvalid, nil, err
	}
	opcode = Opcode(opcodeBuf[0])

	var payloadLen uint16
	err = binary.Read(conn, binary.LittleEndian, &payloadLen)
	if err != nil {
		return opcode, nil, err
	}

	payload = make([]byte, payloadLen)
	n, err := io.ReadFull(conn, payload)
	if err != nil {
		return opcode, payload, err
	}
	if uint16(n) != payloadLen {
		return opcode, payload, errors.New("n != payloadLen")
	}
	return opcode, payload, nil
}

func DecodeJSON[T Payload](bytes []byte) T {
	var data T
	err := json.Unmarshal(bytes, &data)
	if err != nil {
		panic(fmt.Errorf("Error decoding JSON payload: %v\n", err))
	}
	return data
}
