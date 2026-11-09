// Message structure: 4 bytes represent message legth, 5th has message ID, latter contain message data
package peer

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"
)

type Message struct {
	Length  uint32
	ID      byte
	Payload []byte
}

const MaxMessageLength uint32 = 1 << 20 // 1 MiB
const BlockSize uint32 = 16 * 1024      // 16 Kib
const (
	MsgRequest  byte = 6
	MsgHave     byte = 4
	MsgBitfield byte = 5
	MsgPiece    byte = 7
)

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
	return WriteFull(connection, buffer)
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

	//Reject empty paylaods
	if len(message.Payload) == 8 {
		return 0, 0, nil, errors.New("empty payload for piece message")
	}

	//Extract piece index and begin offset
	index := binary.BigEndian.Uint32(message.Payload[0:4])
	begin := binary.BigEndian.Uint32(message.Payload[4:8])

	//return remaning bytes as payload data
	return index, begin, message.Payload[8:], nil
}
func downloadPiece(connection net.Conn, pieceIndex uint32, pieceLength uint32) ([]byte, error) {
	const MaxPipeLineDepth = 16
	requestedBlocks := uint32(0)
	recievedBlocks := uint32(0)

	//allocate byte slice for complete piece

	if pieceLength == 0 {
		return nil, errors.New("piece length is zero")
	}

	defer connection.SetDeadline(time.Time{}) //clear deadline after piece download is complete

	pieceData := make([]byte, pieceLength)

	//Identify num of blocks needed to download piece
	numBlocks := 1 + (pieceLength-1)/BlockSize
	//Track which blocks have been requested and recieved
	recieved := make([]bool, numBlocks)
	requested := make([]bool, numBlocks)

	//Keep requesting until all blocks are recieved
	for recievedBlocks < numBlocks {
		//Flood requests to the pipeline for each block
		for requestedBlocks < numBlocks {

			//If pipeline is full, wait for reposnse before sending more requests
			if requestedBlocks-recievedBlocks >= MaxPipeLineDepth {
				break
			}

			currentBlockSize := uint32(BlockSize)
			//Calculate remaning bytes to download last block as its not always full
			if requestedBlocks == numBlocks-1 {
				currentBlockSize = pieceLength - (requestedBlocks * BlockSize)
			}

			err := connection.SetWriteDeadline(time.Now().Add(30 * time.Second)) // Set a timeout for the request
			if err != nil {
				return nil, err
			}

			err = sendPieceRequest(connection, pieceIndex, requestedBlocks*BlockSize, currentBlockSize)
			if err != nil {
				return nil, err
			}
			requested[requestedBlocks] = true
			requestedBlocks++
		}

		//Collect all responses
		blocksToCollect := requestedBlocks - recievedBlocks
		for i := uint32(0); i < blocksToCollect; i++ {
			//Read response msg and parse it to get piece data
			err := connection.SetReadDeadline(time.Now().Add(30 * time.Second)) // Set a timeout for the response

			if err != nil {
				return nil, err
			}

			message, err := ReadMessage(connection)
			if err != nil {
				return nil, err
			}

			index, begin, data, err := parsePieceMessage(message)
			if err != nil {
				return nil, err
			}

			//verify return piece index and that offset doesn't exceed buffer size
			if index != pieceIndex || begin >= pieceLength || uint32(len(data)) > pieceLength-begin {
				return nil, errors.New("recieved piece index or offset doesn't match expected values")
			}
			//Verify that begian is aligned to requested block bounary, if the block was actually requested, and that the data length is within the expected block size

			if begin%BlockSize != 0 || begin/BlockSize >= numBlocks || uint32(len(data)) > BlockSize {
				return nil, errors.New("recieved piece data is not aligned to block boundary or exceeds expected block size")
			}

			//When response arrives, track that its been recieved and ensure it was requested
			blockIndex := begin / BlockSize
			if !requested[blockIndex] {
				return nil, errors.New("recieved piece data for a block that was not requested")
			}

			//Get expected block size
			expectedSize := uint32(BlockSize)
			if blockIndex == numBlocks-1 {
				expectedSize = pieceLength - begin
			}
			if uint32(len(data)) != expectedSize {
				return nil, errors.New("recieved piece data size doesn't match expected size")
			}

			//Check if blcok has already been recieved, if so, skip it
			if recieved[blockIndex] {
				return nil, errors.New("recieved duplicate piece data for a block that has already been recieved")
			}
			recieved[blockIndex] = true
			recievedBlocks++
			//Copy data into correct position
			copy(pieceData[begin:begin+uint32(len(data))], data)
		}
	}
	return pieceData, nil
}
