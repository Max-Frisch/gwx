package web

import (
	"fmt"
	"math"
	"net/url"
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

	// HX marks a response as an htmx fragment (not a full page): content
	// templates use it to render their own flash/error banner inside the
	// swapped region and to emit out-of-band updates.
	HX bool

	// Account-lifecycle notices shown as a banner on every page.
	OnVacation   bool
	Sanctioned   bool
	SanctionNote string
}

type resView struct {
	Metal, Lutinum, Water, Hydrogen                 float64
	MetalCap, LutinumCap, WaterCap, HydrogenCap     int64
	MetalProd, LutinumProd, WaterProd, HydrogenProd float64
	WaterUse                                        float64
}

func newResView(p *store.Planet) *resView {
	return &resView{
		Metal: p.Metal, Lutinum: p.Lutinum, Water: p.Water, Hydrogen: p.Hydrogen,
		MetalCap: p.MetalCap, LutinumCap: p.LutinumCap, WaterCap: p.WaterCap, HydrogenCap: p.HydrogenCap,
		MetalProd: p.MetalProd, LutinumProd: p.LutinumProd, WaterProd: p.WaterProd, HydrogenProd: p.HydrogenProd,
		WaterUse: p.WaterUse,
	}
}

type queueView struct {
	ID        int64
	Kind      string
	Name      string
	Remaining string
	// FinishUnix is the completion time in unix seconds, used by the
	// client-side countdown ticker (the server text is the fallback).
	FinishUnix int64
}

func newQueueView(q store.Queue) queueView {
	name := q.Code
	if d, ok := game.Get(q.Code); ok {
		name = d.Name
		if q.TargetLevel.Valid {
			name = fmt.Sprintf("%s → %d", d.Name, q.TargetLevel.Int64)
		}
	}
	return queueView{ID: q.ID, Kind: q.Kind, Name: name, Remaining: until(q.FinishAt), FinishUnix: q.FinishAt.Unix()}
}

// queuePartialData backs the self-polling queue panel fragment.
type queuePartialData struct {
	ActivePlanetID int64
	Queue          *queueView
}

// buildQueueRow is one line of the construction queue table.
type buildQueueRow struct {
	Ordinal    int
	ID         int64
	Name       string
	Duration   string
	Remaining  string
	FinishUnix int64
}

// buildQueuePartialData backs the self-polling construction queue fragment.
type buildQueuePartialData struct {
	ActivePlanetID int64
	BuildQueue     []buildQueueRow
	QueueUsed      int
	QueueSlots     int
	QueueFull      bool
}

type buildRow struct {
	Code        string
	Name        string
	Level       int
	NextLevel   int
	Queued      bool
	QueuedCount int
	Unlocked    bool
	Missing     []string
	Affordable  bool
	CostStr     string
	TimeStr     string
	Desc        string
	Icon        string
	// Art is an optional image tile for the card. Empty today (the catalog
	// carries SVG icons only), so the template falls back to the placeholder.
	Art   string
	Costs []costPart
}

// costPart is one resource of an item's price: its label, the CSS class that
// tints the label, the formatted amount and whether the planet can cover it.
type costPart struct {
	Label  string
	Class  string
	Amount string
	OK     bool
}

type unitRow struct {
	Code          string
	Name          string
	Count         int64
	Unlocked      bool
	Missing       []string
	CostStr       string
	Costs         []costPart
	Stats         string
	PerTimeStr    string
	MaxAffordable int64
	Desc          string
	Icon          string
	Kind          string
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
	OwnerTag    string
	IsHomeworld bool
	IsNPC       bool
	CanSpy      bool
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
	Tag            string
	PlanetPoints   int64
	ResearchPoints int64
	Total          int64
	Planets        int
	IsMe           bool
	IsNPC          bool
}

type planetScoreRow struct {
	Rank       int
	Name       string
	Coords     string
	OwnerID    int64
	OwnerName  string
	OwnerTag   string
	OwnerIsNPC bool
	Points     int64
}

type allianceScoreRow struct {
	Rank    int
	ID      int64
	Tag     string
	Name    string
	Points  int64
	Members int
	IsOpen  bool
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
	Alliances  []allianceScoreRow
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
	Tag            string
	AllianceID     int64
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

	// Vacation status (the lifecycle flags themselves live in baseData so the
	// banner on every page can read them).
	VacationSince  string
	VacationUntil  string
	VacationReady  bool
	FleetsInFlight bool
}

