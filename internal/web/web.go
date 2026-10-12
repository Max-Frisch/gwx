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
	mux.HandleFunc("GET /messages", h.requireUser(h.MessagesPage))
	mux.HandleFunc("POST /messages/{id}/read", h.requireUser(h.MessageReadSubmit))
	mux.HandleFunc("POST /messages/{id}/delete", h.requireUser(h.MessageDeleteSubmit))
	mux.HandleFunc("POST /messages/{id}/archive", h.requireUser(h.MessageArchiveSubmit))
	mux.HandleFunc("GET /highscore", h.requireUser(h.HighscorePage))
	mux.HandleFunc("GET /search", h.requireUser(h.SearchPage))
	mux.HandleFunc("GET /player/{id}", h.requireUser(h.PlayerPage))
	mux.HandleFunc("GET /settings", h.requireUser(h.SettingsRedirect))
	mux.HandleFunc("GET /settings/account", h.requireUser(h.SettingsAccountPage))
	mux.HandleFunc("POST /settings/account", h.requireUser(h.SettingsAccountSubmit))
	mux.HandleFunc("GET /settings/game", h.requireUser(h.SettingsGamePage))
	mux.HandleFunc("POST /settings/game", h.requireUser(h.SettingsGameSubmit))
	mux.HandleFunc("POST /queue/{qid}/cancel", h.requireUser(h.QueueCancelSubmit))
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
	tab := r.URL.Query().Get("tab")
	if tab != "planets" {
		tab = "players"
	}
	sortKey := r.URL.Query().Get("sort")

	d := highscoreData{baseData: h.base(ctx, r, userID, "Highscore", h.homeID(ctx, userID)), Tab: tab, Sort: sortKey}

	if tab == "planets" {
		rows, _ := h.scores.Planets(ctx, h.universeCode)
		sort.SliceStable(rows, func(i, j int) bool {
			switch sortKey {
			case "name":
				return rows[i].Name < rows[j].Name
			case "owner":
				return rows[i].OwnerName < rows[j].OwnerName
			default:
				return rows[i].Points > rows[j].Points
			}
		})
		for i, p := range rows {
			d.PlanetRows = append(d.PlanetRows, planetScoreRow{
				Rank:      i + 1,
				Name:      p.Name,
				Coords:    coords(p.Galaxy, p.System, p.Position),
				OwnerName: p.OwnerName,
				Points:    int64(p.Points + 0.5),
			})
		}
	} else {
		rows, _ := h.scores.Players(ctx, h.universeCode)
		sort.SliceStable(rows, func(i, j int) bool {
			switch sortKey {
			case "planet":
				return rows[i].PlanetPoints > rows[j].PlanetPoints
			case "research":
				return rows[i].ResearchPoints > rows[j].ResearchPoints
			case "name":
				return rows[i].Username < rows[j].Username
			default:
				return rows[i].Total() > rows[j].Total()
			}
		})
		for i, p := range rows {
			d.Players = append(d.Players, playerScoreRow{
				Rank:           i + 1,
				UserID:         p.UserID,
				Username:       p.Username,
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
