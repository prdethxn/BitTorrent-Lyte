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

const MaxMessageLength uint32 = 1 << 2 // 1 MiB
const MsgRequest byte = 6
const BlockSize uint32 = 16 * 1024 // 16 Kib

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

func sendPieceRequest(connection net.Conn, index uint32, begin uint32, length uint32) error {
	buffer := make([]byte, 17)
	//Fill buffer with request data
	binary.BigEndian.PutUint32(buffer[0:4], 13)
	buffer[4] = MsgRequest

	binary.BigEndian.PutUint32(buffer[5:9], index)
	binary.BigEndian.PutUint32(buffer[9:13], begin)
	binary.BigEndian.PutUint32(buffer[13:17], length)

	//Send data to connection
	return writeFull(connection, buffer)
}

func parsePieceMessage(message Message) (uint32, uint32, []byte, error) {
	//When given a message, peers respond with message ID 7
	if message.ID != 7 {
		return 0, 0, nil, errors.New("invalid message ID for piece message")
	}

	//Verify payload has at least 8 bytes
	if len(message.Payload) < 8 {
		return 0, 0, nil, errors.New("invalid payload length for piece message")
	}

	//Extract piece index and begin offset
	index := binary.BigEndian.Uint32(message.Payload[0:4])
	begin := binary.BigEndian.Uint32(message.Payload[4:8])

	//return remaning bytes as payload data
	return index, begin, message.Payload[8:], nil
}

func downloadPiece(connection net.Conn, pieceIndex uint32, pieceLength uint32) ([]byte, error) {
	//allocate byte slice for complete piece
	pieceData := make([]byte, pieceLength)

	//Identify num of blocks needed to download piece
	numBlocks := (pieceLength + BlockSize - 1) / BlockSize

	//Flood requests to the pipeline for each block
	for i := uint32(0); i < numBlocks; i++ {
		currentBlockSize := BlockSize
		//Calculate remaning bytes to download last block as its not always full
		if i == numBlocks-1 {
			currentBlockSize = pieceLength - (i * BlockSize)
		}

		err := sendPieceRequest(connection, pieceIndex, i*BlockSize, currentBlockSize)
		if err != nil {
			return nil, err
		}
	}

	//Collect all responses
	for i := uint32(0); i < numBlocks; i++ {
		//Read response msg and parse it to get piece data
		message, err := ReadMessage(connection)
		if err != nil {
			return nil, err
		}

		index, begin, data, err := parsePieceMessage(message)
		if err != nil {
			return nil, err
		}

		//verify return piece index and offset doesn't exceed buffer size
		if index != pieceIndex || begin+uint32(len(data)) > pieceLength {
			return nil, errors.New("recieved piece index or offset doesn't match expected values")
		}

		//Copy data into correct position
		copy(pieceData[begin:begin+uint32(len(data))], data)
	}
	return pieceData, nil
}
