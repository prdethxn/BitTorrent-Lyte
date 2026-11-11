package peer

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/prdethxn/BitTorrent-Lyte/integrity"
)

// Used to communicate between workers and a centeral coordinator
type PieceResult struct {
	PieceIndex uint32
	Data       []byte
	Err        error
}

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

func processJob(job PieceJob) (PieceResult, error) {
	//Connect to peer
	connection, err := net.DialTimeout("tcp", job.PeerAddress, 5*time.Second)
	if err != nil {
		//handle error by skipping job
		return PieceResult{}, fmt.Errorf("failed to connect to peer %s: %v", job.PeerAddress, err)
	}
	defer connection.Close()

	err = SendHandshake(connection, job.Infohash, job.LocalPeerID)
	if err != nil {
		return PieceResult{}, fmt.Errorf("failed to send handshake: %v", err)
	}
	handshake, err := ReadHandshake(connection)

	if err != nil {
		//skip job on failure
		return PieceResult{}, fmt.Errorf("failed to read handshake: %v", err)
	}

	//A peer cannot establish a connection to itself
	if job.LocalPeerID == handshake.PeerID {
		return PieceResult{}, errors.New("peer cannot establish connection to itself")
	}

	//Ensure infohash matching; ensures that is downloaded in the correct file
	if handshake.InfoHash != job.Infohash {
		return PieceResult{}, errors.New("infohash mismatch")
	}

	//Download piece data
	pieceData, err := downloadPiece(connection, job.PieceIndex, job.PieceLength)
	if err != nil {
		return PieceResult{}, fmt.Errorf("failed to download piece: %v", err)
	}

	//Verify donwloaded piece integrity
	if !integrity.VerifyIntegrity(pieceData, job.ExpectedHash) {
		return PieceResult{}, errors.New("integrity check failed for downloaded piece")
	}
	return PieceResult{job.PieceIndex, pieceData, nil}, nil
}

// Worker function to process jobs from queue and send result
func pieceWorker(jobQueue <-chan PieceJob, resultsChannel chan<- PieceResult) {
	for job := range jobQueue {
		result, err := processJob(job)
		if err != nil {
			result.Err = err
			result.PieceIndex = job.PieceIndex
		}
		resultsChannel <- result
	}
}

// Starts workers to grab and process data from queue
func startWorkers(workerCount int, jobQueue chan PieceJob, resultsChannel chan<- PieceResult) error {
	if workerCount < 1 {
		return fmt.Errorf("Invalid number of workers")
	}
	for i := 0; i < workerCount; i++ {
		go pieceWorker(jobQueue, resultsChannel)
	}
	return nil
}

// Consume results from all workers
func collectResults(resultsChannel <-chan PieceResult, expectedResults int, onResult func(PieceResult)) (map[uint32][]byte, []uint32) {
	results := make(map[uint32][]byte, expectedResults)
	failedPieces := make([]uint32, 0)
	for i := 0; i < expectedResults; i++ {
		result := <-resultsChannel
		// Notify the caller whenever a piece finishes processing.
		if onResult != nil {
			onResult(result)
		}
		if result.Err != nil {
			fmt.Printf("Job failed for piece %d: %v\n",
				result.PieceIndex, result.Err)
			failedPieces = append(failedPieces, result.PieceIndex)
		} else {
			results[result.PieceIndex] = result.Data
		}
	}
	return results, failedPieces

}

func makeResultsChannel(jobCount int) chan PieceResult {
	return make(chan PieceResult, jobCount)
}

func DownloadPieces(jobs []PieceJob, workerCount int, onResult func(PieceResult)) (map[uint32][]byte, []uint32, error) {
	// Create job queue and results channel.
	queue := makeJobQueue()
	resultsChannel := makeResultsChannel(len(jobs))
	// Start worker goroutines.
	err := startWorkers(workerCount, queue, resultsChannel)
	if err != nil {
		return nil, nil, err
	}
	// Distribute jobs across the worker pool.
	go func() {
		defer close(queue)
		for _, job := range jobs {
			addJob(queue, job)
		}
	}()
	// Collect results and report progress through callback.
	results, failedPieces := collectResults(
		resultsChannel,
		len(jobs),
		onResult,
	)
	return results, failedPieces, nil
}
