package tally

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	// DefaultPort is the default Tally Prime XML server port.
	DefaultPort = 9000
	// MaxPort is the upper bound of the port scan range.
	MaxPort = 9010
	// discoverTimeout is the per-port connection timeout during discovery.
	discoverTimeout = 2 * time.Second
)

// Discover scans localhost ports DefaultPort-MaxPort for a running Tally instance.
// Returns the port number on success, error if no Tally found.
func Discover(ctx context.Context, host string) (int, error) {
	return DiscoverWithPorts(ctx, host, DefaultPort, MaxPort)
}

// DiscoverWithPorts scans the given port range on host for a running Tally instance.
// It creates a short-timeout HTTP client for each port and attempts a GetCompanyInfo call.
// Returns the first port that responds with valid company info.
func DiscoverWithPorts(ctx context.Context, host string, startPort, endPort int) (int, error) {
	for port := startPort; port <= endPort; port++ {
		client := &Client{
			baseURL: fmt.Sprintf("http://%s:%d", host, port),
			httpClient: &http.Client{
				Timeout: discoverTimeout,
			},
		}
		info, err := client.GetCompanyInfo(ctx)
		if err == nil && info != nil {
			log.Printf("[tally] discovered Tally on port %d (company: %s)", port, info.Name)
			return port, nil
		}
	}
	return 0, fmt.Errorf("no Tally instance found on %s ports %d-%d", host, startPort, endPort)
}
