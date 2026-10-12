// Package web serves the server-rendered UI using the standard library
// html/template plus a hand-written stylesheet. Functional over fancy.
package web

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gwx/internal/auth"
	"gwx/internal/game"
	"gwx/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

const sessionTTL = 7 * 24 * time.Hour

// Handler serves the UI.
type Handler struct {
	db           *sql.DB
	universeCode string
	planets      *store.PlanetStore
	queues       *store.QueueStore
	fleets       *store.FleetStore
	reports      *store.ReportStore
	messages     *store.MessageStore
	auth         *store.AuthStore
	scores       *store.ScoreStore
	stats        *store.StatsStore
	alliances    *store.AllianceStore
	trade        *store.TradeStore
	tmpl         *template.Template
}

// New builds the handler and parses templates.
func New(db *sql.DB, universeCode string) *Handler {
	fm := template.FuncMap{
		"n":      fmtInt,
		"nf":     fmtFloat,
		"coords": coords,
	}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))
	return &Handler{
		db:           db,
		universeCode: universeCode,
		planets:      store.NewPlanetStore(db),
		queues:       store.NewQueueStore(db),
		fleets:       store.NewFleetStore(db),
		reports:      store.NewReportStore(db),
		messages:     store.NewMessageStore(db),
		auth:         store.NewAuthStore(db),
		scores:       store.NewScoreStore(db),
		stats:        store.NewStatsStore(db),
		alliances:    store.NewAllianceStore(db),
		trade:        store.NewTradeStore(db),
		tmpl:         tmpl,
	}
}

// Mount registers all UI routes.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.Handle("GET /static/", http.FileServer(http.FS(mustSub(staticFS, "static"))))

	mux.HandleFunc("GET /{$}", h.Home)
	mux.HandleFunc("GET /login", h.LoginPage)
	mux.HandleFunc("POST /login", h.LoginSubmit)
	mux.HandleFunc("GET /register", h.RegisterPage)
	mux.HandleFunc("POST /register", h.RegisterSubmit)
	mux.HandleFunc("POST /logout", h.Logout)

	mux.HandleFunc("GET /overview/{id}", h.requireUser(h.OverviewPage))
	mux.HandleFunc("GET /buildings/{id}", h.requireUser(h.BuildingsPage))
	mux.HandleFunc("POST /buildings/{id}/build", h.requireUser(h.BuildSubmit))
	mux.HandleFunc("GET /research/{id}", h.requireUser(h.ResearchPage))
	mux.HandleFunc("POST /research/{id}/start", h.requireUser(h.ResearchSubmit))
	mux.HandleFunc("GET /shipyard/{id}", h.requireUser(h.ShipyardPage))
	mux.HandleFunc("POST /shipyard/{id}/build", h.requireUser(h.ShipyardSubmit))
	mux.HandleFunc("GET /fleet/{id}", h.requireUser(h.FleetPage))
	mux.HandleFunc("POST /fleet/{id}/dispatch", h.requireUser(h.FleetDispatchSubmit))
	mux.HandleFunc("POST /fleet/{id}/recall", h.requireUser(h.FleetRecallSubmit))
	mux.HandleFunc("GET /galaxy", h.requireUser(h.GalaxyRedirect))
	mux.HandleFunc("GET /galaxy/{galaxy}/{system}", h.requireUser(h.GalaxyPage))
	mux.HandleFunc("GET /reports", h.requireUser(h.ReportsPage))
	mux.HandleFunc("GET /events", h.requireUser(h.EventsPage))
	mux.HandleFunc("GET /messages", h.requireUser(h.MessagesPage))
	mux.HandleFunc("POST /messages/{id}/read", h.requireUser(h.MessageReadSubmit))
	mux.HandleFunc("POST /messages/{id}/delete", h.requireUser(h.MessageDeleteSubmit))
	mux.HandleFunc("POST /messages/{id}/archive", h.requireUser(h.MessageArchiveSubmit))
	mux.HandleFunc("GET /highscore", h.requireUser(h.HighscorePage))
	mux.HandleFunc("GET /statistics", h.requireUser(h.StatisticsPage))
	mux.HandleFunc("GET /search", h.requireUser(h.SearchPage))
	mux.HandleFunc("GET /player/{id}", h.requireUser(h.PlayerPage))
	mux.HandleFunc("GET /settings", h.requireUser(h.SettingsRedirect))
	mux.HandleFunc("GET /settings/account", h.requireUser(h.SettingsAccountPage))
	mux.HandleFunc("POST /settings/account", h.requireUser(h.SettingsAccountSubmit))
	mux.HandleFunc("GET /settings/game", h.requireUser(h.SettingsGamePage))
	mux.HandleFunc("POST /settings/game", h.requireUser(h.SettingsGameSubmit))
	mux.HandleFunc("POST /queue/{qid}/cancel", h.requireUser(h.QueueCancelSubmit))

	mux.HandleFunc("GET /alliance", h.requireUser(h.AlliancePage))
	mux.HandleFunc("GET /alliance/create", h.requireUser(h.AllianceCreatePage))
	mux.HandleFunc("POST /alliance/create", h.requireUser(h.AllianceCreateSubmit))
	mux.HandleFunc("GET /alliance/applications", h.requireUser(h.AllianceApplicationsPage))
	mux.HandleFunc("POST /alliance/applications/{id}/withdraw", h.requireUser(h.AllianceWithdrawSubmit))
	mux.HandleFunc("GET /alliance/internal", h.requireUser(h.AllianceInternalPage))
	mux.HandleFunc("GET /alliance/internal/members", h.requireUser(h.AllianceMembersPage))
	mux.HandleFunc("POST /alliance/internal/leave", h.requireUser(h.AllianceLeaveSubmit))
	mux.HandleFunc("POST /alliance/internal/members/{id}/rank", h.requireUser(h.AllianceRankSubmit))
	mux.HandleFunc("GET /alliance/internal/board", h.requireUser(h.AllianceBoardPage))
	mux.HandleFunc("POST /alliance/internal/board", h.requireUser(h.AllianceBoardSubmit))
	mux.HandleFunc("POST /alliance/internal/board/{id}/delete", h.requireUser(h.AlliancePostDeleteSubmit))
	mux.HandleFunc("GET /alliance/internal/applications", h.requireUser(h.AllianceApplicationsInternalPage))
	mux.HandleFunc("POST /alliance/internal/applications/{id}/decide", h.requireUser(h.AllianceApplicationDecideSubmit))
	mux.HandleFunc("GET /alliance/internal/settings", h.requireUser(h.AllianceSettingsPage))
	mux.HandleFunc("POST /alliance/internal/settings", h.requireUser(h.AllianceSettingsSubmit))
	mux.HandleFunc("POST /alliance/internal/close", h.requireUser(h.AllianceCloseSubmit))
	mux.HandleFunc("POST /alliance/{id}/join", h.requireUser(h.AllianceJoinSubmit))
	mux.HandleFunc("POST /alliance/{id}/apply", h.requireUser(h.AllianceApplySubmit))
	mux.HandleFunc("GET /alliance/{id}", h.requireUser(h.AllianceProfilePage))

	mux.HandleFunc("GET /trade", h.requireUser(h.TradePage))
	mux.HandleFunc("GET /trade/new", h.requireUser(h.TradeNewPage))
	mux.HandleFunc("POST /trade/new", h.requireUser(h.TradeCreateSubmit))
	mux.HandleFunc("GET /trade/own", h.requireUser(h.TradeOwnPage))
	mux.HandleFunc("GET /trade/history", h.requireUser(h.TradeHistoryPage))
	mux.HandleFunc("GET /trade/storage", h.requireUser(h.TradeStoragePage))
	mux.HandleFunc("POST /trade/storage/credit", h.requireUser(h.TradeStorageCreditSubmit))
	mux.HandleFunc("GET /trade/offer/{id}", h.requireUser(h.TradeOfferPage))
	mux.HandleFunc("POST /trade/offer/{id}/accept", h.requireUser(h.TradeAcceptSubmit))
	mux.HandleFunc("POST /trade/offer/{id}/revoke", h.requireUser(h.TradeRevokeSubmit))
}

func mustSub(fsys embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		log.Fatalf("[WEB] static sub: %v", err)
	}
	return sub
}

// ---- session helpers ----

func (h *Handler) userID(r *http.Request) (int64, bool) {
	token, ok := auth.TokenFromRequest(r)
	if !ok {
		return 0, false
	}
	id, err := h.auth.SessionUser(r.Context(), auth.HashToken(token))
	if err != nil {
		return 0, false
	}
	return id, true
}

func (h *Handler) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := h.userID(r); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (h *Handler) secureCookies() bool {
	return strings.ToLower(getenv("APP_ENV", "development")) != "development"
}

func getenv(k, d string) string {
	if v := lookupEnv(k); v != "" {
		return v
	}
	return d
}

// ---- render helpers ----

