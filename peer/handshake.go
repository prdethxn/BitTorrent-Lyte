package peer

import (
	"errors"
	"io"
	"net"
)

// Represent data recieved in BitTorrent handshake
type Handshake struct {
	InfoHash [20]byte
	PeerID   [20]byte
}

// Read handshake
func readHandshake(connection net.Conn) (Handshake, error) {
	buffer := make([]byte, 68)
	//Read connection until buffer is full
	_, err := io.ReadFull(connection, buffer)
	if err != nil {
		return Handshake{}, err
	}

	//Exact data from buffer

	//Make sure the handshake is valid first
	if buffer[0] != 19 || string(buffer[1:20]) != "BitTorrent protocol" {
		return Handshake{}, errors.New("invalid protocol header")
	}

	//Exact hash and peer ID from handshake buffer
	var handshake Handshake

	copy(handshake.InfoHash[:], buffer[28:48])
	copy(handshake.PeerID[:], buffer[48:68])

	return handshake, nil
}
