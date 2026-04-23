package agentserver

import "net"

// netListenReal is the default implementation used by the test helper in
// server_test.go. Kept here so it remains exported at package scope for
// tests across the package.
func netListenReal(network, addr string) (net.Listener, error) {
	return net.Listen(network, addr)
}
