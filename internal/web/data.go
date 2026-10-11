package web

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gwx/internal/game"
	"gwx/internal/store"
)

// baseData is embedded in every page's view data.
type baseData struct {
	Title          string
	Username       string
	ActivePlanetID int64
	Planets        []store.OwnedPlanet
	Resources      *resView
	Flash          string
	Error          string
}

type resView struct {
	Metal, Lutinum, Water, Hydrogen                 float64
	MetalCap, LutinumCap, WaterCap, HydrogenCap     int64
	MetalProd, LutinumProd, WaterProd, HydrogenProd float64
	EnergyProd, EnergyUsed                          int64
}

func newResView(p *store.Planet) *resView {
	return &resView{
		Metal: p.Metal, Lutinum: p.Lutinum, Water: p.Water, Hydrogen: p.Hydrogen,
		MetalCap: p.MetalCap, LutinumCap: p.LutinumCap, WaterCap: p.WaterCap, HydrogenCap: p.HydrogenCap,
		MetalProd: p.MetalProd, LutinumProd: p.LutinumProd, WaterProd: p.WaterProd, HydrogenProd: p.HydrogenProd,
		EnergyProd: p.EnergyProd, EnergyUsed: p.EnergyUsed,
	}
}

type queueView struct {
	ID        int64
	Kind      string
	Name      string
	Remaining string
}

func newQueueView(q store.Queue) queueView {
	name := q.Code
	if d, ok := game.Get(q.Code); ok {
		name = d.Name
		if q.TargetLevel.Valid {
			name = fmt.Sprintf("%s → %d", d.Name, q.TargetLevel.Int64)
		}
	}
	return queueView{ID: q.ID, Kind: q.Kind, Name: name, Remaining: until(q.FinishAt)}
}

type buildRow struct {
	Code       string
	Name       string
	Level      int
	Unlocked   bool
	Missing    []string
	Affordable bool
	Queued     bool
	CostStr    string
	TimeStr    string
}

type unitRow struct {
	Code          string
	Name          string
	Count         int64
	Unlocked      bool
	Missing       []string
	CostStr       string
	PerTimeStr    string
	MaxAffordable int64
}

type shipOption struct {
	Code  string
	Name  string
	Count int64
}

type fleetRow struct {
	ID        int64
	Mission   string
	Phase     string
	From      string
	To        string
	Remaining string
	CanRecall bool
}

type galaxyRow struct {
	Position    int
	PlanetID    int64
	Name        string
	OwnerName   string
	IsHomeworld bool
}

type reportRow struct {
	Kind  string
	Title string
	When  string
	Body  string
}

type authData struct {
	baseData
	Login string
	Email string
}

type overviewData struct {
	baseData
	Planet   *store.Planet
	Queues   []queueView
	Incoming []fleetRow
}

type buildingsData struct {
	baseData
	Rows  []buildRow
	Queue *queueView
}

type researchData struct {
	baseData
	Rows  []buildRow
	Queue *queueView
}

type shipyardData struct {
	baseData
	Ships   []unitRow
	Defense []unitRow
	Queue   *queueView
}

type fleetData struct {
	baseData
	Ships    []shipOption
	Active   []fleetRow
	Incoming []fleetRow
	OriginG  int
	OriginS  int
}

type galaxyData struct {
	baseData
	Galaxy     int
	System     int
	PrevSystem int
	NextSystem int
	Rows       []galaxyRow
}

type reportsData struct {
	baseData
	Reports []reportRow
}

// ---- formatting helpers (also exposed to templates) ----

// fmtInt accepts any integer-ish value so templates can format int and int64
// fields with one function.
func fmtInt(v any) string {
	switch x := v.(type) {
	case int:
		return fmtInt64(int64(x))
	case int32:
		return fmtInt64(int64(x))
	case int64:
		return fmtInt64(x)
	case float64:
		return fmtInt64(int64(x + 0.5))
	case float32:
		return fmtInt64(int64(x + 0.5))
	default:
		return fmt.Sprint(v)
	}
}

func fmtInt64(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func fmtFloat(f float64) string {
	return fmtInt64(int64(f + 0.5))
}

func coords(g, s, p int) string {
	return strconv.Itoa(g) + ":" + strconv.Itoa(s) + ":" + strconv.Itoa(p)
}

func until(t time.Time) string {
	d := time.Until(t)
	if d <= 0 {
		return "now"
	}
	return humanDur(int64(d.Seconds()))
}

func humanDur(secs int64) string {
	if secs < 0 {
		secs = 0
	}
	d := secs / 86400
	h := (secs % 86400) / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	case h > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func costString(c game.Resources) string {
	parts := []string{}
	add := func(v float64, label string) {
		if v > 0 {
			parts = append(parts, fmtFloat(v)+" "+label)
		}
	}
	add(c.Metal, "Fe")
	add(c.Lutinum, "Lu")
	add(c.Water, "Wa")
	add(c.Hydrogen, "Hy")
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " · ")
}
