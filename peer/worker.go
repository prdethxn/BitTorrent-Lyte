package peer

import (
	"errors"
	"fmt"
	"net"

	"github.com/prdethxn/BitTorrent-Lyte/integrity"
)

type PieceJob struct {
	PieceIndex   uint32   //Identifies which piece the worker is assigned
	Infohash     [32]byte //idenfifies swarm
	LocalPeerID  [20]byte //Idenfifies client during handshake
	PeerAddress  string   //IP and port of remote peer
	PieceLength  uint32
	ExpectedHash string
}

func makeJobQueue() chan PieceJob {
	return make(chan PieceJob, 10) //Buffer enables workers to get queued without being assigned to workers.
}

func addJob(jobQueue chan PieceJob, job PieceJob) {
	jobQueue <- job
}

func processJob(job PieceJob) error {
	//Connect to peer
	connection, err := net.Dial("tcp", job.PeerAddress)
	if err != nil {
		//handle error by skipping job
		return fmt.Errorf("failed to connect to peer %s: %v", job.PeerAddress, err)
	}
	defer connection.Close()

	err = sendHandshake(connection, job.Infohash, job.LocalPeerID)
	if err != nil {
		return fmt.Errorf("failed to send handshake: %v", err)
	}
	handshake, err := readHandshake(connection)

	if err != nil {
		//skip job on failure
		return fmt.Errorf("failed to read handshake: %v", err)
	}

    //A peer cannot establish a connection to itself
	if job.LocalPeerID == handshake.PeerID
	{
		return errors.New("peer cannot establish connection to itself")
	}

	//Ensure infohash matching; ensures that is downloaded in the correct file
	if handshake.InfoHash != job.Infohash {
		return errors.New("infohash mismatch")
	}

	//Download piece data
	pieceData, err := downloadPiece(connection, job.PieceIndex, job.PieceLength)
	if err != nil {
		return fmt.Errorf("failed to download piece: %v", err)
	}

	//Verify donwloaded piece integrity
	if !integrity.VerifyIntegrity(pieceData, job.ExpectedHash) {
		return errors.New("integrity check failed for downloaded piece")
	}
	return nil
}

// Worker function to process jobs from queue
func pieceWorker(jobQueue <-chan PieceJob) {
	for job := range jobQueue {
		err := processJob(job)
		if err != nil {
			continue //Skip failed job for now.
		}
	}
}

// Starts workers to grab and process data from queue
func startWorkers(workerCount int, jobQueue chan PieceJob) {
	for i := 0; i < workerCount; i++ {
		go pieceWorker(jobQueue)
	}
}