func (h *Handler) render(w http.ResponseWriter, name string, status int, data any) {
	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("[WEB] render %s: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (h *Handler) base(ctx context.Context, r *http.Request, userID int64, title string, activeID int64) baseData {
	b := baseData{Title: title, ActivePlanetID: activeID}
	b.Username, _ = h.auth.Username(ctx, userID)
	b.Planets, _ = h.planets.ListOwned(ctx, userID)
	if activeID != 0 {
		if p, err := h.planets.Get(ctx, activeID); err == nil {
			b.Resources = newResView(p)
		}
	}
	b.Flash = r.URL.Query().Get("ok")
	b.Error = r.URL.Query().Get("err")
	if st, err := h.auth.Settings(ctx, userID); err == nil {
		b.NavRight = st.NavigationPosition == "right"
	}
	if n, err := h.messages.UnreadCount(ctx, userID); err == nil {
		b.MessagesUnread = n
	}
	return b
}

func (h *Handler) owned(ctx context.Context, userID, planetID int64) (*store.Planet, bool) {
	p, err := h.planets.Get(ctx, planetID)
	if err != nil || !p.UserID.Valid || p.UserID.Int64 != userID {
		return nil, false
	}
	return p, true
}

func (h *Handler) homeID(ctx context.Context, userID int64) int64 {
	if id, err := h.planets.HomeworldID(ctx, userID); err == nil && id != 0 {
		return id
	}
	if ps, err := h.planets.ListOwned(ctx, userID); err == nil && len(ps) > 0 {
		return ps[0].ID
	}
	return 0
}

func flashRedirect(w http.ResponseWriter, r *http.Request, to, okMsg, errMsg string) {
	if errMsg != "" {
		okMsg = ""
	}
	q := url.Values{}
	if okMsg != "" {
		q.Set("ok", okMsg)
	}
	if errMsg != "" {
		q.Set("err", errMsg)
	}
	if len(q) > 0 {
		sep := "?"
		if strings.Contains(to, "?") {
			sep = "&"
		}
		to += sep + q.Encode()
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func msg(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, store.ErrInsufficientResources):
		return "Not enough resources."
	case errors.Is(err, store.ErrInsufficientShips):
		return "Not enough ships stationed here."
	case errors.Is(err, store.ErrInsufficientFuel):
		return "Not enough hydrogen for fuel (and cargo)."
	case errors.Is(err, store.ErrQueueBusy):
		return "That queue is already busy."
	case errors.Is(err, store.ErrPrerequisites):
		return "Prerequisites not met."
	case errors.Is(err, store.ErrNoFields):
		return "No free fields on this planet."
	case errors.Is(err, store.ErrTargetOccupied):
		return "Those coordinates are already occupied."
	case errors.Is(err, store.ErrInvalidCoordinates):
		return "Coordinates out of range."
	case errors.Is(err, store.ErrNoTarget):
		return "Target planet does not exist."
	case errors.Is(err, store.ErrNotOwner):
		return "You do not own that planet."
	case errors.Is(err, store.ErrAllianceTagTaken):
		return "That alliance tag is already taken."
	case errors.Is(err, store.ErrAlreadyMember):
		return "You are already in an alliance."
	case errors.Is(err, store.ErrNotMember):
		return "You are not a member of that alliance."
	case errors.Is(err, store.ErrAllianceClosed):
		return "That alliance is closed — send an application instead."
	case errors.Is(err, store.ErrForbidden):
		return "You do not have the rights to do that."
	case errors.Is(err, store.ErrNoTradingPost):
		return "Build a Trading Post on that planet first."
	case errors.Is(err, store.ErrTradeSlotsFull):
		return "No free offer slots on that planet — upgrade its Trading Post."
	case errors.Is(err, store.ErrTradeNotOpen):
		return "That offer is no longer open."
	case errors.Is(err, store.ErrTradeOwn):
		return "You cannot accept your own offer."
	case errors.Is(err, store.ErrTradePrivate):
		return "That offer is directed at another player."
	case errors.Is(err, store.ErrUserExists):
		return "That username or email is already taken."
	case errors.Is(err, store.ErrUnknownCode):
		return "Unknown item."
	case errors.Is(err, store.ErrNotFound):
		return "Not found."
	default:
		return err.Error()
	}
}

// ---- auth handlers ----

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	home := h.homeID(r.Context(), userID)
	if home == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/overview/"+strconv.FormatInt(home, 10), http.StatusSeeOther)
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "login", http.StatusOK, authData{baseData: baseData{Title: "Log in"}})
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "register", http.StatusOK, authData{baseData: baseData{Title: "Register"}})
}

func (h *Handler) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	login := strings.TrimSpace(r.FormValue("login"))
	password := r.FormValue("password")
	userID, hash, err := h.auth.UserAuth(r.Context(), login)
	if err != nil || !auth.VerifyPassword(password, hash) {
		d := authData{baseData: baseData{Title: "Log in", Error: "Invalid username or password"}, Login: login}
		h.render(w, "login", http.StatusUnauthorized, d)
		return
	}
	h.issueSession(w, r, userID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) RegisterSubmit(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	fail := func(m string) {
		h.render(w, "register", http.StatusBadRequest,
			authData{baseData: baseData{Title: "Register", Error: m}, Login: username, Email: email})
	}
	if m := validateUsername(username); m != "" {
		fail(m)
		return
	}
	if m := validateEmail(email); m != "" {
		fail(m)
		return
	}
	if m := validatePassword(password); m != "" {
		fail(m)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		fail("Could not secure password")
		return
	}
	userID, _, err := h.auth.CreateUser(r.Context(), h.universeCode, username, email, hash)
	if err != nil {
		if errors.Is(err, store.ErrUserExists) {
			fail("Username or email already registered")
			return
		}
		log.Printf("[WEB] register: %v", err)
		fail("Could not create account")
		return
	}
	h.issueSession(w, r, userID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if token, ok := auth.TokenFromRequest(r); ok {
		_ = h.auth.DeleteSession(r.Context(), auth.HashToken(token))
	}
	auth.ClearSessionCookie(w, h.secureCookies())
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) issueSession(w http.ResponseWriter, r *http.Request, userID int64) {
	raw, hash, err := auth.NewSessionToken()
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	_ = h.auth.CreateSession(r.Context(), userID, hash, time.Now().Add(sessionTTL), r.UserAgent(), ip)
	auth.SetSessionCookie(w, raw, int(sessionTTL.Seconds()), h.secureCookies())
}

// ---- page handlers ----

func (h *Handler) OverviewPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id := pathID(r)
	p, ok := h.owned(r.Context(), userID, id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	d := overviewData{baseData: h.base(r.Context(), r, userID, "Overview", id), Planet: p}

	qs, _ := h.queues.Active(r.Context(), id, userID)
	for _, q := range qs {
		d.Queues = append(d.Queues, newQueueView(q))
	}
	incoming, _ := h.fleets.Incoming(r.Context(), id)
	for _, f := range incoming {
		d.Incoming = append(d.Incoming, fleetRow{
			Mission: f.Mission, From: coords(f.OriginG, f.OriginS, f.OriginP), Remaining: until(f.ArrivalAt),
		})
	}
	h.render(w, "overview", http.StatusOK, d)
}

func (h *Handler) BuildingsPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id := pathID(r)
	p, ok := h.owned(r.Context(), userID, id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	structs, _ := h.planets.Structures(ctx, id)
	techs, _ := h.planets.Techs(ctx, userID)
	gameSpeed, _ := h.gameSpeed(ctx)

	d := buildingsData{baseData: h.base(ctx, r, userID, "Buildings", id)}
	if q, ok := h.activeQueue(ctx, id, userID, "building"); ok {
		v := newQueueView(q)
		d.Queue = &v
	}

	for _, def := range sortedDefs(game.KindBuilding) {
		next := structs[def.Code] + 1
		cost := def.LevelCost(next)
		row := buildRow{
			Code: def.Code, Name: def.Name, Level: structs[def.Code],
			Unlocked:   game.PrereqsMet(def, structs, techs),
			Queued:     d.Queue != nil && strings.HasPrefix(d.Queue.Name, def.Name),
			CostStr:    costString(cost),
			TimeStr:    humanDur(int64(def.LevelTime(next) * game.BuildTimeMultiplier(structs["command_center"]) / gameSpeed)),
			Affordable: p.Metal >= cost.Metal && p.Lutinum >= cost.Lutinum && p.Water >= cost.Water && p.Hydrogen >= cost.Hydrogen,
		}
		if !row.Unlocked {
			row.Missing = game.MissingPrereqs(def, structs, techs)
		}
		d.Rows = append(d.Rows, row)
	}
	h.render(w, "buildings", http.StatusOK, d)
}

func (h *Handler) ResearchPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id := pathID(r)
	p, ok := h.owned(r.Context(), userID, id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	structs, _ := h.planets.Structures(ctx, id)
	techs, _ := h.planets.Techs(ctx, userID)
	gameSpeed, _ := h.gameSpeed(ctx)

	d := researchData{baseData: h.base(ctx, r, userID, "Research", id)}
	if q, ok := h.activeQueue(ctx, id, userID, "research"); ok {
		v := newQueueView(q)
		d.Queue = &v
	}
	for _, def := range sortedDefs(game.KindResearch) {
		next := techs[def.Code] + 1
		cost := def.LevelCost(next)
		row := buildRow{
			Code: def.Code, Name: def.Name, Level: techs[def.Code],
			Unlocked:   game.PrereqsMet(def, structs, techs),
			Queued:     d.Queue != nil && strings.HasPrefix(d.Queue.Name, def.Name),
			CostStr:    costString(cost),
			TimeStr:    humanDur(int64(def.LevelTime(next) / gameSpeed)),
			Affordable: p.Metal >= cost.Metal && p.Lutinum >= cost.Lutinum && p.Water >= cost.Water && p.Hydrogen >= cost.Hydrogen,
		}
		if !row.Unlocked {
			row.Missing = game.MissingPrereqs(def, structs, techs)
		}
		d.Rows = append(d.Rows, row)
	}
	h.render(w, "research", http.StatusOK, d)
}

func (h *Handler) ShipyardPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id := pathID(r)
	p, ok := h.owned(r.Context(), userID, id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	structs, _ := h.planets.Structures(ctx, id)
	techs, _ := h.planets.Techs(ctx, userID)
	gameSpeed, _ := h.gameSpeed(ctx)

	d := shipyardData{baseData: h.base(ctx, r, userID, "Shipyard", id)}
	if q, ok := h.activeQueue(ctx, id, userID, "ship"); ok {
		v := newQueueView(q)
		d.Queue = &v
	} else if q, ok := h.activeQueue(ctx, id, userID, "defense"); ok {
		v := newQueueView(q)
		d.Queue = &v
	}
	d.Ships = h.unitRows(ctx, p, structs, techs, game.KindShip, gameSpeed)
	d.Defense = h.unitRows(ctx, p, structs, techs, game.KindDefense, gameSpeed)
	h.render(w, "shipyard", http.StatusOK, d)
}

func (h *Handler) unitRows(ctx context.Context, p *store.Planet, structs, techs map[string]int, kind game.Kind, gameSpeed float64) []unitRow {
	owned, _ := h.planets.Units(ctx, p.ID, string(kind))
	var rows []unitRow
	for _, def := range sortedDefs(kind) {
		row := unitRow{
			Code: def.Code, Name: def.Name, Count: owned[def.Code],
			Unlocked:   game.PrereqsMet(def, structs, techs),
			CostStr:    costString(def.Cost),
			PerTimeStr: humanDur(int64(def.TimeSecs / gameSpeed)),
		}
		if !row.Unlocked {
			row.Missing = game.MissingPrereqs(def, structs, techs)
		} else {
			row.MaxAffordable = maxAffordable(p, def.Cost)
		}
		rows = append(rows, row)
	}
	return rows
}

func (h *Handler) FleetPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id := pathID(r)
	p, ok := h.owned(r.Context(), userID, id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	owned, _ := h.planets.Units(ctx, id, "ship")
	var opts []shipOption
	for code, n := range owned {
		name := code
		if d, ok := game.Get(code); ok {
			name = d.Name
		}
		opts = append(opts, shipOption{Code: code, Name: name, Count: n})
	}
	sort.Slice(opts, func(i, j int) bool { return opts[i].Name < opts[j].Name })

	d := fleetData{
		baseData: h.base(ctx, r, userID, "Fleet", id),
		Ships:    opts,
		OriginG:  p.Galaxy,
		OriginS:  p.System,
	}
	active, _ := h.fleets.ListActive(ctx, userID)
	for _, f := range active {
		d.Active = append(d.Active, fleetRow{
			ID: f.ID, Mission: f.Mission, Phase: f.Phase,
			From: coords(f.OriginG, f.OriginS, f.OriginP), To: coords(f.TargetG, f.TargetS, f.TargetP),
			Remaining: until(f.ArrivalAt), CanRecall: f.Phase == "OUTBOUND",
		})
	}
	incoming, _ := h.fleets.Incoming(ctx, id)
	for _, f := range incoming {
		d.Incoming = append(d.Incoming, fleetRow{
			Mission: f.Mission, From: coords(f.OriginG, f.OriginS, f.OriginP), Remaining: until(f.ArrivalAt),
		})
	}
	h.render(w, "fleet", http.StatusOK, d)
}

func (h *Handler) GalaxyRedirect(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	home := h.homeID(r.Context(), userID)
	if home == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	p, err := h.planets.Get(r.Context(), home)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/galaxy/"+strconv.Itoa(p.Galaxy)+"/"+strconv.Itoa(p.System), http.StatusSeeOther)
}

func (h *Handler) GalaxyPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	g, _ := strconv.Atoi(r.PathValue("galaxy"))
	s, _ := strconv.Atoi(r.PathValue("system"))
	if g < 1 {
		g = 1
	}
	if s < 1 {
		s = 1
	}
	ctx := r.Context()
	_, _, _, maxG, maxS := h.universeSpeeds(ctx)
	if g > maxG {
		g = maxG
	}
	if s > maxS {
		s = maxS
	}

	d := galaxyData{
		baseData:   h.base(ctx, r, userID, "Galaxy", h.homeID(ctx, userID)),
		Galaxy:     g,
		System:     s,
		PrevSystem: maxInt(1, s-1),
		NextSystem: minInt(maxS, s+1),
	}
	rows, _ := h.planets.Scan(ctx, h.universeCode, g, s)
	tags, _ := h.alliances.TagsByUser(ctx, h.universeCode)
	byPos := map[int]store.ScanRow{}
	for _, row := range rows {
		byPos[row.Position] = row
	}
	for pos := 1; pos <= 15; pos++ {
		gr := galaxyRow{Position: pos}
		if row, ok := byPos[pos]; ok {
			if row.PlanetID.Valid {
				gr.PlanetID = row.PlanetID.Int64
			}
			if row.OwnerID.Valid {
				gr.OwnerID = row.OwnerID.Int64
				gr.OwnerTag = tags[row.OwnerID.Int64]
			}
			gr.Name = row.Name.String
			gr.OwnerName = row.OwnerName.String
			gr.IsHomeworld = row.IsHomeworld
		}
		d.Rows = append(d.Rows, gr)
	}
	h.render(w, "galaxy", http.StatusOK, d)
}

