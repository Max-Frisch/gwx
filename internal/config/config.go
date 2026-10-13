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

	UniverseCode string
	UniverseName string
	GameSpeed    float64
	FleetSpeed   float64
	MaxGalaxies  int
	MaxSystems   int
	EndsAt       *time.Time

	// BuildQueueSlots is how many building construction orders a planet may
	// queue at once (the reference construction queue holds 6).
	BuildQueueSlots int

	// ResearchQueueSlots is how many research orders an account may queue at
	// once (the reference research queue holds 4).
	ResearchQueueSlots int

	// NPCCount is how many self-playing NPC accounts the universe should hold.
	// The server seeds up to this many on start (it only ever adds). 0 disables
	// NPCs entirely.
	NPCCount int

	// NPCBatch caps how many NPCs one scheduler tick may let think, so a large
	// universe cannot starve the queue/fleet resolvers. Default 25.
	NPCBatch int

	// NPCRaid enables the opt-in NPC offense: scout a target and attack when
	// favorable. Off (default) is the defensive-only brain. NPCRaidChance
	// multiplies the profile's scout probability (default 0.15), and NPCRaidMax
	// caps ATTACK dispatches per tick (default 5).
	NPCRaid       bool
	NPCRaidChance float64
	NPCRaidMax    int
}

// Load reads and validates configuration. DATABASE_URL is the only required
// variable.
func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	// Defaults are 1x/1x. GalaxyWars has exactly two speed modifiers: a game
	// speed (construction, research and resource production together) and a
	// fleet speed. Resource production is therefore not split out separately.
	gameSpeed, err := floatEnv("GAME_SPEED", 1)
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
	queueSlots, err := intEnv("BUILD_QUEUE_SLOTS", 6)
	if err != nil {
		return nil, err
	}
	researchSlots, err := intEnv("RESEARCH_QUEUE_SLOTS", 4)
	if err != nil {
		return nil, err
	}
	npcCount, err := countEnv("NPC_COUNT", 0)
	if err != nil {
		return nil, err
	}
	npcBatch, err := intEnv("NPC_BATCH", 25)
	if err != nil {
		return nil, err
	}
	npcRaid, err := boolEnv("NPC_RAID", false)
	if err != nil {
		return nil, err
	}
	npcRaidChance, err := unitFloatEnv("NPC_RAID_CHANCE", 0.15)
	if err != nil {
		return nil, err
	}
	npcRaidMax, err := intEnv("NPC_RAID_MAX", 5)
	if err != nil {
		return nil, err
	}

	return &Config{
		Env:                getenv("APP_ENV", "development"),
		HTTPAddr:           getenv("HTTP_ADDR", ":8090"),
		DatabaseURL:        dbURL,
		UniverseCode:       getenv("UNIVERSE_CODE", "universe_gwx_1"),
		UniverseName:       getenv("UNIVERSE_NAME", "Classic Universe"),
		GameSpeed:          gameSpeed,
		FleetSpeed:         fleetSpeed,
		MaxGalaxies:        maxGal,
		MaxSystems:         maxSys,
		EndsAt:             endsAt,
		BuildQueueSlots:    queueSlots,
		ResearchQueueSlots: researchSlots,
		NPCCount:           npcCount,
		NPCBatch:           npcBatch,
		NPCRaid:            npcRaid,
		NPCRaidChance:      npcRaidChance,
		NPCRaidMax:         npcRaidMax,
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

// countEnv is like intEnv but permits zero (used for optional counts).
func countEnv(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid %s: must be zero or a positive integer", key)
	}
	return v, nil
}

// boolEnv reads an optional boolean, accepting 1/0/true/false (case-insensitive).
func boolEnv(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	switch strings.ToLower(raw) {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s: must be one of 1/0/true/false", key)
	}
}

// unitFloatEnv reads an optional number in (0,1] (used for probability scales).
func unitFloatEnv(key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 || v > 1 {
		return 0, fmt.Errorf("invalid %s: must be a number in (0,1]", key)
	}
	return v, nil
}
