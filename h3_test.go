package dht

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber/h3-go/v4"
)

func TestH3Enabled(t *testing.T) {
	ctx := context.Background()

	// Create DHT with H3 location (Lisboa, Portugal)
	lat, lon := 38.7223, -9.1393
	dht := setupDHT(ctx, t, false, H3Location(lat, lon))

	// Verify H3 is enabled
	require.True(t, dht.IsH3Enabled(), "H3 should be enabled when location is provided")

	// Verify actual cell (resolution 12)
	actualCell := dht.H3Actual()
	require.NotEqual(t, h3.Cell(0), actualCell, "H3 actual cell should not be zero")
	t.Logf("H3 Actual Cell (res 12): %d", actualCell)

	// Verify public cell (resolution 7)
	publicCell := dht.H3Public()
	require.NotEqual(t, h3.Cell(0), publicCell, "H3 public cell should not be zero")
	t.Logf("H3 Public Cell (res 7): %d", publicCell)

	// Verify public cell is parent of actual cell
	parentOfActual, err := actualCell.Parent(7)
	require.NoError(t, err)
	require.Equal(t, publicCell, parentOfActual, "Public cell should be parent of actual cell at resolution 7")

	// Verify k-anonymity value
	kAnon := dht.KAnonymity()
	require.Greater(t, kAnon, 0, "K-anonymity should be greater than 0")
	t.Logf("K-Anonymity: %d", kAnon)
}

func TestH3Disabled(t *testing.T) {
	ctx := context.Background()

	// Create DHT without H3 location
	dht := setupDHT(ctx, t, false)

	// Verify H3 is disabled
	require.False(t, dht.IsH3Enabled(), "H3 should be disabled when no location is provided")

	// Verify cells are zero
	actualCell := dht.H3Actual()
	publicCell := dht.H3Public()
	require.Equal(t, h3.Cell(0), actualCell, "H3 actual cell should be zero when disabled")
	require.Equal(t, h3.Cell(0), publicCell, "H3 public cell should be zero when disabled")
}

func TestH3Weights(t *testing.T) {
	ctx := context.Background()

	// Create DHT with custom H3 weights
	dht := setupDHT(ctx, t, false,
		H3Location(40.7128, -74.0060), // New York
		WithH3Weights(0.7, 0.3),       // 70% XOR, 30% geo
	)

	require.True(t, dht.IsH3Enabled())

	alpha, beta := dht.GetH3Weights()
	require.Equal(t, 0.7, alpha, "Alpha weight should be 0.7")
	require.Equal(t, 0.3, beta, "Beta weight should be 0.3")
	require.InDelta(t, 1.0, alpha+beta, 0.001, "Alpha + Beta should equal 1.0")
}

func TestH3KAnonymity(t *testing.T) {
	ctx := context.Background()

	// Create DHT with custom k-anonymity
	customK := 100
	dht := setupDHT(ctx, t, false,
		H3Location(51.5074, -0.1278), // London
		KAnonymity(customK),
	)

	require.True(t, dht.IsH3Enabled())
	require.Equal(t, customK, dht.KAnonymity(), "K-anonymity should match configured value")
}

func TestH3Resolution(t *testing.T) {
	ctx := context.Background()

	// Test with custom resolution (5 instead of default 7)
	customResolution := 5
	dht := setupDHT(ctx, t, false,
		H3Location(40.7128, -74.0060), // New York
		WithH3Resolution(customResolution),
	)

	require.True(t, dht.IsH3Enabled())

	// Get the actual and public cells
	actualCell := dht.H3Actual()
	publicCell := dht.H3Public()

	// Verify public cell is parent at resolution 5
	expectedPublic, err := actualCell.Parent(customResolution)
	require.NoError(t, err)
	require.Equal(t, expectedPublic, publicCell, "Public cell should be parent at resolution 5")

	t.Logf("H3 Actual Cell (res 12): %d", actualCell)
	t.Logf("H3 Public Cell (res %d): %d", customResolution, publicCell)
}

func TestH3StoreAndRetrievePeerCell(t *testing.T) {
	ctx := context.Background()

	dht := setupDHT(ctx, t, false, H3Location(38.7223, -9.1393))
	require.True(t, dht.IsH3Enabled())

	// Create a test peer ID
	testPeerID := dht.PeerID() // Use self for testing

	// Create a test H3 cell (different location)
	testCell, err := h3.NewLatLng(40.7128, -74.0060).Cell(7) // New York at res 7
	require.NoError(t, err)

	// Store the cell
	dht.StoreH3Cell(testPeerID, testCell)

	// Retrieve the cell
	retrievedCell, found := dht.GetH3Cell(testPeerID)
	require.True(t, found, "Cell should be found after storing")
	require.Equal(t, testCell, retrievedCell, "Retrieved cell should match stored cell")
}

