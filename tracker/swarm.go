package main

import (
	"sync"
)

type peer struct {
	peerId    string
	port      int
	completed bool
}

type swarm struct {
	//Identify peers via their id
	peerCollection map[string]*peer
	info_hash      string
	mu             sync.RWMutex
}

type swarmStore struct {
	//info_hash is the unique id for each swarm
	swarmCollection map[string]*swarm
	mu              sync.RWMutex
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

func markPeerCompleted(mySwarm *swarm, myPeer *peer) {
	mySwarm.mu.Lock()
	defer mySwarm.mu.Unlock()
	myPeer.completed = true
}

func removePeerFromSwarm(mySwarm *swarm, myPeer *peer) {
	mySwarm.mu.Lock()
	defer mySwarm.mu.Unlock()
	delete(mySwarm.peerCollection, myPeer.peerId)
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

func getSwarm(myStore *swarmStore, info_hash string) *swarm {
	myStore.mu.RLock()
	defer myStore.mu.RUnlock()

	return myStore.swarmCollection[info_hash]
}

func getPeer(mySwarm *swarm, peerId string) *peer {
	mySwarm.mu.RLock()
	defer mySwarm.mu.RUnlock()
	return mySwarm.peerCollection[peerId]
}

func getOrCreateSwarm(myStore *swarmStore, info_hash string) *swarm {
	myStore.mu.Lock()
	defer myStore.mu.Unlock() //Unlock upon exit

	_, exists := myStore.swarmCollection[info_hash]

	if !exists {
		myStore.swarmCollection[info_hash] = makeSwarm(info_hash)
	}
	return myStore.swarmCollection[info_hash]

}
