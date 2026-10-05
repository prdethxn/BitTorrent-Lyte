package tracker

type peer struct {
	peerId string
	port   int
}

type swarm struct {
	//Identify peers via their id
	peerCollection map[string]*peer
	info_hash      string
}

type swarmStore struct {
	//info_hash is the unique id for each swarm
	swarmCollection map[string]swarm
}

// constuctors
func makeSwarmStore() *swarmStore {
	return &swarmStore{
		swarmCollection: make(map[string]swarm),
	}
}

func makeSwarm(swarm_hash string) *swarm {
	return &swarm{
		peerCollection: make(map[string]*peer),
		info_hash:      swarm_hash,
	}
}

func makePeer(id string, portNum int) *peer {
	return &peer{
		peerId: id,
		port:   portNum,
	}
}