func (h *Handler) ReportsPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	d := reportsData{baseData: h.base(ctx, r, userID, "Reports", h.homeID(ctx, userID))}
	rs, _ := h.reports.List(ctx, userID, 50)
	for _, rep := range rs {
		var pretty bytes.Buffer
		_ = json.Indent(&pretty, rep.Payload, "", "  ")
		d.Reports = append(d.Reports, reportRow{
			ID:   rep.ID,
			Kind: rep.Kind, Title: rep.Title,
			When: rep.CreatedAt.Format("2006-01-02 15:04"),
			Body: pretty.String(),
		})
	}
	h.render(w, "reports", http.StatusOK, d)
}

// EventsPage renders the derived "activity timeline": running builds/research/
// unit orders, fleets in flight (own and incoming) and recent resolved items.
// No new state — everything is read from queues, fleets and messages.
func (h *Handler) EventsPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	d := eventsData{baseData: h.base(ctx, r, userID, "Events", h.homeID(ctx, userID))}

	owned, _ := h.planets.ListOwned(ctx, userID)
	byID := make(map[int64]store.OwnedPlanet, len(owned))
	for _, p := range owned {
		byID[p.ID] = p
	}
	where := func(id int64) string {
		if p, ok := byID[id]; ok {
			return coords(p.Galaxy, p.System, p.Position)
		}
		return ""
	}

	qs, _ := h.queues.ActiveForUser(ctx, userID)
	for _, q := range qs {
		d.Rows = append(d.Rows, eventRow{
			Kind:      q.Kind,
			Title:     queueEventName(q),
			Where:     where(q.PlanetID),
			Status:    "in progress",
			Remaining: until(q.FinishAt),
			Link:      queueLink(q),
			When:      q.FinishAt,
		})
	}

	active, _ := h.fleets.ListActive(ctx, userID)
	for _, f := range active {
		status := "arriving"
		if f.Phase == "RETURNING" {
			status = "returning"
		}
		d.Rows = append(d.Rows, eventRow{
			Kind:      "fleet",
			Title:     missionLabel(f.Mission),
			Where:     coords(f.TargetG, f.TargetS, f.TargetP),
			Status:    status,
			Remaining: until(f.ArrivalAt),
			Link:      "/fleet/" + strconv.FormatInt(f.OriginID, 10),
			When:      f.ArrivalAt,
		})
	}

	incoming, _ := h.fleets.IncomingForUser(ctx, userID)
	for _, f := range incoming {
		link := "/fleet/" + strconv.FormatInt(h.homeID(ctx, userID), 10)
		if f.TargetID.Valid {
			link = "/overview/" + strconv.FormatInt(f.TargetID.Int64, 10)
		}
		d.Rows = append(d.Rows, eventRow{
			Kind:      "fleet",
			Title:     missionLabel(f.Mission),
			Where:     coords(f.TargetG, f.TargetS, f.TargetP),
			Status:    "incoming",
			Remaining: until(f.ArrivalAt),
			Link:      link,
			Incoming:  true,
			When:      f.ArrivalAt,
		})
	}

	sort.SliceStable(d.Rows, func(i, j int) bool { return d.Rows[i].When.Before(d.Rows[j].When) })

	msgs, _ := h.messages.List(ctx, userID, 1, 10, store.MessageFilter{})
	for _, m := range msgs {
		link := "/messages"
		if m.ReportID.Valid {
			link = "/reports#report-" + strconv.FormatInt(m.ReportID.Int64, 10)
		}
		d.Recent = append(d.Recent, eventRow{
			Kind:      m.Kind,
			Title:     m.Title,
			Status:    "done",
			Remaining: ago(m.CreatedAt),
			Link:      link,
		})
	}

	h.render(w, "events", http.StatusOK, d)
}

