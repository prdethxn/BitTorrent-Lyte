package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
)

// Define regex
var hex64Regex = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// Manage incoming requests, allowing handler to access store
func makeHandler(myStore *swarmStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		//Get parameters
		queryParams := r.URL.Query()

		//Port number
		portValues, exists := queryParams["port"]
		if !exists {
			http.Error(w, "Port Value Doesn't Exist", http.StatusBadRequest)
			return
		}
		//Convert port number to INT & keep it within TCP range
		portNum, err := strconv.Atoi(portValues[0])

		if err != nil || portNum < 1 || portNum > 65535 {
			http.Error(w, "Bad Port Value Given", http.StatusBadRequest)
			return
		}
		//info_hash, which identifies which swarm the peer is announcing to
		infoHash, exists := queryParams["info_hash"]
		if !exists {
			http.Error(w, "InfoHash Value Doesn't Exist", http.StatusBadRequest)
			return
		}

		infoHashValue := infoHash[0]

		if !hex64Regex.MatchString(infoHashValue) {
			http.Error(w, "Invalid InfoHash Value Given", http.StatusBadRequest)
			return
		}

		//Peer id (who is making this request)
		peerId, exists := queryParams["peer_id"]

		if !exists {
			http.Error(w, "peerId Value Doesn't Exist", http.StatusBadRequest)
			return
		}

		peerIdValue := peerId[0]

		if len(peerIdValue) == 0 {
			http.Error(w, "Invalid peerId Value Given", http.StatusBadRequest)
			return
		}

		//transactionId  (associate announce request with transaction)
		transactionId, exists := queryParams["transaction_id"]

		if !exists {
			http.Error(w, "transactionId Value Doesn't Exist", http.StatusBadRequest)
			return
		}
		//Make transaciton ID an INT
		transactionIdValue, err := strconv.Atoi(transactionId[0])

		if err != nil {
			http.Error(w, "Invalid transactionId Value Given", http.StatusBadRequest)
			return
		}

		//event
		allowedEvents := map[string]struct{}{
			"completed": {},
			"stopped":   {},
			"started":   {},
		}

		event, exists := queryParams["event"]

		if !exists {
			http.Error(w, "event Value Doesn't Exist", http.StatusBadRequest)
			return
		}

		eventValue := event[0]

		if _, exists := allowedEvents[eventValue]; !exists {
			http.Error(w, "Invalid event Value Given", http.StatusBadRequest)
			return
		}

		//Respond if all parameters are gotten sucessfully

		//Set content type
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		//Send status code and transaction id
		w.WriteHeader(http.StatusOK)

		//Convert from int to string prior to sending
		stringTransactionIdValue := strconv.Itoa(transactionIdValue)
		w.Write([]byte(stringTransactionIdValue))

		//Do something based on the event
		switch eventValue {
		case "started":
			//Add peer to swarm
			mySwarm := getOrCreateSwarm(myStore, infoHashValue)
			myPeer := makePeer(peerIdValue, portNum)
			addPeerToSwarm(mySwarm, myPeer)

		case "stopped":
			//Remove peer from swarm
			currentSwarm := getSwarm(myStore, infoHashValue)

			//If swarm/peer don't exist, exit
			if currentSwarm == nil {
				break
			}

			currentPeer := getPeer(currentSwarm, peerIdValue)

			if currentPeer == nil {
				break
			}

			removePeerFromSwarm(currentSwarm, currentPeer)

		case "completed":
			mySwarm := getSwarm(myStore, infoHashValue)
			if mySwarm == nil {
				break
			}
			myPeer := getPeer(mySwarm, peerIdValue)
			if myPeer == nil {
				break
			}
			markPeerCompleted(mySwarm, myPeer)
		}

	}

}

func main() {
	store := makeSwarmStore()

	//Route requests hitting /annouce to handler
	http.HandleFunc("/announce", makeHandler(store))

	//Start server on port 8080
	fmt.Println("Starting Server on Port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Error starting server: ", err)
	}

}
