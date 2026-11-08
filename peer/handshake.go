package peer

import (
	"errors"
	"io"
	"net"
)

// Represent data recieved in BitTorrent handshake
type Handshake struct {
	InfoHash [32]byte
	PeerID   [20]byte
}

func sendHandshake(connection net.Conn, infoHash [32]byte, peerID [20]byte) error {
	buffer := make([]byte, 80)
	//Fill buffer with hanshake data
	buffer[0] = 19
	copy(buffer[1:20], "BitTorrent protocol")
	copy(buffer[28:60], infoHash[:])
	copy(buffer[60:80], peerID[:])

	//Send handshake data to connection
	err := WriteFull(connection, buffer)
	if err != nil {
		return err
	}
	return nil
}

func readHandshake(connection net.Conn) (Handshake, error) {
	buffer := make([]byte, 80)
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

	copy(handshake.InfoHash[:], buffer[28:60])
	copy(handshake.PeerID[:], buffer[60:80])

	return handshake, nil
}

// Guarentee that all bytes are written
func WriteFull(connection net.Conn, buffer []byte) error {
	totalWritten := 0
	for totalWritten < len(buffer) {
		bytes, err := connection.Write(buffer[totalWritten:])
		if err != nil {
			return err
		}

		if bytes == 0 {
			return errors.New("no bytes written to the connection")
		}
		totalWritten += bytes
	}
	return nil
}
