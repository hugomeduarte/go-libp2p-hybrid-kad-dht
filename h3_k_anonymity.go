package dht

import (
	"context"

	"github.com/uber/h3-go/v4"
)

// ensureKAnonymity ensures that the h3_public cell meets the k-anonymity requirement.
// If the current h3_public cell has fewer than k nodes, it reduces the resolution
// (making the cell larger) until the requirement is met.
// This function queries the network to count nodes in the same h3_public cell.
func (dht *IpfsDHT) ensureKAnonymity(ctx context.Context) error {
	if dht.h3Public == 0 {
		return nil // No H3 location configured
	}

	// Start with resolution 7
	resolution := 7
	h3Public := dht.h3Public

	// Try to find nodes in the same h3_public cell
	// We'll query the routing table and network to count nodes
	for resolution >= 0 {
		// Count nodes in the same h3_public cell from routing table
		count := dht.countNodesInH3Cell(ctx, h3Public)

		if count >= dht.kAnonymity {
			// Privacy guarantee met
			dht.h3Public = h3Public
			logger.Debugw("k-anonymity requirement met", "h3_public", h3Public, "count", count, "k", dht.kAnonymity, "resolution", resolution)
			return nil
		}

		// If we haven't met k-anonymity, reduce resolution (make cell larger)
		if resolution > 0 {
			resolution--
			var err error
			h3Public, err = dht.h3Actual.Parent(resolution)
			if err != nil {
				logger.Errorw("failed to reduce H3 resolution", "error", err, "resolution", resolution)
				break
			}
			logger.Debugw("reducing H3 resolution for k-anonymity", "new_resolution", resolution, "count", count, "k", dht.kAnonymity)
		} else {
			// Can't reduce further, log warning
			logger.Warnw("unable to meet k-anonymity requirement", "h3_public", h3Public, "count", count, "k", dht.kAnonymity, "resolution", resolution)
			dht.h3Public = h3Public
			return nil
		}
	}

	return nil
}

// countNodesInH3Cell counts the number of nodes in the routing table that share
// the same h3_public cell (or are in a child cell of the given cell).
// It uses the h3PeerStore to check each peer's H3 public cell.
func (dht *IpfsDHT) countNodesInH3Cell(ctx context.Context, h3Cell h3.Cell) int {
	if h3Cell == 0 {
		return 0
	}

	peers := dht.routingTable.ListPeers()
	if len(peers) == 0 {
		return 0
	}

	count := 0
	dht.h3Mutex.RLock()
	defer dht.h3Mutex.RUnlock()

	// Get the resolution of the target cell
	targetResolution := h3Cell.Resolution()

	for _, peerID := range peers {
		// Skip self
		if peerID == dht.self {
			continue
		}

		// Check if we have H3 info for this peer
		peerCell, ok := dht.h3PeerStore[peerID]
		if !ok || peerCell == 0 {
			continue // Don't have H3 info for this peer
		}

		// Check if peer's cell is the same or a child of the target cell
		// We need to check if the peer's cell at the target resolution matches
		peerCellResolution := peerCell.Resolution()

		if peerCellResolution == targetResolution {
			// Same resolution - direct comparison
			if peerCell == h3Cell {
				count++
			}
		} else if peerCellResolution > targetResolution {
			// Peer's cell is more specific (higher resolution) - check if it's a child
			peerParent, err := peerCell.Parent(targetResolution)
			if err == nil && peerParent == h3Cell {
				count++
			}
		} else {
			// Peer's cell is less specific (lower resolution) - check if target is a child
			targetParent, err := h3Cell.Parent(peerCellResolution)
			if err == nil && targetParent == peerCell {
				count++
			}
		}
	}

	return count
}