func (h *Handler) MessagesPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()

	filter := r.URL.Query().Get("filter")
	switch filter {
	case "system", "report", "unread", "archive":
	default:
		filter = "all"
	}

	st, _ := h.auth.Settings(ctx, userID)
	perPage := st.MessagesPerPage
	if perPage <= 0 {
		perPage = 20
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	f := store.MessageFilter{}
	switch filter {
	case "system", "report":
		f.Category = filter
	case "unread":
		f.UnreadOnly = true
	case "archive":
		f.Archived = true
	}

	d := messagesData{baseData: h.base(ctx, r, userID, "Messages", h.homeID(ctx, userID)), Filter: filter, Page: page}
	total, _ := h.messages.Count(ctx, userID, f)
	d.Total = total
	d.Pages = (total + perPage - 1) / perPage
	if d.Pages < 1 {
		d.Pages = 1
	}
	if page > d.Pages {
		page = d.Pages
		d.Page = page
	}
	d.PrevPage = page - 1
	d.NextPage = page + 1
	msgs, _ := h.messages.List(ctx, userID, page, perPage, f)
	for _, m := range msgs {
		row := messageRow{
			ID: m.ID, Category: m.Category, Kind: m.Kind, Title: m.Title, Body: m.Body,
			When:   m.CreatedAt.Format("2006-01-02 15:04"),
			IsRead: m.IsRead,
		}
		if m.ReportID.Valid {
			row.ReportID = m.ReportID.Int64
		}
		d.Rows = append(d.Rows, row)
	}
	h.render(w, "messages", http.StatusOK, d)
}

func (h *Handler) MessageReadSubmit(w http.ResponseWriter, r *http.Request) {
	h.messageAction(w, r, func(userID, id int64) error { return h.messages.MarkRead(r.Context(), userID, id) }, "Marked as read.")
}

func (h *Handler) MessageDeleteSubmit(w http.ResponseWriter, r *http.Request) {
	h.messageAction(w, r, func(userID, id int64) error { return h.messages.Delete(r.Context(), userID, id) }, "Message deleted.")
}

func (h *Handler) MessageArchiveSubmit(w http.ResponseWriter, r *http.Request) {
	h.messageAction(w, r, func(userID, id int64) error { return h.messages.Archive(r.Context(), userID, id) }, "Message archived.")
}

func (h *Handler) messageAction(w http.ResponseWriter, r *http.Request, fn func(userID, id int64) error, okMsg string) {
	userID, _ := h.userID(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	to := r.Referer()
	if to == "" {
		to = "/messages"
	}
	flashRedirect(w, r, to, okMsg, msg(fn(userID, id)))
}

func (h *Handler) HighscorePage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	q := r.URL.Query()

	tab := q.Get("tab")
	switch tab {
	case "planets", "battles", "alliances":
	default:
		tab = "players"
	}

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	direction := q.Get("direction")
	if direction != "asc" {
		direction = "desc"
	}

	d := highscoreData{baseData: h.base(ctx, r, userID, "Highscore", h.homeID(ctx, userID)), Tab: tab, Direction: direction}
	tags, _ := h.alliances.TagsByUser(ctx, h.universeCode)

	switch tab {
	case "planets":
		d.Sort = planetSort(q.Get("sort"))
		rows, _ := h.scores.Planets(ctx, h.universeCode)
		sort.SliceStable(rows, func(i, j int) bool {
			return ordered(comparePlanets(rows[i], rows[j], d.Sort), direction)
		})
		start, end, pages, page := pageWindow(len(rows), page, highscorePageSize)
		d.Page, d.Pages, d.PrevPage, d.NextPage = page, pages, page-1, page+1
		for i := start; i < end; i++ {
			p := rows[i]
			d.PlanetRows = append(d.PlanetRows, planetScoreRow{
				Rank:      i + 1,
				Name:      p.Name,
				Coords:    coords(p.Galaxy, p.System, p.Position),
				OwnerID:   p.OwnerID,
				OwnerName: p.OwnerName,
				OwnerTag:  tags[p.OwnerID],
				Points:    int64(p.Points + 0.5),
			})
		}
	case "alliances":
		d.Sort = allianceSort(q.Get("sort"))
		rows, _ := h.scores.Alliances(ctx, h.universeCode)
		sort.SliceStable(rows, func(i, j int) bool {
			return ordered(compareAlliances(rows[i], rows[j], d.Sort), direction)
		})
		start, end, pages, page := pageWindow(len(rows), page, highscorePageSize)
		d.Page, d.Pages, d.PrevPage, d.NextPage = page, pages, page-1, page+1
		for i := start; i < end; i++ {
			a := rows[i]
			d.Alliances = append(d.Alliances, allianceScoreRow{
				Rank:    i + 1,
				ID:      a.ID,
				Tag:     a.Tag,
				Name:    a.Name,
				Points:  int64(a.Points + 0.5),
				Members: a.Members,
				IsOpen:  a.IsOpen,
			})
		}
	case "battles":
		d.Sort = battleSort(q.Get("sort"))
		rows, _ := h.scores.Battles(ctx, h.universeCode)
		sort.SliceStable(rows, func(i, j int) bool {
			return ordered(compareBattles(rows[i], rows[j], d.Sort), direction)
		})
		start, end, pages, page := pageWindow(len(rows), page, highscorePageSize)
		d.Page, d.Pages, d.PrevPage, d.NextPage = page, pages, page-1, page+1
		for i := start; i < end; i++ {
			b := rows[i]
			d.Battles = append(d.Battles, battleRow{
				Rank:      i + 1,
				When:      b.When.Format("2006-01-02 15:04"),
				Attacker:  b.Attacker,
				Defender:  b.Defender,
				Coords:    b.Coords,
				Rounds:    b.Rounds,
				Destroyed: int64(b.Destroyed + 0.5),
				Plundered: int64(b.Plundered + 0.5),
			})
		}
	default:
		d.Sort = playerSort(q.Get("sort"))
		rows, _ := h.scores.Players(ctx, h.universeCode)
		sort.SliceStable(rows, func(i, j int) bool {
			return ordered(comparePlayers(rows[i], rows[j], d.Sort), direction)
		})
		start, end, pages, page := pageWindow(len(rows), page, highscorePageSize)
		d.Page, d.Pages, d.PrevPage, d.NextPage = page, pages, page-1, page+1
		for i := start; i < end; i++ {
			p := rows[i]
			d.Players = append(d.Players, playerScoreRow{
				Rank:           i + 1,
				UserID:         p.UserID,
				Username:       p.Username,
				Tag:            tags[p.UserID],
				PlanetPoints:   int64(p.PlanetPoints + 0.5),
				ResearchPoints: int64(p.ResearchPoints + 0.5),
				Total:          int64(p.Total() + 0.5),
				Planets:        p.Planets,
				IsMe:           p.UserID == userID,
			})
		}
	}
	h.render(w, "highscore", http.StatusOK, d)
}

// playerSort normalises a Players-tab sort key, accepting the legacy short
// names so old bookmarks keep working.
func playerSort(key string) string {
	switch key {
	case "name", "planetCount":
		return key
	case "planet", "planetPoints":
		return "planetPoints"
	case "research", "researchPoints":
		return "researchPoints"
	case "planets", "allPoints", "total", "":
		return "allPoints"
	default:
		return "allPoints"
	}
}

// planetSort normalises a Planets-tab sort key.
func planetSort(key string) string {
	switch key {
	case "name", "owner":
		return key
	default:
		return "points"
	}
}

// battleSort normalises a Battles-tab sort key.
func battleSort(key string) string {
	switch key {
	case "date", "loot":
		return key
	default:
		return "destroyed"
	}
}

// allianceSort normalises an Alliances-tab sort key.
func allianceSort(key string) string {
	switch key {
	case "name", "members":
		return key
	default:
		return "points"
	}
}

func compareAlliances(a, b store.AllianceScore, key string) int {
	switch key {
	case "name":
		return strings.Compare(a.Name, b.Name)
	case "members":
		return cmpInt(int64(a.Members), int64(b.Members))
	default:
		return cmpFloat(a.Points, b.Points)
	}
}

func comparePlayers(a, b store.PlayerScore, key string) int {
	switch key {
	case "name":
		return strings.Compare(a.Username, b.Username)
	case "planetPoints":
		return cmpFloat(a.PlanetPoints, b.PlanetPoints)
	case "researchPoints":
		return cmpFloat(a.ResearchPoints, b.ResearchPoints)
	case "planetCount":
		return cmpInt(int64(a.Planets), int64(b.Planets))
	default:
		return cmpFloat(a.Total(), b.Total())
	}
}

func comparePlanets(a, b store.PlanetScore, key string) int {
	switch key {
	case "name":
		return strings.Compare(a.Name, b.Name)
	case "owner":
		return strings.Compare(a.OwnerName, b.OwnerName)
	default:
		return cmpFloat(a.Points, b.Points)
	}
}

func compareBattles(a, b store.BattleRow, key string) int {
	switch key {
	case "date":
		return cmpTime(a.When, b.When)
	case "loot":
		return cmpFloat(a.Plundered, b.Plundered)
	default:
		return cmpFloat(a.Destroyed, b.Destroyed)
	}
}

