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

	//Continiosly accept incoming connections. Use gorountine to run concurrently with client
	go func() {
		//shut down listner once background loop is done
		defer listener.Close()
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
	}()
	return nil //return once listner is running
}

func StartSeeding(address string, filePath string, pieceLength uint32, trackerURL string, peerID string, port int, infoHash string, transactionID int) error {
	//start upload server and annouce completed event to tracker
	err := StartUploadServer(address, filePath, pieceLength)
	if err != nil {
		return err
	}

	trackerResponse, err := Announce(trackerURL, peerID, port, infoHash, transactionID, "completed")
	if err != nil {
		return err
	}
	return nil
}
