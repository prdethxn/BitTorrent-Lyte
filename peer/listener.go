package peer

import (
	"fmt"
	"net"
)

// Create and return listening socket
func Listen(address string) (net.Listener, error) {
	outcome, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}
	return outcome, nil
}