// ordered turns a compare result into a "before" predicate for the given
// direction (ascending when "asc", otherwise descending).
func ordered(comp int, direction string) bool {
	if direction == "asc" {
		return comp < 0
	}
	return comp > 0
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpInt(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpTime(a, b time.Time) int {
	switch {
	case a.Before(b):
		return -1
	case a.After(b):
		return 1
	default:
		return 0
	}
}

// pageWindow clamps page against the row count and returns the slice bounds,
// total page count and the clamped page number.
func pageWindow(total, page, perPage int) (start, end, pages, clamped int) {
	pages = (total + perPage - 1) / perPage
	if pages < 1 {
		pages = 1
	}
	clamped = page
	if clamped < 1 {
		clamped = 1
	}
	if clamped > pages {
		clamped = pages
	}
	start = (clamped - 1) * perPage
	end = start + perPage
	if end > total {
		end = total
	}
	if start > total {
		start = total
	}
	return start, end, pages, clamped
}

func (h *Handler) StatisticsPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	tab := r.URL.Query().Get("tab")
	switch tab {
	case "farming", "resources":
	default:
		tab = "battle"
	}

	d := statisticsData{baseData: h.base(ctx, r, userID, "Statistics", h.homeID(ctx, userID)), Tab: tab}
	switch tab {
	case "farming":
		d.Farming, _ = h.stats.Farming(ctx, userID)
	case "resources":
		d.ResourceBalance, _ = h.stats.Resources(ctx, userID)
	default:
		d.Battle, _ = h.stats.BattleBalance(ctx, userID)
	}
	h.render(w, "statistics", http.StatusOK, d)
}

func (h *Handler) SearchPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	d := searchData{baseData: h.base(ctx, r, userID, "Search", h.homeID(ctx, userID)), Query: q}
	if q == "" {
		h.render(w, "search", http.StatusOK, d)
		return
	}

	if planets, err := h.planets.SearchPlanets(ctx, h.universeCode, q); err == nil {
		for _, p := range planets {
			d.PlanetHits = append(d.PlanetHits, planetSearchRow{
				Name:      p.Name,
				Coords:    coords(p.Galaxy, p.System, p.Position),
				OwnerID:   p.OwnerID,
				OwnerName: p.OwnerName,
			})
		}
	}

	needle := strings.ToLower(q)
	tags, _ := h.alliances.TagsByUser(ctx, h.universeCode)
	if players, err := h.scores.Players(ctx, h.universeCode); err == nil {
		var matches []store.PlayerScore
		for _, p := range players {
			if strings.Contains(strings.ToLower(p.Username), needle) {
				matches = append(matches, p)
			}
		}
		sort.SliceStable(matches, func(i, j int) bool { return matches[i].Total() > matches[j].Total() })
		for _, p := range matches {
			d.Players = append(d.Players, playerScoreRow{
				UserID:         p.UserID,
				Username:       p.Username,
				Tag:            tags[p.UserID],
				PlanetPoints:   int64(p.PlanetPoints + 0.5),
				ResearchPoints: int64(p.ResearchPoints + 0.5),
				Total:          int64(p.Total() + 0.5),
				Planets:        p.Planets,
				IsMe:           p.UserID == userID,
			})
		}
	}
	h.render(w, "search", http.StatusOK, d)
}

func (h *Handler) PlayerPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	playerID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	name, err := h.auth.Username(ctx, playerID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	d := playerData{
		baseData: h.base(ctx, r, userID, "Player", h.homeID(ctx, userID)),
		PlayerID: playerID,
		Username: name,
		IsMe:     playerID == userID,
	}
	if tags, err := h.alliances.TagsByUser(ctx, h.universeCode); err == nil {
		d.Tag = tags[playerID]
	}
	if m, err := h.alliances.Membership(ctx, playerID); err == nil {
		d.AllianceID = m.AllianceID
	}

	if players, err := h.scores.Players(ctx, h.universeCode); err == nil {
		sort.SliceStable(players, func(i, j int) bool { return players[i].Total() > players[j].Total() })
		for i, p := range players {
			if p.UserID == playerID {
				d.Rank = i + 1
				d.PlanetPoints = int64(p.PlanetPoints + 0.5)
				d.ResearchPoints = int64(p.ResearchPoints + 0.5)
				d.TotalPoints = int64(p.Total() + 0.5)
				break
			}
		}
	}

	planetPoints := map[int64]int64{}
	if scores, err := h.scores.Planets(ctx, h.universeCode); err == nil {
		for _, p := range scores {
			if p.OwnerID == playerID {
				planetPoints[p.PlanetID] = int64(p.Points + 0.5)
			}
		}
	}
	planets, _ := h.planets.ListOwned(ctx, playerID)
	for _, p := range planets {
		d.PlanetRows = append(d.PlanetRows, playerPlanetRow{
			Name:   p.Name,
			Coords: coords(p.Galaxy, p.System, p.Position),
			Points: planetPoints[p.ID],
			IsHome: p.IsHomeworld,
		})
	}
	h.render(w, "player", http.StatusOK, d)
}

// ---- alliance handlers ----

// allianceScores maps every player id to their total economy points.
func (h *Handler) allianceScores(ctx context.Context) map[int64]float64 {
	out := map[int64]float64{}
	if players, err := h.scores.Players(ctx, h.universeCode); err == nil {
		for _, p := range players {
			out[p.UserID] = p.Total()
		}
	}
	return out
}

// memberContext resolves the caller's alliance, or ok=false when not a member.
func (h *Handler) memberContext(ctx context.Context, userID int64) (*store.Alliance, store.Membership, bool) {
	m, err := h.alliances.Membership(ctx, userID)
	if err != nil {
		return nil, store.Membership{}, false
	}
	a, err := h.alliances.Get(ctx, m.AllianceID)
	if err != nil {
		return nil, store.Membership{}, false
	}
	return a, m, true
}

// allianceDetailView builds the header detail (points summed over members) and
// the member list for an alliance.
func (h *Handler) allianceDetailView(ctx context.Context, a *store.Alliance, scores map[int64]float64) (*allianceDetail, []store.AllianceMember) {
	members, _ := h.alliances.Members(ctx, a.ID)
	d := &allianceDetail{
		ID: a.ID, Tag: a.Tag, Name: a.Name, Description: a.Description,
		FounderID: a.FounderID, FounderName: a.FounderName,
		IsOpen: a.IsOpen, MinPoints: a.MinPoints,
		CreatedAt: a.CreatedAt.Format("2006-01-02"), Members: len(members),
	}
	var total float64
	for _, m := range members {
		total += scores[m.UserID]
	}
	d.Points = int64(total + 0.5)
	return d, members
}

// allianceScaffold loads the shared membership context used by every internal
// page. It redirects to /alliance and returns ok=false for non-members.
func (h *Handler) allianceScaffold(w http.ResponseWriter, r *http.Request, tab string) (allianceData, *store.Alliance, bool) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	a, m, ok := h.memberContext(ctx, userID)
	if !ok {
		http.Redirect(w, r, "/alliance", http.StatusSeeOther)
		return allianceData{}, nil, false
	}
	d := allianceData{
		baseData:              h.base(ctx, r, userID, "Alliance", h.homeID(ctx, userID)),
		Tab:                   tab,
		InAlliance:            true,
		MyRank:                string(m.Rank),
		CanManageApplications: m.Rank.CanManageApplications(),
		CanModerateBoard:      m.Rank.CanModerateBoard(),
		CanManageAlliance:     m.Rank.CanManageAlliance(),
	}
	detail, members := h.allianceDetailView(ctx, a, h.allianceScores(ctx))
	d.Alliance = detail
	d.Members = newMemberRows(members, userID)
	return d, a, true
}

// AlliancePage is the public hub: the alliance directory (paged + searchable).
// Members are redirected to their internal dashboard.
func (h *Handler) AlliancePage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	if _, _, ok := h.memberContext(ctx, userID); ok {
		http.Redirect(w, r, "/alliance/internal", http.StatusSeeOther)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	list, total, _ := h.alliances.List(ctx, h.universeCode, q, page, browsePageSize)
	d := allianceData{
		baseData: h.base(ctx, r, userID, "Alliance", h.homeID(ctx, userID)),
		Tab:      "browse", Query: q, Total: total,
	}
	d.Pages = (total + browsePageSize - 1) / browsePageSize
	if d.Pages < 1 {
		d.Pages = 1
	}
	if page > d.Pages {
		page = d.Pages
	}
	d.Page, d.PrevPage, d.NextPage = page, page-1, page+1

	agg, _ := h.scores.Alliances(ctx, h.universeCode)
	byID := make(map[int64]store.AllianceScore, len(agg))
	for _, a := range agg {
		byID[a.ID] = a
	}
	for _, a := range list {
		row := allianceListRow{ID: a.ID, Tag: a.Tag, Name: a.Name, IsOpen: a.IsOpen}
		if s, ok := byID[a.ID]; ok {
			row.Points = int64(s.Points + 0.5)
			row.Members = s.Members
		}
		d.Alliances = append(d.Alliances, row)
	}
	if apps, err := h.alliances.MyApplications(ctx, userID); err == nil {
		d.MyApps = newApplicationRows(apps)
	}
	h.render(w, "alliance", http.StatusOK, d)
}

// AllianceProfilePage is the public profile of one alliance.
func (h *Handler) AllianceProfilePage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	a, err := h.alliances.Get(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	d := allianceData{baseData: h.base(ctx, r, userID, "Alliance", h.homeID(ctx, userID)), Tab: "profile"}
	detail, members := h.allianceDetailView(ctx, a, h.allianceScores(ctx))
	d.Alliance = detail
	d.Members = newMemberRows(members, userID)
	if _, m, ok := h.memberContext(ctx, userID); ok {
		d.InAlliance = true
		d.IsMyAlliance = m.AllianceID == id
		d.MyRank = string(m.Rank)
		d.CanManageApplications = m.Rank.CanManageApplications()
		d.CanModerateBoard = m.Rank.CanModerateBoard()
		d.CanManageAlliance = m.Rank.CanManageAlliance()
	}
	if apps, err := h.alliances.MyApplications(ctx, userID); err == nil {
		for _, ap := range apps {
			if ap.AllianceID == id {
				d.Applied, d.AppID = true, ap.ID
			}
		}
	}
	h.render(w, "alliance", http.StatusOK, d)
}

