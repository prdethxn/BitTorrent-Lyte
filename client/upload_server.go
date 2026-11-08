package client

import (
	"log"
	"net"

	"github.com/prdethxn/BitTorrent-Lyte/peer"
)

func StartUploadServer(address string, filePath string, pieceLength uint32) error {
	//Open TCP listener
	listener, err := peer.Listen(address)

	if err != nil {
		return err
	}
	defer listener.Close()

	//Continiosly accept incoming connections
	for {
		connection, err := peer.AcceptConnection(listener)

		if err != nil {
			log.Printf("Failed to accept connection %v", err)
			continue
		}

		//handle upload requests concurrently
		go func(conn net.Conn) {
			err := handleUpload(conn, filePath, pieceLength)
			if err != nil {
				log.Printf("Failed to upload to peer: %v", err)
			}
		}(connection) //passing connection to prevent racing

	}
}