type authData struct {
	baseData
	Login string
	Email string
}

// ---- alliances ----

// allianceDetail is the public/dashboard view of one alliance.
type allianceDetail struct {
	ID          int64
	Tag         string
	Name        string
	Description string
	FounderID   int64
	FounderName string
	IsOpen      bool
	MinPoints   int
	Points      int64
	Members     int
	CreatedAt   string
}

type allianceListRow struct {
	ID      int64
	Tag     string
	Name    string
	Points  int64
	Members int
	IsOpen  bool
}

type allianceMemberRow struct {
	UserID    int64
	Username  string
	Rank      string
	Joined    string
	IsFounder bool
	IsMe      bool
}

type allianceApplicationRow struct {
	ID         int64
	AllianceID int64
	Tag        string
	Name       string
	Username   string
	UserID     int64
	Message    string
	When       string
}

type alliancePostRow struct {
	ID         int64
	AuthorID   int64
	AuthorName string
	Title      string
	Body       string
	IsPinned   bool
	When       string
	CanDelete  bool
}

// allianceDiplomacyRow is one NAP/war relationship relative to the viewer's
// alliance (Outgoing = we are the proposer).
type allianceDiplomacyRow struct {
	ID          int64
	Kind        string
	Status      string
	Outgoing    bool
	OtherID     int64
	OtherTag    string
	OtherName   string
	Message     string
	When        string
	Decided     string
	CanRespond  bool
	CanWithdraw bool
	CanEnd      bool
}

// allianceData backs both the public hub and the internal member area.
type allianceData struct {
	baseData
	Tab      string
	Query    string
	Page     int
	Pages    int
	PrevPage int
	NextPage int
	Total    int

	// browse
	Alliances []allianceListRow
	MyApps    []allianceApplicationRow

	// membership context (internal pages)
	InAlliance            bool
	IsMyAlliance          bool
	Alliance              *allianceDetail
	MyRank                string
	CanManageApplications bool
	CanModerateBoard      bool
	CanManageAlliance     bool

	// my own application to the alliance currently being viewed
	Applied bool
	AppID   int64

	// pending applications (internal)
	Pending []allianceApplicationRow

	// members
	Members []allianceMemberRow

	// diplomacy
	Diplomacy          []allianceDiplomacyRow
	Targets            []allianceListRow
	CanManageDiplomacy bool

	// board
	Posts []alliancePostRow
}

// browsePageSize is the alliance directory page length (reference: 20).
const browsePageSize = 20

// PageURL links to another directory page while keeping the search term.
func (d allianceData) PageURL(n int) string {
	u := "/alliance?page=" + strconv.Itoa(n)
	if d.Query != "" {
		u += "&q=" + url.QueryEscape(d.Query)
	}
	return u
}

type overviewData struct {
	baseData
	Planet   *store.Planet
	Queues   []queueView
	Incoming []fleetRow
}

// buildingSection groups the building cards under a category heading.
type buildingSection struct {
	Name string
	Rows []buildRow
}

type buildingsData struct {
	baseData
	Sections   []buildingSection
	BuildQueue []buildQueueRow
	QueueUsed  int
	QueueSlots int
	QueueFull  bool
}

// resProdRow is one line of the resources overview table. The four resource
// cells are pre-formatted (signed for production, plain for capacity) so the
// template stays logic-free.
type resProdRow struct {
	Name                            string
	Metal, Lutinum, Water, Hydrogen string
}

type resourcesData struct {
	baseData
	Rows      []resProdRow
	Total     resProdRow
	Storage   resProdRow
	Protected resProdRow
}

type researchData struct {
	baseData
	Sections []buildingSection
	Queue    *queueView
}

type shipyardData struct {
	baseData
	Ships []unitRow
	Queue *queueView
}

type defenseData struct {
	baseData
	Units []unitRow
	Queue *queueView
}

type fleetData struct {
	baseData
	Ships    []shipOption
	Active   []fleetRow
	Incoming []fleetRow
	OriginG  int
	OriginS  int
}

// simulatorRow is one editable unit line in the battle simulator: the count
// entered for each side plus the unit's combat stats.
type simulatorRow struct {
	Code     string
	Name     string
	Kind     string
	Icon     string
	Stats    string
	Attacker int64
	Defender int64
}

// simulatorOutcomeRow is one unit's result in the deterministic sample battle.
type simulatorOutcomeRow struct {
	Name             string
	AttackerBefore   int64
	AttackerLost     int64
	AttackerSurvived int64
	DefenderBefore   int64
	DefenderLost     int64
	DefenderSurvived int64
}