func (h *Handler) AllianceCreatePage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	if _, _, ok := h.memberContext(ctx, userID); ok {
		http.Redirect(w, r, "/alliance/internal", http.StatusSeeOther)
		return
	}
	d := allianceData{baseData: h.base(ctx, r, userID, "Alliance", h.homeID(ctx, userID)), Tab: "create"}
	h.render(w, "alliance", http.StatusOK, d)
}

func (h *Handler) AllianceCreateSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	tag := strings.TrimSpace(r.FormValue("tag"))
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	if m := validateAllianceTag(tag); m != "" {
		flashRedirect(w, r, "/alliance/create", "", m)
		return
	}
	if m := validateAllianceName(name); m != "" {
		flashRedirect(w, r, "/alliance/create", "", m)
		return
	}
	if _, err := h.alliances.Create(r.Context(), h.universeCode, userID, tag, name, description); err != nil {
		flashRedirect(w, r, "/alliance/create", "", msg(err))
		return
	}
	flashRedirect(w, r, "/alliance/internal", "Alliance founded.", "")
}

// AllianceApplicationsPage lists the caller's own open applications.
func (h *Handler) AllianceApplicationsPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	d := allianceData{baseData: h.base(ctx, r, userID, "Alliance", h.homeID(ctx, userID)), Tab: "myapps"}
	if _, _, ok := h.memberContext(ctx, userID); ok {
		d.InAlliance = true
	}
	if apps, err := h.alliances.MyApplications(ctx, userID); err == nil {
		d.MyApps = newApplicationRows(apps)
	}
	h.render(w, "alliance", http.StatusOK, d)
}

func (h *Handler) AllianceJoinSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	a, err := h.alliances.Get(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if a.MinPoints > 0 {
		if pts := h.allianceScores(ctx)[userID]; int(pts) < a.MinPoints {
			flashRedirect(w, r, "/alliance/"+strconv.FormatInt(id, 10), "",
				"Your economy points are below this alliance's minimum.")
			return
		}
	}
	if err := h.alliances.Join(ctx, userID, id); err != nil {
		flashRedirect(w, r, "/alliance/"+strconv.FormatInt(id, 10), "", msg(err))
		return
	}
	flashRedirect(w, r, "/alliance/internal", "Welcome to the alliance.", "")
}

func (h *Handler) AllianceApplySubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	message := strings.TrimSpace(r.FormValue("message"))
	err := h.alliances.Apply(r.Context(), userID, id, message)
	flashRedirect(w, r, "/alliance/applications", "Application sent.", msg(err))
}

func (h *Handler) AllianceWithdrawSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	err := h.alliances.Withdraw(r.Context(), userID, id)
	flashRedirect(w, r, "/alliance/applications", "Application withdrawn.", msg(err))
}

func (h *Handler) AllianceInternalPage(w http.ResponseWriter, r *http.Request) {
	d, _, ok := h.allianceScaffold(w, r, "dashboard")
	if !ok {
		return
	}
	h.render(w, "alliance", http.StatusOK, d)
}

func (h *Handler) AllianceMembersPage(w http.ResponseWriter, r *http.Request) {
	d, _, ok := h.allianceScaffold(w, r, "members")
	if !ok {
		return
	}
	h.render(w, "alliance", http.StatusOK, d)
}

func (h *Handler) AllianceBoardPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	d, a, ok := h.allianceScaffold(w, r, "board")
	if !ok {
		return
	}
	posts, _ := h.alliances.Posts(r.Context(), a.ID)
	d.Posts = newPostRows(posts, userID, d.CanModerateBoard)
	h.render(w, "alliance", http.StatusOK, d)
}

func (h *Handler) AllianceApplicationsInternalPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	d, a, ok := h.allianceScaffold(w, r, "applications")
	if !ok {
		return
	}
	if apps, err := h.alliances.PendingApplications(r.Context(), userID, a.ID); err == nil {
		d.Pending = newApplicationRows(apps)
	}
	h.render(w, "alliance", http.StatusOK, d)
}

func (h *Handler) AllianceSettingsPage(w http.ResponseWriter, r *http.Request) {
	d, _, ok := h.allianceScaffold(w, r, "settings")
	if !ok {
		return
	}
	h.render(w, "alliance", http.StatusOK, d)
}

func (h *Handler) AllianceLeaveSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	err := h.alliances.Leave(r.Context(), userID)
	flashRedirect(w, r, "/alliance", "You left the alliance.", msg(err))
}

func (h *Handler) AllianceRankSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	targetID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	rank := store.ParseRank(r.FormValue("rank"))
	_, m, ok := h.memberContext(r.Context(), userID)
	if !ok {
		http.Redirect(w, r, "/alliance", http.StatusSeeOther)
		return
	}
	err := h.alliances.SetRank(r.Context(), userID, m.AllianceID, targetID, rank)
	flashRedirect(w, r, "/alliance/internal/members", "Rank updated.", msg(err))
}

func (h *Handler) AllianceBoardSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	_, m, ok := h.memberContext(r.Context(), userID)
	if !ok {
		http.Redirect(w, r, "/alliance", http.StatusSeeOther)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimSpace(r.FormValue("body"))
	pinned := r.FormValue("pinned") != ""
	if v := validatePost(title, body); v != "" {
		flashRedirect(w, r, "/alliance/internal/board", "", v)
		return
	}
	err := h.alliances.AddPost(r.Context(), userID, m.AllianceID, title, body, pinned)
	flashRedirect(w, r, "/alliance/internal/board", "Posted.", msg(err))
}

func (h *Handler) AlliancePostDeleteSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	err := h.alliances.DeletePost(r.Context(), userID, id)
	flashRedirect(w, r, "/alliance/internal/board", "Post deleted.", msg(err))
}

func (h *Handler) AllianceApplicationDecideSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	accept := r.FormValue("decision") == "accept"
	err := h.alliances.Decide(r.Context(), userID, id, accept)
	flashRedirect(w, r, "/alliance/internal/applications", "Application reviewed.", msg(err))
}

func (h *Handler) AllianceSettingsSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	_, m, ok := h.memberContext(r.Context(), userID)
	if !ok {
		http.Redirect(w, r, "/alliance", http.StatusSeeOther)
		return
	}
	description := strings.TrimSpace(r.FormValue("description"))
	isOpen := r.FormValue("isOpen") != ""
	minPoints := atoiDefault(r.FormValue("minPoints"), 0)
	if minPoints < 0 {
		minPoints = 0
	}
	err := h.alliances.UpdateSettings(r.Context(), userID, m.AllianceID, description, isOpen, minPoints)
	flashRedirect(w, r, "/alliance/internal/settings", "Settings saved.", msg(err))
}

func (h *Handler) AllianceCloseSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	_, m, ok := h.memberContext(r.Context(), userID)
	if !ok {
		http.Redirect(w, r, "/alliance", http.StatusSeeOther)
		return
	}
	err := h.alliances.Close(r.Context(), userID, m.AllianceID)
	flashRedirect(w, r, "/alliance", "Alliance closed.", msg(err))
}

func newApplicationRows(apps []store.AllianceApplication) []allianceApplicationRow {
	var out []allianceApplicationRow
	for _, a := range apps {
		out = append(out, allianceApplicationRow{
			ID: a.ID, AllianceID: a.AllianceID, Tag: a.Tag, Name: a.Name,
			Username: a.Username, UserID: a.UserID, Message: a.Message,
			When: a.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	return out
}

func newMemberRows(members []store.AllianceMember, meID int64) []allianceMemberRow {
	var out []allianceMemberRow
	for _, m := range members {
		out = append(out, allianceMemberRow{
			UserID: m.UserID, Username: m.Username, Rank: m.Rank,
			Joined: m.JoinedAt.Format("2006-01-02"), IsFounder: m.IsFounder, IsMe: m.UserID == meID,
		})
	}
	return out
}

func newPostRows(posts []store.AlliancePost, meID int64, canModerate bool) []alliancePostRow {
	var out []alliancePostRow
	for _, p := range posts {
		out = append(out, alliancePostRow{
			ID: p.ID, AuthorID: p.AuthorID, AuthorName: p.AuthorName,
			Title: p.Title, Body: p.Body, IsPinned: p.IsPinned,
			When:      p.CreatedAt.Format("2006-01-02 15:04"),
			CanDelete: p.AuthorID == meID || canModerate,
		})
	}
	return out
}

func (h *Handler) SettingsRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/settings/account", http.StatusSeeOther)
}

func (h *Handler) SettingsAccountPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	d := settingsData{baseData: h.base(ctx, r, userID, "Settings", h.homeID(ctx, userID)), Tab: "account"}
	d.Username, _ = h.auth.Username(ctx, userID)
	d.Email, _ = h.auth.Email(ctx, userID)
	h.render(w, "settings", http.StatusOK, d)
}

func (h *Handler) SettingsGamePage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	st, _ := h.auth.Settings(ctx, userID)
	d := settingsData{
		baseData:           h.base(ctx, r, userID, "Settings", h.homeID(ctx, userID)),
		Tab:                "game",
		NavigationPosition: st.NavigationPosition,
		Locale:             st.Locale,
		MessagesPerPage:    st.MessagesPerPage,
		SpyShortcutCount:   st.SpyShortcutCount,
		SpyLimitOnSpy:      st.SpyLimitOnSpy,
	}
	h.render(w, "settings", http.StatusOK, d)
}

