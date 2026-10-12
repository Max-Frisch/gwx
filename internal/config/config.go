// Package config loads runtime configuration from the environment. The universe
// block (speeds, bounds, optional seasonal end) is applied/upserted on every
// server start, so operators tune a round by editing env and restarting.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the full process configuration.
type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseURL string

	UniverseCode  string
	UniverseName  string
	GameSpeed     float64
	ResourceSpeed float64
	FleetSpeed    float64
	MaxGalaxies   int
	MaxSystems    int
	EndsAt        *time.Time
}

// Load reads and validates configuration. DATABASE_URL is the only required
// variable.
func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	// Defaults are 1x/1x/1x. The classic GalaxyWars reference captures ran at
	// 2x game speed / 1x fleet speed; the catalog values fitted from them are
	// therefore 2x-speed figures. Keeping gwx at 1x lets a round re-scale them
	// cleanly through these knobs instead of baking the capture's speed in.
	gameSpeed, err := floatEnv("GAME_SPEED", 1)
	if err != nil {
		return nil, err
	}
	// RESOURCE_SPEED defaults to GAME_SPEED: one knob speeds build, research and
	// production together, while RESOURCE_SPEED can still split production out.
	resSpeed, err := floatEnv("RESOURCE_SPEED", gameSpeed)
	if err != nil {
		return nil, err
	}
	fleetSpeed, err := floatEnv("FLEET_SPEED", 1)
	if err != nil {
		return nil, err
	}
	maxGal, err := intEnv("MAX_GALAXIES", 3)
	if err != nil {
		return nil, err
	}
	maxSys, err := intEnv("MAX_SYSTEMS", 199)
	if err != nil {
		return nil, err
	}

	endsAt, err := parseEndsAt(os.Getenv("ENDS_AT"))
	if err != nil {
		return nil, err
	}

	return &Config{
		Env:           getenv("APP_ENV", "development"),
		HTTPAddr:      getenv("HTTP_ADDR", ":8090"),
		DatabaseURL:   dbURL,
		UniverseCode:  getenv("UNIVERSE_CODE", "universe_gwx_1"),
		UniverseName:  getenv("UNIVERSE_NAME", "Classic Universe"),
		GameSpeed:     gameSpeed,
		ResourceSpeed: resSpeed,
		FleetSpeed:    fleetSpeed,
		MaxGalaxies:   maxGal,
		MaxSystems:    maxSys,
		EndsAt:        endsAt,
	}, nil
}

func parseEndsAt(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("invalid ENDS_AT (want RFC3339, e.g. 2027-01-01T00:00:00Z): %w", err)
	}
	return &t, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func floatEnv(key string, fallback float64) (float64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid %s: must be a positive number", key)
	}
	return v, nil
}

func intEnv(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid %s: must be a positive integer", key)
	}
	return v, nil
}
