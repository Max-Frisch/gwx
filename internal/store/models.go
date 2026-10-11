package store

import (
	"database/sql"
	"time"
)

// Planet is a full planet row.
type Planet struct {
	ID          int64
	UniverseID  string
	UserID      sql.NullInt64
	Name        string
	Galaxy      int
	System      int
	Position    int
	IsHomeworld bool
	DiameterKM  int
	FieldsMax   int
	FieldsUsed  int

	Metal    float64
	Lutinum  float64
	Water    float64
	Hydrogen float64

	MetalCap    int64
	LutinumCap  int64
	WaterCap    int64
	HydrogenCap int64

	MetalProd    float64
	LutinumProd  float64
	WaterProd    float64
	HydrogenProd float64

	EnergyUsed int64
	EnergyProd int64

	ProtectedMetal    int64
	ProtectedLutinum  int64
	ProtectedWater    int64
	ProtectedHydrogen int64

	LastCalc time.Time
}

// OwnedPlanet is a lightweight planet summary for the switcher.
type OwnedPlanet struct {
	ID          int64
	Name        string
	Galaxy      int
	System      int
	Position    int
	IsHomeworld bool
}

// ScanRow is one slot in a galaxy/system scan.
type ScanRow struct {
	Position    int
	PlanetID    sql.NullInt64
	Name        sql.NullString
	OwnerID     sql.NullInt64
	OwnerName   sql.NullString
	IsHomeworld bool
}

// Queue is a row in the generic queues table.
type Queue struct {
	ID           int64
	Kind         string
	Code         string
	TargetLevel  sql.NullInt64
	Quantity     sql.NullInt64
	QuantityDone int64
	StartAt      time.Time
	FinishAt     time.Time
	Status       string
}

// Fleet is an active fleet row plus its unit composition.
type Fleet struct {
	ID            int64
	Mission       string
	Phase         string
	OriginID      int64
	TargetID      sql.NullInt64
	OriginG       int
	OriginS       int
	OriginP       int
	TargetG       int
	TargetS       int
	TargetP       int
	StartAt       time.Time
	ArrivalAt     time.Time
	ReturnAt      sql.NullTime
	SpeedPct      float64
	HydrogenCost  int64
	CargoMetal    int64
	CargoLutinum  int64
	CargoWater    int64
	CargoHydrogen int64
	Units         map[string]int64
}

// Report is a stored battle or espionage report.
type Report struct {
	ID        int64
	Kind      string
	Title     string
	Payload   []byte
	CreatedAt time.Time
}
