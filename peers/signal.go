package peers

import (
	"github.com/ethereum/go-ethereum/p2p"

	"github.com/planq-network/status-go/signal"
)

// SendDiscoverySummary sends discovery.summary signal.
func SendDiscoverySummary(peers []*p2p.PeerInfo) {
	signal.SendDiscoverySummary(peers)
}
