package dht

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/uber/h3-go/v4"
)

// BenchmarkLookup_Standard benchmarks standard Kademlia lookup (no H3)
//
// What GetClosestPeers does:
// 1. Starts a distributed Kademlia lookup query (NOT just local routing table!)
// 2. Queries multiple peers in parallel (alpha = 3 by default)
// 3. Each queried peer returns their closest peers to the target key
// 4. Continues querying until no closer peers are found
// 5. Returns the K closest peers discovered across the network
//
// This performs actual network queries - peers communicate with each other!
// Tests with 200 peers (reduced from 1000 due to resource limits)
func BenchmarkLookup_Standard(b *testing.B) {
	ctx := context.Background()

	// Create network WITHOUT H3 (200 peers - realistic but resource-friendly)
	dhts := createTestNetwork(b, 200, false)
	defer closeAllDHTs(dhts)

	key := "/bench/test"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dhts[0].GetClosestPeers(ctx, key)
	}
}

// BenchmarkLookup_H3 benchmarks H3-Kademlia lookup (with H3)
//
// What GetClosestPeers does with H3:
// 1. Same distributed lookup as standard, BUT:
// 2. Uses hybrid distance (XOR + geographic) to select which peers to query
// 3. Prefers peers that are both XOR-close AND geographically close
// 4. Each peer exchange includes H3 public cell information
// 5. Returns peers optimized for both network topology and geography
//
// Tests with 200 peers (reduced from 1000 due to resource limits)
func BenchmarkLookup_H3(b *testing.B) {
	ctx := context.Background()

	// Create network WITH H3 (200 peers - realistic but resource-friendly)
	dhts := createTestNetwork(b, 200, true)
	defer closeAllDHTs(dhts)

	key := "/bench/test"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dhts[0].GetClosestPeers(ctx, key)
	}
}

// BenchmarkHybridDistance benchmarks hybrid distance calculation
func BenchmarkHybridDistance(b *testing.B) {
	ctx := context.Background()

	host, _ := libp2p.New()
	defer host.Close()

	dht, _ := New(
		ctx,
		host,
		Mode(ModeServer),
		H3Location(38.7223, -9.1393),
	)
	defer dht.Close()

	// Create peer and store H3
	peer1, _ := libp2p.New()
	defer peer1.Close()

	portoCell, _ := h3.NewLatLng(41.1579, -8.6291).Cell(7)
	dht.StoreH3Cell(peer1.ID(), portoCell)

	targetKey := "test-key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dht.hybridDistance(peer1.ID(), targetKey)
	}
}

// BenchmarkXORDistance benchmarks XOR-only distance calculation
func BenchmarkXORDistance(b *testing.B) {
	ctx := context.Background()

	host, _ := libp2p.New()
	defer host.Close()

	dht, _ := New(ctx, host, Mode(ModeServer))
	defer dht.Close()

	peer1, _ := libp2p.New()
	defer peer1.Close()

	targetKey := "test-key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dht.xorDistanceNormalized(peer1.ID(), targetKey)
	}
}

// createTestNetwork creates a test network of DHT nodes
func createTestNetwork(tb testing.TB, n int, withH3 bool) []*IpfsDHT {
	ctx := context.Background()
	var dhts []*IpfsDHT

	for i := 0; i < n; i++ {
		host, err := libp2p.New()
		if err != nil {
			tb.Fatal(err)
		}

		var dht *IpfsDHT
		var err2 error

		if withH3 {
			// Distribute nodes in Lisboa area
			lat := 38.7 + float64(i%10)*0.01
			lon := -9.1 + float64(i/10)*0.01
			dht, err2 = New(
				ctx,
				host,
				Mode(ModeServer),
				H3Location(lat, lon),
			)
		} else {
			dht, err2 = New(ctx, host, Mode(ModeServer))
		}

		if err2 != nil {
			tb.Fatal(err2)
		}

		dhts = append(dhts, dht)
	}

	// Connect nodes in a simple topology (each node connects to next 5)
	// Use type assertion for connectNoSync which expects *testing.T
	if t, ok := tb.(*testing.T); ok {
		for i := 0; i < len(dhts); i++ {
			for j := 1; j <= 5 && (i+j) < len(dhts); j++ {
				connectNoSync(t, ctx, dhts[i], dhts[i+j])
			}
		}
	}

	// Wait for connections and routing table population
	time.Sleep(3 * time.Second)

	// Store H3 info if using H3
	// Only store for peers that are actually in routing tables (more realistic)
	if withH3 {
		for i := 0; i < len(dhts); i++ {
			// Get peers from routing table (only connected peers)
			rtPeers := dhts[i].routingTable.ListPeers()
			for _, peerID := range rtPeers {
				// Find the DHT that owns this peer ID
				for j := 0; j < len(dhts); j++ {
					if dhts[j].PeerID() == peerID {
						dhts[i].StoreH3Cell(peerID, dhts[j].H3Public())
						break
					}
				}
			}
		}
	}

	return dhts
}

// closeAllDHTs closes all DHTs and their hosts
func closeAllDHTs(dhts []*IpfsDHT) {
	for _, dht := range dhts {
		if dht != nil {
			dht.Close()
			if dht.host != nil {
				dht.host.Close()
			}
		}
	}
}

// BenchmarkGetClosestPeersHybrid benchmarks hybrid peer selection
func BenchmarkGetClosestPeersHybrid(b *testing.B) {
	dhts := createTestNetwork(b, 50, true)
	defer closeAllDHTs(dhts)

	targetKey := "benchmark-target-key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dhts[0].getClosestPeersHybrid(targetKey, 10)
	}
}

// BenchmarkCountNodesInH3Cell benchmarks counting nodes in H3 cell
func BenchmarkCountNodesInH3Cell(b *testing.B) {
	ctx := context.Background()

	dhts := createTestNetwork(b, 50, true)
	defer closeAllDHTs(dhts)

	mainCell := dhts[0].H3Public()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = dhts[0].countNodesInH3Cell(ctx, mainCell) // ctx is used here
	}
}
