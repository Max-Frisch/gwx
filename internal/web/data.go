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
	NavRight       bool
	MessagesUnread int
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
	OwnerID     int64
	OwnerName   string
	IsHomeworld bool
}

type reportRow struct {
	ID    int64
	Kind  string
	Title string
	When  string
	Body  string
}

type playerScoreRow struct {
	Rank           int
	UserID         int64
	Username       string
	PlanetPoints   int64
	ResearchPoints int64
	Total          int64
	Planets        int
	IsMe           bool
}

type planetScoreRow struct {
	Rank      int
	Name      string
	Coords    string
	OwnerID   int64
	OwnerName string
	Points    int64
}

type battleRow struct {
	Rank      int
	When      string
	Attacker  string
	Defender  string
	Coords    string
	Rounds    int
	Destroyed int64
	Plundered int64
}

// highscorePageSize is the reference leaderboard page length (100 rows).
const highscorePageSize = 100

type highscoreData struct {
	baseData
	Tab        string
	Sort       string
	Direction  string
	Players    []playerScoreRow
	PlanetRows []planetScoreRow
	Battles    []battleRow
	Page       int
	Pages      int
	PrevPage   int
	NextPage   int
}

// SortURL is the header link for a sort key: clicking the active key flips the
// direction, any other key picks a sensible default (names ascending,
// scores descending).
func (d highscoreData) SortURL(key string) string {
	dir := "desc"
	if key == "name" || key == "owner" {
		dir = "asc"
	}
	if d.Sort == key {
		if d.Direction == "asc" {
			dir = "desc"
		} else {
			dir = "asc"
		}
	}
	return "/highscore?tab=" + d.Tab + "&sort=" + key + "&direction=" + dir
}

// SortMark marks the active sort key with an arrow.
func (d highscoreData) SortMark(key string) string {
	if d.Sort != key {
		return ""
	}
	if d.Direction == "asc" {
		return " ▲"
	}
	return " ▼"
}

// PageURL links to another page while preserving the tab and sort order.
func (d highscoreData) PageURL(n int) string {
	u := "/highscore?tab=" + d.Tab
	if d.Sort != "" {
		u += "&sort=" + d.Sort + "&direction=" + d.Direction
	}
	return u + "&page=" + strconv.Itoa(n)
}

type statisticsData struct {
	baseData
	Tab             string
	Battle          game.BattleBalance
	Farming         game.Farming
	ResourceBalance game.ResourceBalance
}

type planetSearchRow struct {
	Name      string
	Coords    string
	OwnerID   int64
	OwnerName string
}

type searchData struct {
	baseData
	Query      string
	Players    []playerScoreRow
	PlanetHits []planetSearchRow
}

type playerPlanetRow struct {
	Name   string
	Coords string
	Points int64
	IsHome bool
}

type playerData struct {
	baseData
	PlayerID       int64
	Username       string
	Rank           int
	PlanetPoints   int64
	ResearchPoints int64
	TotalPoints    int64
	IsMe           bool
	PlanetRows     []playerPlanetRow
}

type settingsData struct {
	baseData
	Tab                string
	Username           string
	Email              string
	NavigationPosition string
	Locale             string
	MessagesPerPage    int
	SpyShortcutCount   int
	SpyLimitOnSpy      int
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

type messageRow struct {
	ID       int64
	Category string
	Kind     string
	Title    string
	Body     string
	When     string
	IsRead   bool
	ReportID int64
}

type messagesData struct {
	baseData
	Filter   string
	Page     int
	Pages    int
	PrevPage int
	NextPage int
	Total    int
	Rows     []messageRow
}

// eventRow is one line of the events timeline: an in-progress queue or a
// fleet movement (upcoming), or a recently resolved item (recent).
type eventRow struct {
	Kind      string
	Title     string
	Where     string
	Status    string
	Remaining string
	Link      string
	Incoming  bool
	When      time.Time
}

type eventsData struct {
	baseData
	Rows   []eventRow
	Recent []eventRow
}

// queueEventName renders a queue's human title: "Iron Mine → 3" for
// buildings/research and "5 × Spy Probe" for unit batches.
func queueEventName(q store.Queue) string {
	def, ok := game.Get(q.Code)
	if !ok {
		return q.Code
	}
	if q.Quantity.Valid && q.Quantity.Int64 > 0 {
		return fmt.Sprintf("%d × %s", q.Quantity.Int64, def.Name)
	}
	if q.TargetLevel.Valid {
		return fmt.Sprintf("%s → %d", def.Name, q.TargetLevel.Int64)
	}
	return def.Name
}

// missionLabel maps an engine mission code to a display name.
func missionLabel(mission string) string {
	switch mission {
	case "TRANSPORT":
		return "Transport"
	case "DEPLOY":
		return "Deploy"
	case "ATTACK":
		return "Attack"
	case "ESPIONAGE":
		return "Espionage"
	case "COLONIZE":
		return "Colonize"
	default:
		return mission
	}
}

// queueLink returns the page a queue's item is managed from.
func queueLink(q store.Queue) string {
	id := strconv.FormatInt(q.PlanetID, 10)
	switch q.Kind {
	case "building":
		return "/buildings/" + id
	case "research":
		return "/research/" + id
	default:
		return "/shipyard/" + id
	}
}

// ago renders a past timestamp as a relative "… ago" string.
func ago(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	return humanDur(int64(d.Seconds())) + " ago"
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