func (h *Handler) SettingsAccountSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	back := "/settings/account"

	hash, err := h.auth.PasswordHash(ctx, userID)
	if err != nil || !auth.VerifyPassword(r.FormValue("currentPassword"), hash) {
		flashRedirect(w, r, back, "", "Current password is incorrect.")
		return
	}

	switch r.FormValue("action") {
	case "username":
		name := strings.TrimSpace(r.FormValue("name"))
		if m := validateUsername(name); m != "" {
			flashRedirect(w, r, back, "", m)
			return
		}
		flashRedirect(w, r, back, "Username updated.", msg(h.auth.UpdateUsername(ctx, userID, name)))
	case "email":
		email := strings.ToLower(strings.TrimSpace(r.FormValue("mail")))
		if m := validateEmail(email); m != "" {
			flashRedirect(w, r, back, "", m)
			return
		}
		flashRedirect(w, r, back, "Email updated.", msg(h.auth.UpdateEmail(ctx, userID, email)))
	case "password":
		first := r.FormValue("passwordFirst")
		if m := validatePassword(first); m != "" {
			flashRedirect(w, r, back, "", m)
			return
		}
		if first != r.FormValue("passwordSecond") {
			flashRedirect(w, r, back, "", "Passwords do not match.")
			return
		}
		newHash, err := auth.HashPassword(first)
		if err != nil {
			flashRedirect(w, r, back, "", "Could not secure password.")
			return
		}
		if err := h.auth.UpdatePassword(ctx, userID, newHash); err != nil {
			flashRedirect(w, r, back, "", msg(err))
			return
		}
		h.issueSession(w, r, userID)
		flashRedirect(w, r, back, "Password updated.", "")
	default:
		flashRedirect(w, r, back, "", "Unknown action.")
	}
}

func (h *Handler) SettingsGameSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()

	st := store.DefaultUserSettings()
	if r.FormValue("navigationPosition") == "right" {
		st.NavigationPosition = "right"
	}
	if r.FormValue("locale") == "de" {
		st.Locale = "de"
	}
	st.MessagesPerPage = clampInt(atoiDefault(r.FormValue("messagesPerPage"), st.MessagesPerPage), 1, 100)
	st.SpyShortcutCount = clampInt(atoiDefault(r.FormValue("spyShortcutCount"), st.SpyShortcutCount), 0, 100)
	st.SpyLimitOnSpy = clampInt(atoiDefault(r.FormValue("spyLimitOnSpy"), st.SpyLimitOnSpy), 0, 100000)

	if err := h.auth.UpdateSettings(ctx, userID, st); err != nil {
		flashRedirect(w, r, "/settings/game", "", "Could not save settings.")
		return
	}
	flashRedirect(w, r, "/settings/game", "Settings saved.", "")
}

// ---- submit handlers ----

func (h *Handler) BuildSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id := pathID(r)
	if _, ok := h.owned(r.Context(), userID, id); !ok {
		http.NotFound(w, r)
		return
	}
	code := r.FormValue("code")
	err := h.queues.StartBuilding(r.Context(), h.universeCode, userID, id, code)
	flashRedirect(w, r, "/buildings/"+strconv.FormatInt(id, 10), "Construction started.", msg(err))
}

func (h *Handler) ResearchSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id := pathID(r)
	if _, ok := h.owned(r.Context(), userID, id); !ok {
		http.NotFound(w, r)
		return
	}
	code := r.FormValue("code")
	err := h.queues.StartResearch(r.Context(), h.universeCode, userID, id, code)
	flashRedirect(w, r, "/research/"+strconv.FormatInt(id, 10), "Research started.", msg(err))
}

func (h *Handler) ShipyardSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id := pathID(r)
	if _, ok := h.owned(r.Context(), userID, id); !ok {
		http.NotFound(w, r)
		return
	}
	code := r.FormValue("code")
	qty, _ := strconv.ParseInt(r.FormValue("qty"), 10, 64)
	def, ok := game.Get(code)
	if !ok {
		flashRedirect(w, r, "/shipyard/"+strconv.FormatInt(id, 10), "", "Unknown item.")
		return
	}
	err := h.queues.StartUnits(r.Context(), h.universeCode, userID, id, string(def.Kind), code, qty)
	flashRedirect(w, r, "/shipyard/"+strconv.FormatInt(id, 10), "Production queued.", msg(err))
}

func (h *Handler) FleetDispatchSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id := pathID(r)
	if _, ok := h.owned(r.Context(), userID, id); !ok {
		http.NotFound(w, r)
		return
	}
	back := "/fleet/" + strconv.FormatInt(id, 10)

	mission := r.FormValue("mission")
	g, _ := strconv.Atoi(r.FormValue("g"))
	s, _ := strconv.Atoi(r.FormValue("s"))
	pos, _ := strconv.Atoi(r.FormValue("p"))
	speedPct, _ := strconv.Atoi(r.FormValue("speed"))
	if speedPct <= 0 {
		speedPct = 100
	}

	owned, _ := h.planets.Units(r.Context(), id, "ship")
	ships := map[string]int64{}
	for code := range owned {
		n, _ := strconv.ParseInt(r.FormValue("ship_"+code), 10, 64)
		if n > 0 {
			ships[code] = n
		}
	}
	if len(ships) == 0 {
		flashRedirect(w, r, back, "", "Select at least one ship.")
		return
	}

	parseRes := func(name string) float64 {
		v, _ := strconv.ParseFloat(r.FormValue(name), 64)
		return v
	}
	cargo := game.Resources{
		Metal: parseRes("cargo_metal"), Lutinum: parseRes("cargo_lutinum"),
		Water: parseRes("cargo_water"), Hydrogen: parseRes("cargo_hydrogen"),
	}

	_, err := h.fleets.Dispatch(r.Context(), store.DispatchParams{
		UniverseCode: h.universeCode,
		UserID:       userID,
		OriginID:     id,
		Mission:      mission,
		TargetG:      g, TargetS: s, TargetP: pos,
		SpeedPct: float64(speedPct) / 100.0,
		Ships:    ships,
		Cargo:    cargo,
	})
	flashRedirect(w, r, back, "Fleet dispatched.", msg(err))
}

func (h *Handler) FleetRecallSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id := pathID(r)
	fleetID, _ := strconv.ParseInt(r.FormValue("fleet"), 10, 64)
	err := h.fleets.Recall(r.Context(), fleetID, userID)
	flashRedirect(w, r, "/fleet/"+strconv.FormatInt(id, 10), "Fleet recalled.", msg(err))
}

func (h *Handler) QueueCancelSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	qid, _ := strconv.ParseInt(r.PathValue("qid"), 10, 64)
	err := h.queues.Cancel(r.Context(), qid, userID)
	to := r.Referer()
	if to == "" {
		to = "/overview/" + strconv.FormatInt(h.homeID(r.Context(), userID), 10)
	}
	flashRedirect(w, r, to, "Order cancelled.", msg(err))
}

// ---- small helpers ----

func (h *Handler) activeQueue(ctx context.Context, planetID, userID int64, kind string) (store.Queue, bool) {
	qs, err := h.queues.Active(ctx, planetID, userID)
	if err != nil {
		return store.Queue{}, false
	}
	for _, q := range qs {
		if q.Kind == kind {
			return q, true
		}
	}
	return store.Queue{}, false
}

func (h *Handler) gameSpeed(ctx context.Context) (float64, error) {
	var v float64
	err := h.db.QueryRowContext(ctx, `SELECT game_speed FROM universes WHERE code_name = $1`, h.universeCode).Scan(&v)
	return v, err
}

func (h *Handler) universeSpeeds(ctx context.Context) (game, resource, fleet float64, maxG, maxS int) {
	_ = h.db.QueryRowContext(ctx, `
		SELECT game_speed, resource_speed, fleet_speed, max_galaxies, max_systems
		FROM universes WHERE code_name = $1
	`, h.universeCode).Scan(&game, &resource, &fleet, &maxG, &maxS)
	if maxG < 1 {
		maxG = 1
	}
	if maxS < 1 {
		maxS = 1
	}
	return
}

func sortedDefs(kind game.Kind) []game.Def {
	defs := game.DefsOf(kind)
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs
}

