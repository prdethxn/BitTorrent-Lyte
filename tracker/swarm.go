package main

import (
	"sync"
)

type peer struct {
	peerId string
	port   int
}

type swarm struct {
	//Identify peers via their id
	peerCollection map[string]*peer
	info_hash      string
	mu             sync.Mutex
}

type swarmStore struct {
	//info_hash is the unique id for each swarm
	swarmCollection map[string]*swarm
	mu              sync.Mutex
}

// constuctors
func makeSwarmStore() *swarmStore {
	return &swarmStore{
		swarmCollection: make(map[string]*swarm),
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

func addPeerToSwarm(mySwarm *swarm, myPeer *peer) {
	//lock other gorountes out while adding
	mySwarm.mu.Lock()
	defer mySwarm.mu.Unlock() //Unlock upon exit

	//Add peer to swarm
	mySwarm.peerCollection[myPeer.peerId] = myPeer
}

// Add swarm to swarm store
func addToStore(store *swarmStore, info_hash string, newSwarm *swarm) {
	//lock out other goroutines while check/writing
	store.mu.Lock()
	defer store.mu.Unlock() //Unlock when the upon exit

	//Add swarm if it doesn't exist
	_, exists := store.swarmCollection[info_hash]
	if !exists {
		store.swarmCollection[info_hash] = newSwarm
	}

}
