package dht

import (
	"context"
	"testing"
	"time"

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

func TestCountNodesInH3Cell(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create main DHT in Lisboa
	lisboaLat, lisbaLon := 38.7223, -9.1393
	dht1 := setupDHT(ctx, t, false, H3Location(lisboaLat, lisbaLon))
	require.True(t, dht1.IsH3Enabled())

	// Get Lisboa's public cell (resolution 7)
	lisboaCell := dht1.H3Public()
	require.NotEqual(t, h3.Cell(0), lisboaCell, "Lisboa cell should not be zero")
	t.Logf("Lisboa H3 Public Cell: %d (res %d)", lisboaCell, lisboaCell.Resolution())

	// Test 1: Empty routing table should return 0
	count := dht1.countNodesInH3Cell(ctx, lisboaCell)
	require.Equal(t, 0, count, "Should return 0 when routing table is empty")

	// Test 2: Create multiple DHTs in same area (Lisboa) and different area (New York)
	dht2 := setupDHT(ctx, t, false, H3Location(38.7300, -9.1500))  // Near Lisboa
	dht3 := setupDHT(ctx, t, false, H3Location(38.7100, -9.1300))  // Near Lisboa
	dht4 := setupDHT(ctx, t, false, H3Location(40.7128, -74.0060)) // New York (different)

	// Connect DHTs
	connectNoSync(t, ctx, dht1, dht2)
	connectNoSync(t, ctx, dht1, dht3)
	connectNoSync(t, ctx, dht1, dht4)

	// Wait for routing table updates
	wait(t, ctx, dht1, dht2)
	wait(t, ctx, dht1, dht3)
	wait(t, ctx, dht1, dht4)

	// Store H3 cells manually (simulating H3 info exchange)
	// In real scenario, this happens via extractH3FromMessage
	dht2Public := dht2.H3Public()
	dht3Public := dht3.H3Public()
	dht4Public := dht4.H3Public()

	dht1.StoreH3Cell(dht2.PeerID(), dht2Public)
	dht1.StoreH3Cell(dht3.PeerID(), dht3Public)
	dht1.StoreH3Cell(dht4.PeerID(), dht4Public)

	// Test 3: Count nodes in Lisboa cell
	// Should count dht2 and dht3 (both near Lisboa), but not dht4 (New York)
	count = dht1.countNodesInH3Cell(ctx, lisboaCell)
	t.Logf("Count in Lisboa cell: %d", count)

	// Verify dht2 and dht3 are in same cell (or child of Lisboa cell)
	dht2Parent, err := dht2Public.Parent(7)
	require.NoError(t, err)
	dht3Parent, err := dht3Public.Parent(7)
	require.NoError(t, err)

	// Check if they match Lisboa cell
	dht2InLisboa := (dht2Parent == lisboaCell) || (dht2Public == lisboaCell)
	dht3InLisboa := (dht3Parent == lisboaCell) || (dht3Public == lisboaCell)

	t.Logf("dht2 cell: %d (res %d), parent: %d, in Lisboa: %v", dht2Public, dht2Public.Resolution(), dht2Parent, dht2InLisboa)
	t.Logf("dht3 cell: %d (res %d), parent: %d, in Lisboa: %v", dht3Public, dht3Public.Resolution(), dht3Parent, dht3InLisboa)
	t.Logf("Lisboa cell: %d (res %d)", lisboaCell, lisboaCell.Resolution())

	// Count how many should be in Lisboa cell
	expectedCount := 0
	if dht2InLisboa {
		expectedCount++
	}
	if dht3InLisboa {
		expectedCount++
	}

	t.Logf("Expected count in Lisboa cell: %d (dht2: %v, dht3: %v)", expectedCount, dht2InLisboa, dht3InLisboa)
	require.Equal(t, expectedCount, count, "Count should match expected peers in Lisboa cell")

	// Test 4: Count nodes in New York cell
	nyCell := dht4Public
	countNY := dht1.countNodesInH3Cell(ctx, nyCell)
	t.Logf("Count in New York cell: %d (expected at least 1: dht4)", countNY)
	if nyCell != 0 {
		require.GreaterOrEqual(t, countNY, 1, "Should count dht4 in New York cell")
	}

	// Test 5: Test with zero cell
	count = dht1.countNodesInH3Cell(ctx, h3.Cell(0))
	require.Equal(t, 0, count, "Should return 0 for zero cell")

	t.Logf("countNodesInH3Cell tests passed")
}