func maxAffordable(p *store.Planet, cost game.Resources) int64 {
	max := int64(-1)
	check := func(have, need float64) {
		if need <= 0 {
			return
		}
		v := int64(have / need)
		if max < 0 || v < max {
			max = v
		}
	}
	check(p.Metal, cost.Metal)
	check(p.Lutinum, cost.Lutinum)
	check(p.Water, cost.Water)
	check(p.Hydrogen, cost.Hydrogen)
	if max < 0 {
		return 0
	}
	return max
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---- trade handlers ----

// tradePageSize is the marketplace page length.
const tradePageSize = 30

// tradeResourceOptions fixes the select order for the marketplace and offer form.
var tradeResourceOptions = []resourceOption{
	{Key: "iron", Name: "Iron"},
	{Key: "lutinum", Name: "Lutinum"},
	{Key: "water", Name: "Water"},
	{Key: "hydrogen", Name: "Hydrogen"},
}

// tradeResFromForm turns one resource key + amount into a single-resource basket.
func tradeResFromForm(key string, amount float64) game.Resources {
	if amount <= 0 {
		return game.Resources{}
	}
	switch key {
	case "iron":
		return game.Resources{Metal: amount}
	case "lutinum":
		return game.Resources{Lutinum: amount}
	case "water":
		return game.Resources{Water: amount}
	case "hydrogen":
		return game.Resources{Hydrogen: amount}
	default:
		return game.Resources{}
	}
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func resAdd(a, b game.Resources) game.Resources {
	return game.Resources{
		Metal: a.Metal + b.Metal, Lutinum: a.Lutinum + b.Lutinum,
		Water: a.Water + b.Water, Hydrogen: a.Hydrogen + b.Hydrogen,
	}
}

// tradeResString renders a basket with full resource names, e.g. "10,000 Iron".
func tradeResString(r game.Resources) string {
	var parts []string
	if r.Metal > 0 {
		parts = append(parts, fmtFloat(r.Metal)+" Iron")
	}
	if r.Lutinum > 0 {
		parts = append(parts, fmtFloat(r.Lutinum)+" Lutinum")
	}
	if r.Water > 0 {
		parts = append(parts, fmtFloat(r.Water)+" Water")
	}
	if r.Hydrogen > 0 {
		parts = append(parts, fmtFloat(r.Hydrogen)+" Hydrogen")
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " + ")
}

func (h *Handler) tradeOfferRowView(o store.TradeOffer, viewerID int64) tradeOfferRow {
	return tradeOfferRow{
		ID:          o.ID,
		CreatorID:   o.CreatorID,
		CreatorName: o.CreatorName,
		Planet:      coords(o.Galaxy, o.System, o.Position),
		Private:     o.IsPrivate(),
		TargetName:  o.TargetName,
		GiveStr:     tradeResString(o.Give),
		WantStr:     tradeResString(o.Want),
		Status:      o.Status,
		Expires:     o.ExpiresAt.Format("2006-01-02 15:04"),
		Created:     o.CreatedAt.Format("2006-01-02 15:04"),
		IsMine:      o.CreatorID == viewerID,
		CanAccept:   o.Status == "OPEN" && o.CreatorID != viewerID && (!o.TargetID.Valid || o.TargetID.Int64 == viewerID),
	}
}

// TradePage is the public marketplace listing.
func (h *Handler) TradePage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	_ = h.trade.ExpireOverdue(ctx, 100)

	q := r.URL.Query()
	offered, requested := q.Get("offered"), q.Get("requested")
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}

	d := tradeData{
		baseData:       h.base(ctx, r, userID, "Trade", h.homeID(ctx, userID)),
		Tab:            "market",
		Offered:        offered,
		Requested:      requested,
		OfferedOptions: tradeResourceOptions,
	}
	offers, total, _ := h.trade.ListMarket(ctx, h.universeCode, userID, offered, requested, page, tradePageSize)
	d.Total = total
	d.Pages = (total + tradePageSize - 1) / tradePageSize
	if d.Pages < 1 {
		d.Pages = 1
	}
	if page > d.Pages {
		page = d.Pages
	}
	d.Page, d.PrevPage, d.NextPage = page, page-1, page+1
	for _, o := range offers {
		d.Offers = append(d.Offers, h.tradeOfferRowView(o, userID))
	}
	h.render(w, "trade", http.StatusOK, d)
}

// TradeNewPage renders the create-offer form.
func (h *Handler) TradeNewPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	planets, _ := h.planets.ListOwned(ctx, userID)
	d := tradeData{
		baseData:       h.base(ctx, r, userID, "Trade", h.homeID(ctx, userID)),
		Tab:            "new",
		Planets:        planets,
		OfferedOptions: tradeResourceOptions,
	}
	if target := strings.TrimSpace(r.URL.Query().Get("target")); target != "" {
		if id, err := strconv.ParseInt(target, 10, 64); err == nil {
			if id != userID {
				d.TargetID = id
				d.Target, _ = h.auth.Username(ctx, id)
			}
		} else if id, err := h.auth.UserIDByUsername(ctx, h.universeCode, target); err == nil && id != userID {
			d.TargetID, d.Target = id, target
		}
	}
	h.render(w, "trade", http.StatusOK, d)
}

func (h *Handler) TradeCreateSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	back := "/trade/new"

	planetID, _ := strconv.ParseInt(r.FormValue("planet"), 10, 64)
	give := tradeResFromForm(r.FormValue("give"), parseFloat(r.FormValue("give_amount")))
	want := tradeResFromForm(r.FormValue("want"), parseFloat(r.FormValue("want_amount")))
	if give.IsZero() || want.IsZero() {
		flashRedirect(w, r, back, "", "Enter a positive amount for both sides.")
		return
	}

	var targetID int64
	if name := strings.TrimSpace(r.FormValue("target")); name != "" {
		id, err := h.auth.UserIDByUsername(ctx, h.universeCode, name)
		if err != nil {
			flashRedirect(w, r, back, "", "No such trading partner.")
			return
		}
		if id == userID {
			flashRedirect(w, r, back, "", "You cannot trade with yourself.")
			return
		}
		targetID = id
	}

	if _, err := h.trade.CreateOffer(ctx, h.universeCode, userID, planetID, targetID, give, want); err != nil {
		flashRedirect(w, r, back, "", msg(err))
		return
	}
	flashRedirect(w, r, "/trade/own", "Offer posted.", "")
}

// TradeOwnPage lists the caller's open offers.
func (h *Handler) TradeOwnPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	_ = h.trade.ExpireOverdue(ctx, 100)

	d := tradeData{baseData: h.base(ctx, r, userID, "Trade", h.homeID(ctx, userID)), Tab: "own"}
	offers, _ := h.trade.MyOffers(ctx, userID)
	for _, o := range offers {
		d.MyOffers = append(d.MyOffers, h.tradeOfferRowView(o, userID))
	}
	h.render(w, "trade", http.StatusOK, d)
}

// TradeHistoryPage lists completed/closed trades the caller took part in.
func (h *Handler) TradeHistoryPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	d := tradeData{baseData: h.base(ctx, r, userID, "Trade", h.homeID(ctx, userID)), Tab: "history"}
	entries, _ := h.trade.History(ctx, userID, 100)
	for _, e := range entries {
		d.History = append(d.History, tradeHistoryRow{
			When:           e.At.Format("2006-01-02 15:04"),
			Action:         e.Action,
			Actor:          e.ActorName,
			ActorID:        e.ActorID,
			Counterparty:   e.CounterpartyName,
			CounterpartyID: e.CounterpartyID,
			GiveStr:        tradeResString(e.Give),
			WantStr:        tradeResString(e.Want),
		})
	}
	h.render(w, "trade", http.StatusOK, d)
}

// TradeStoragePage shows each owned planet's pending trade deliveries.
func (h *Handler) TradeStoragePage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	d := tradeData{baseData: h.base(ctx, r, userID, "Trade", h.homeID(ctx, userID)), Tab: "storage"}

	owned, _ := h.planets.ListOwned(ctx, userID)
	for _, p := range owned {
		entries, _ := h.trade.Storage(ctx, p.ID)
		sp := tradeStoragePlanet{
			PlanetID: p.ID, Name: p.Name,
			Coords: coords(p.Galaxy, p.System, p.Position),
		}
		var total game.Resources
		for _, e := range entries {
			sp.Entries = append(sp.Entries, tradeStorageEntryRow{
				GiveStr: tradeResString(e.Resources),
				Expires: e.ExpiresAt.Format("2006-01-02 15:04"),
			})
			total = resAdd(total, e.Resources)
		}
		sp.Total = tradeResString(total)
		d.Storage = append(d.Storage, sp)
	}
	h.render(w, "trade", http.StatusOK, d)
}

func (h *Handler) TradeStorageCreditSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	planetID, _ := strconv.ParseInt(r.FormValue("planet"), 10, 64)
	sum, err := h.trade.CreditStorage(ctx, userID, planetID)
	if err != nil {
		flashRedirect(w, r, "/trade/storage", "", msg(err))
		return
	}
	flashRedirect(w, r, "/trade/storage", "Credited "+tradeResString(sum)+" to the planet.", "")
}

// TradeOfferPage shows one offer with an accept (or revoke) form.
func (h *Handler) TradeOfferPage(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	o, err := h.trade.GetOffer(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	row := h.tradeOfferRowView(*o, userID)
	planets, _ := h.planets.ListOwned(ctx, userID)
	d := tradeData{
		baseData: h.base(ctx, r, userID, "Trade", h.homeID(ctx, userID)),
		Tab:      "offer",
		Offer:    &row,
		Planets:  planets,
	}
	h.render(w, "trade", http.StatusOK, d)
}

func (h *Handler) TradeAcceptSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	ctx := r.Context()
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	planetID, _ := strconv.ParseInt(r.FormValue("planet"), 10, 64)
	if planetID == 0 {
		planetID = h.homeID(ctx, userID)
	}
	err := h.trade.Accept(ctx, id, userID, planetID)
	flashRedirect(w, r, "/trade/storage", "Trade complete — collect it from storage.", msg(err))
}

func (h *Handler) TradeRevokeSubmit(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.userID(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	err := h.trade.Revoke(r.Context(), id, userID)
	flashRedirect(w, r, "/trade/own", "Offer revoked.", msg(err))
}
