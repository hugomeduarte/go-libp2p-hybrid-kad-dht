package dht

import (
	"errors"
	"fmt"

	dhtcfg "github.com/libp2p/go-libp2p-kad-dht/internal/config"
	"github.com/uber/h3-go/v4"
)

// H3Location configures the node's actual geographic location for H3 cell computation.
// This sets the h3_actual cell at resolution 12 (~100m precision).
// The h3_public cell will be automatically computed at resolution 7 (~5km precision).
func H3Location(latitude, longitude float64) Option {
	return func(c *dhtcfg.Config) error {
		c.H3Latitude = latitude
		c.H3Longitude = longitude
		return nil
	}
}

// KAnonymity configures the target k value for k-anonymity privacy guarantee.
// The h3_public cell must contain at least k nodes to maintain anonymity.
// Defaults to 50.
func KAnonymity(k int) Option {
	return func(c *dhtcfg.Config) error {
		c.KAnonymity = k
		return nil
	}
}

// WithH3Weights configures the weights for hybrid distance calculation.
// alpha = weight for XOR distance (network topology)
// beta = weight for geographic distance (H3 cell distance)
// alpha + beta must equal 1.0
// Defaults: alpha=0.5, beta=0.5
func WithH3Weights(alpha, beta float64) Option {
	return func(c *dhtcfg.Config) error {
		if alpha+beta != 1.0 {
			return errors.New("alpha + beta must equal 1.0")
		}
		c.H3Alpha = alpha
		c.H3Beta = beta
		return nil
	}
}

// WithH3Resolution sets the public H3 resolution (overrides default resolution 7).
// Lower resolution = larger cells = more privacy but less precision.
// Valid range: 0-15
func WithH3Resolution(resolution int) Option {
	return func(c *dhtcfg.Config) error {
		if resolution < 0 || resolution > 15 {
			return errors.New("H3 resolution must be between 0 and 15")
		}
		// Store the resolution in config
		c.H3Resolution = resolution

		// Validate the resolution works with coordinates if they're already set
		// (The actual h3_public will be computed later in makeDHT using this resolution)
		if c.H3Latitude != 0 || c.H3Longitude != 0 {
			latLng := h3.NewLatLng(c.H3Latitude, c.H3Longitude)
			h3Actual, err := latLng.Cell(12)
			if err != nil {
				return fmt.Errorf("failed to create H3 cell: %w", err)
			}
			// Validate that we can compute parent at this resolution (error check only)
			if _, err := h3Actual.Parent(resolution); err != nil {
				return fmt.Errorf("invalid H3 resolution %d for coordinates: %w", resolution, err)
			}
		}
		return nil
	}
}
