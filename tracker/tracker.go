package main

import (
	"fmt"
	"log"
	"net/http"
)

//Decides what to do with a request

// Manage incoming requests
func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Endpoint Reached")

}

func main() {
	//Route requests hitting /annouce to handler
	http.HandleFunc("/announce", handler)

	//Start server on port 8080
	fmt.Println("Starting Server on Port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}
