// Message structure: 4 bytes represent message legth, 5th has message ID, latter contain message data
package peer

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
)

type Message struct {
	Length  uint32
	ID      byte
	Payload []byte
}

// 1 MiB
const MaxMessageLength uint32 = 1 << 20

func ReadMessage(connection net.Conn) (Message, error) {
	//Get length
	buffer := make([]byte, 4)

	//Exactly get 4 bytes and convert them to unsigned ints
	_, err := io.ReadFull(connection, buffer)
	if err != nil {
		return Message{}, err
	}

	length := binary.BigEndian.Uint32(buffer)

	//If length is 0, it's a request to keep connection alive.
	if length == 0 {
		return Message{}, nil
	}

	//If length exceeds 1 Mib, terminate. Prevents overly-large message requests from crashing the app.
	if length > MaxMessageLength {
		return Message{}, errors.New("message length exceeds 1 Mib.")

	}

	//Read and extract messageID and data
	payload := make([]byte, length)
	_, err = io.ReadFull(connection, payload)

	if err != nil {
		return Message{}, err
	}

	msgId := payload[0]
	payloadData := payload[1:]

	return Message{
		Length:  length,
		ID:      msgId,
		Payload: payloadData,
	}, nil
}