// simulatorResult is the sampled battle plus the stochastic win estimate.
type simulatorResult struct {
	WinPct           string
	Trials           int
	Rounds           int
	AttackerSurvived int64
	DefenderSurvived int64
	Rows             []simulatorOutcomeRow
}

type simulatorData struct {
	baseData
	Rows   []simulatorRow
	Result *simulatorResult
}

type galaxyData struct {
	baseData
	Galaxy     int
	System     int
	PrevSystem int
	NextSystem int
	Rows       []galaxyRow
	// SpyCount is the player's configured quick-dispatch probe count
	// (settings.spyShortcutCount); 0 disables the one-click spy action.
	SpyCount int
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
	case "defense":
		return "/defense/" + id
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

// fmtAmount renders a resource amount with two decimals and thousands
// separators (e.g. "18,810.24"). Used on the resources overview.
func fmtAmount(f float64) string {
	neg := f < 0
	if neg {
		f = -f
	}
	cents := int64(math.Round(f * 100))
	s := fmtInt64(cents/100) + fmt.Sprintf(".%02d", cents%100)
	if neg && cents != 0 {
		return "-" + s
	}
	return s
}

// fmtSigned is fmtAmount with an explicit "+" for positive output. Zero stays
// unsigned.
func fmtSigned(f float64) string {
	s := fmtAmount(f)
	if f > 0 {
		return "+" + s
	}
	return s
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

// untilDur formats a fixed duration between two instants.
func untilDur(start, finish time.Time) string {
	d := finish.Sub(start)
	if d <= 0 {
		return "0s"
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

// costParts splits a price into its non-zero resources with the planet's
// affordability so the card can tint each value green (ok) or red (short).
func costParts(c game.Resources, p *store.Planet) []costPart {
	var parts []costPart
	add := func(v, have float64, label, class string) {
		if v <= 0 {
			return
		}
		parts = append(parts, costPart{Label: label, Class: class, Amount: fmtFloat(v), OK: have >= v})
	}
	add(c.Metal, p.Metal, "Iron", "m")
	add(c.Lutinum, p.Lutinum, "Lutinum", "l")
	add(c.Water, p.Water, "Water", "w")
	add(c.Hydrogen, p.Hydrogen, "Hydrogen", "h")
	return parts
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

// ---- trading ----

// resourceOption is one entry of a resource <select>.
type resourceOption struct {
	Key  string
	Name string
}

// tradeOfferRow is the display shape of one marketplace/own offer.
type tradeOfferRow struct {
	ID          int64
	CreatorID   int64
	CreatorName string
	Planet      string
	Private     bool
	TargetName  string
	GiveStr     string
	WantStr     string
	Status      string
	Expires     string
	Created     string
	IsMine      bool
	CanAccept   bool
}

// tradeHistoryRow is one audit line in the trade history.
type tradeHistoryRow struct {
	When           string
	Action         string
	Actor          string
	ActorID        int64
	Counterparty   string
	CounterpartyID int64
	GiveStr        string
	WantStr        string
}

// tradeStorageEntryRow is one pending delivery shown under a planet.
type tradeStorageEntryRow struct {
	GiveStr string
	Expires string
}

// tradeStoragePlanet groups one planet's pending deliveries.
type tradeStoragePlanet struct {
	PlanetID int64
	Name     string
	Coords   string
	Total    string
	Entries  []tradeStorageEntryRow
}

// tradeData backs every trade tab.
type tradeData struct {
	baseData
	Tab string

	// marketplace
	Offered   string
	Requested string
	Offers    []tradeOfferRow
	Page      int
	Pages     int
	PrevPage  int
	NextPage  int
	Total     int

	// create form / offer detail
	Planets  []store.OwnedPlanet
	Target   string
	TargetID int64
	Offer    *tradeOfferRow

	// own / history / storage
	MyOffers []tradeOfferRow
	History  []tradeHistoryRow
	Storage  []tradeStoragePlanet

	OfferedOptions []resourceOption
}

// PageURL links to another marketplace page while keeping the filters.
func (d tradeData) PageURL(n int) string {
	u := "/trade?page=" + strconv.Itoa(n)
	if d.Offered != "" {
		u += "&offered=" + url.QueryEscape(d.Offered)
	}
	if d.Requested != "" {
		u += "&requested=" + url.QueryEscape(d.Requested)
	}
	return u
}
