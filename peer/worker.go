package peer

type PieceJob struct {
	PieceIndex int //Identifies which piece the worker is assigned
}

func makeJobQueue() chan PieceJob {
	return make(chan PieceJob, 10) //Buffer enables workers to get queued without being assigned to workers.
}

func addJob(jobQueue chan PieceJob, job PieceJob) {
	jobQueue <- job
}

// Worker function to process jobs from queue
func pieceWorker(jobQueue <-chan PieceJob) {
	for job := range jobQueue {
		//process job
	}
}
