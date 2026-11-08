package client

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"

	"github.com/prdethxn/BitTorrent-Lyte/bitfield"
	"github.com/prdethxn/BitTorrent-Lyte/peer"
)

func StartUploadServer(address string, filePath string, pieceLength uint32, infoHash [32]byte, peerID [20]byte, availablePieces *bitfield.Bitfield) error {
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
				return
			}

			//handle upload requests concurrently
			go func(conn net.Conn) {

				handshake, err := peer.ReadHandshake(conn)
				if err != nil {
					conn.Close()
					return
				}

				if handshake.InfoHash != infoHash {
					conn.Close()
					return
				}

				err = peer.SendHandshake(conn, infoHash, peerID)
				if err != nil {
					conn.Close()
					return
				}

				err = handleUpload(conn, filePath, pieceLength, availablePieces)
				if err != nil {
					log.Printf("Failed to upload to peer: %v", err)
				}
			}(connection) //passing connection to prevent racing

		}
	}()
	return nil //return once listner is running
}

func StartSeeding(address string, filePath string, pieceLength uint32, trackerURL string, peerID string, port int, infoHash string, transactionID int, availablePieces *bitfield.Bitfield) error {
	// Convert peer ID into a fixed-size array.
	if len(peerID) != 20 {
		return fmt.Errorf("invalid peer ID length")
	}
	var localPeerID [20]byte
	copy(localPeerID[:], peerID)

	// Decode hexadecimal infohash into a fixed-size array.
	hashBytes, err := hex.DecodeString(infoHash)
	if err != nil || len(hashBytes) != 32 {
		return fmt.Errorf("invalid infohash")
	}
	var localInfoHash [32]byte
	copy(localInfoHash[:], hashBytes)

	// Start upload server.
	err = StartUploadServer(address, filePath, pieceLength, localInfoHash, localPeerID, availablePieces)
	if err != nil {
		return err
	}

	// Announce completed event to tracker.
	_, err = Announce(trackerURL, peerID, port, infoHash, transactionID, "completed")
	if err != nil {
		return err
	}

	return nil
}
