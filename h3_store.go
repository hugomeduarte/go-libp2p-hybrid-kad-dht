package dht

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/uber/h3-go/v4"
)

// H3Actual returns the node's actual H3 cell at resolution 12 (~100m precision).
func (dht *IpfsDHT) H3Actual() h3.Cell {
	return dht.h3Actual
}

// H3Public returns the node's public H3 cell at resolution 7 (~5km precision).
// This is the location published to the network for privacy-preserving routing.
func (dht *IpfsDHT) H3Public() h3.Cell {
	return dht.h3Public
}

// KAnonymity returns the target k value for k-anonymity privacy guarantee.
func (dht *IpfsDHT) KAnonymity() int {
	return dht.kAnonymity
}

// IsH3Enabled returns whether H3 geographic awareness is enabled.
func (dht *IpfsDHT) IsH3Enabled() bool {
	return dht.h3Enabled
}

// GetH3Weights returns the current alpha (XOR) and beta (geographic) weights.
func (dht *IpfsDHT) GetH3Weights() (alpha, beta float64) {
	return dht.h3Alpha, dht.h3Beta
}

// StoreH3Cell stores a peer's H3 public cell in the peer store.
// This is called when we learn about a peer's location (from protocol messages, queries, etc.).
func (dht *IpfsDHT) StoreH3Cell(p peer.ID, cell h3.Cell) {
	if !dht.h3Enabled || cell == 0 {
		return
	}

	dht.h3Mutex.Lock()
	defer dht.h3Mutex.Unlock()

	dht.h3PeerStore[p] = cell
	logger.Debugw("stored peer H3 cell", "peer", p, "h3_cell", cell)
}

// GetH3Cell retrieves a peer's H3 public cell from the peer store.
// Returns (cell, true) if found, (0, false) if not found or H3 disabled.
func (dht *IpfsDHT) GetH3Cell(p peer.ID) (h3.Cell, bool) {
	if !dht.h3Enabled {
		return h3.Cell(0), false
	}

	dht.h3Mutex.RLock()
	defer dht.h3Mutex.RUnlock()

	cell, ok := dht.h3PeerStore[p]
	return cell, ok
}
