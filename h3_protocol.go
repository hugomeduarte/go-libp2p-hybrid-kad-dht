package dht

import (
	pb "github.com/libp2p/go-libp2p-kad-dht/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/uber/h3-go/v4"
)

// addH3ToPBPeers adds H3 public cells to protobuf peer messages.
// This is called when sending peers in DHT responses to share geographic information.
func (dht *IpfsDHT) addH3ToPBPeers(pbPeers []*pb.Message_Peer) {
	if !dht.h3Enabled || dht.h3Public == 0 {
		return
	}

	dht.h3Mutex.RLock()
	defer dht.h3Mutex.RUnlock()

	for _, pbPeer := range pbPeers {
		if pbPeer == nil {
			continue
		}
		peerID := peer.ID(pbPeer.Id)

		// If this is ourselves, include our h3Public
		if peerID == dht.self {
			pbPeer.H3PublicCell = uint64(dht.h3Public)
			continue
		}

		// If we have this peer's H3 cell stored, include it
		if cell, ok := dht.h3PeerStore[peerID]; ok && cell != 0 {
			pbPeer.H3PublicCell = uint64(cell)
		}
	}
}

// extractH3FromMessage extracts H3 public cells from a DHT message and stores them.
// This is called when receiving DHT messages to learn about peers' geographic locations.
func (dht *IpfsDHT) extractH3FromMessage(msg *pb.Message) {
	if !dht.h3Enabled {
		return
	}

	// Extract H3 from closer peers
	for _, pbPeer := range msg.GetCloserPeers() {
		if pbPeer == nil {
			continue
		}
		if h3Cell := pbPeer.GetH3PublicCell(); h3Cell != 0 {
			peerID := peer.ID(pbPeer.Id)
			if peerID != dht.self && peerID != "" {
				dht.StoreH3Cell(peerID, h3.Cell(h3Cell))
			}
		}
	}

	// Extract H3 from provider peers
	for _, pbPeer := range msg.GetProviderPeers() {
		if pbPeer == nil {
			continue
		}
		if h3Cell := pbPeer.GetH3PublicCell(); h3Cell != 0 {
			peerID := peer.ID(pbPeer.Id)
			if peerID != dht.self && peerID != "" {
				dht.StoreH3Cell(peerID, h3.Cell(h3Cell))
			}
		}
	}
}
