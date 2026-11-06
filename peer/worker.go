package peer

import (
	"errors"
	"fmt"
	"net"
)

type PieceJob struct {
	PieceIndex  int      //Identifies which piece the worker is assigned
	Infohash    [32]byte //idenfifies swarm
	LocalPeerID [20]byte //Idenfifies client during handshake
	PeerAddress string   //IP and port of remote peer
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

	//Ensure infohash matching; ensures that is downloadin the correct file
	if handshake.InfoHash != job.Infohash {
		return errors.New("infohash mismatch")
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
