package dht

import (
	"sort"

	kb "github.com/libp2p/go-libp2p-kbucket"
	"github.com/libp2p/go-libp2p/core/peer"
)

// getClosestPeersHybrid returns the closest peers using hybrid distance (XOR + geographic)
// when H3 is enabled, otherwise falls back to standard Kademlia XOR distance.
func (dht *IpfsDHT) getClosestPeersHybrid(targetKey string, count int) []peer.ID {
	targetKadID := kb.ConvertKey(targetKey)

	// If H3 is disabled, use standard Kademlia routing
	if !dht.h3Enabled {
		return dht.routingTable.NearestPeers(targetKadID, count)
	}

	// Get candidates for hybrid distance calculation
	// For fair comparison with standard Kademlia, we use the same count
	// If we need more candidates for better geographic selection, we can increase this
	candidates := dht.routingTable.NearestPeers(targetKadID, count)
	if len(candidates) == 0 {
		return nil
	}

	// Calculate hybrid distance for each candidate
	type peerDist struct {
		peerID peer.ID
		dist   float64
	}
	distances := make([]peerDist, 0, len(candidates))

	for _, p := range candidates {
		if p == dht.self {
			continue // Skip self
		}
		dist := dht.hybridDistance(p, targetKey)
		distances = append(distances, peerDist{peerID: p, dist: dist})
	}

	// Sort by hybrid distance (higher = closer)
	// hybridDistance returns higher values for closer peers (consistent with XOR CPL)
	// So we sort in descending order to get closest peers first
	// Using sort.Slice for better performance than bubble sort
	sort.Slice(distances, func(i, j int) bool {
		return distances[i].dist > distances[j].dist // Descending order
	})

	// Return the closest 'count' peers
	result := make([]peer.ID, 0, min(len(distances), count))
	for i := 0; i < len(distances) && len(result) < count; i++ {
		result = append(result, distances[i].peerID)
	}

	return result
}

// hybridDistance calculates a combined distance using both XOR (network topology)
// and geographic (H3 cell) distance. This enables location-aware routing decisions.
// Returns: alpha × XOR_distance + beta × geo_distance
func (dht *IpfsDHT) hybridDistance(peerID peer.ID, targetKey string) float64 {
	if !dht.h3Enabled {
		// H3 disabled, use only XOR distance (normal Kademlia behavior)
		return dht.xorDistanceNormalized(peerID, targetKey)
	}

	// XOR component (network topology distance)
	xorDist := dht.xorDistanceNormalized(peerID, targetKey)

	// Geographic component (H3 cell distance)
	// Note: We invert the distance so that closer = higher value (consistent with XOR)
	geoDist := 0.5 // Default if no H3 info available
	dht.h3Mutex.RLock()
	if peerCell, ok := dht.h3PeerStore[peerID]; ok && dht.h3Public != 0 {
		dist, err := dht.h3Public.GridDistance(peerCell)
		if err == nil && dist < 10000 {
			// Normalize and invert: closer cells = higher value (1 - dist/10000)
			// This makes it consistent with XOR distance (higher = closer)
			geoDist = 1.0 - (float64(dist) / 10000.0)
		}
	}
	dht.h3Mutex.RUnlock()

	// Weighted combination: hybrid = alpha×XOR + beta×geo
	// Both components now have higher values for closer peers
	return dht.h3Alpha*xorDist + dht.h3Beta*geoDist
}

// xorDistanceNormalized converts Kademlia's XOR distance to a normalized value [0, 1].
// 0 = very far, 1 = very close (based on common prefix length).
func (dht *IpfsDHT) xorDistanceNormalized(peerID peer.ID, targetKey string) float64 {
	peerKadID := kb.ConvertPeerID(peerID)
	targetKadID := kb.ConvertKey(targetKey)

	// Calculate common prefix length (CPL) - how many leading bits match
	// Higher CPL = closer in keyspace
	cpl := kb.CommonPrefixLen(peerKadID, targetKadID)

	// Normalize: more common prefix = closer (higher value)
	// 256-bit keyspace, so max CPL = 256
	return float64(cpl) / 256.0
}
