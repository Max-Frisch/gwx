package integration

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"gwx/internal/engine"
	"gwx/internal/store"
	"gwx/internal/web"
)

// The integration suite drives the real stack — HTTP handlers, the durable
// scheduler and Postgres — against a throwaway database. It is deliberately
// run at absurdly high speeds so the "real thing" (builds, flights, battles)
// finishes in milliseconds instead of hours, then polls the database until the
// effect lands. No production servers are touched; if Postgres is unreachable
// the whole suite skips rather than failing.
const (
	testUniverse   = "itest_universe"
	testGameSpeed  = 10000.0  // build/research time + resource production
	testFleetSpeed = 100000.0 // travel time (flights floor at 1s anyway)
	tickInterval   = 5 * time.Millisecond
	waitTimeout    = 15 * time.Second
	seedAmount     = 5_000_000
)

// harness owns one isolated universe + HTTP server + fast scheduler for a test.
type harness struct {
	db       *sql.DB
	server   *httptest.Server
	engine   *engine.EventEngine
	universe string
	stop     chan struct{}
	done     chan struct{}
}

// baseDSN is the admin connection string used to provision a test database.
func baseDSN() string {
	for _, k := range []string{"GWX_TEST_DATABASE_URL", "DATABASE_URL"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return "postgres://postgres:password@localhost:5433/gwx?sslmode=disable"
}

// withDatabase swaps the database name in a URL-style DSN.
func withDatabase(dsn, name string) (string, bool) {
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		return "", false
	}
	u.Path = "/" + name
	return u.String(), true
}

func quoteIdent(s string) string { return `"` + s + `"` }

func newHarness(t *testing.T) *harness {
	t.Helper()

	dsn := baseDSN()
	adminDSN, ok := withDatabase(dsn, "postgres")
	if !ok {
		t.Skipf("integration tests need a URL-style DSN (got %q)", dsn)
	}
	dbName := fmt.Sprintf("gwx_itest_%d_%d", os.Getpid(), rand.Intn(1_000_000))

	admin, err := sql.Open("postgres", adminDSN)
	if err != nil {
		t.Skipf("open postgres: %v", err)
	}
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := admin.PingContext(pingCtx); err != nil {
		pingCancel()
		admin.Close()
		t.Skipf("postgres unavailable at %s: %v", adminDSN, err)
	}
	pingCancel()

	if _, err := admin.ExecContext(context.Background(), `CREATE DATABASE `+quoteIdent(dbName)); err != nil {
		admin.Close()
		t.Fatalf("create test database %s: %v", dbName, err)
	}

	testDSN, _ := withDatabase(dsn, dbName)
	db, err := store.NewPostgres(context.Background(), testDSN)
	if err != nil {
		admin.Close()
		t.Fatalf("open test database: %v", err)
	}
	applyMigrations(t, db)

	if _, err := db.Exec(`
		INSERT INTO universes (code_name, display_name, game_speed, resource_speed, fleet_speed, max_galaxies, max_systems)
		VALUES ($1, 'Integration Universe', $2, $2, $3, 3, 199)
	`, testUniverse, testGameSpeed, testFleetSpeed); err != nil {
		db.Close()
		admin.Close()
		t.Fatalf("seed universe: %v", err)
	}

	h := &harness{
		db:       db,
		universe: testUniverse,
		engine:   engine.NewEventEngine(db),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	// The Colony Builder's default cadence is 30s; drive it on the same fast
	// tick as everything else so tests do not wait real seconds.
	h.engine.SetBuilderInterval(10 * tickInterval)

	mux := http.NewServeMux()
	web.New(db, h.universe).Mount(mux)
	h.server = httptest.NewServer(mux)

	go h.tickLoop()

	t.Cleanup(func() {
		close(h.stop)
		<-h.done // let any in-flight tick finish before closing the pool
		h.server.Close()
		db.Close()
		_, _ = admin.ExecContext(context.Background(), `DROP DATABASE IF EXISTS `+quoteIdent(dbName)+` WITH (FORCE)`)
		admin.Close()
	})
	return h
}

// tickLoop is the "REALLY fast test server": it resolves due work every few
// milliseconds so a x10000/x100000 universe plays out almost instantly.
func (h *harness) tickLoop() {
	defer close(h.done)
	tk := time.NewTicker(tickInterval)
	defer tk.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-tk.C:
			h.engine.Tick(context.Background())
		}
	}
}

func applyMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate migrations")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "migrations")
	if _, err := store.Migrate(context.Background(), db, dir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// action POSTs a form and returns the final (post-redirect) body. Any `err`
// flash appended to the redirect target fails the test with its message, so a
// rejected game action (bad prereqs, no resources, ...) fails loudly.
func (h *harness) action(t *testing.T, c *http.Client, path string, form url.Values) string {
	t.Helper()
	resp, err := c.PostForm(h.server.URL+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		t.Fatalf("POST %s -> %d: %s", path, resp.StatusCode, snippet(body))
	}
	if e := resp.Request.URL.Query().Get("err"); e != "" {
		t.Fatalf("POST %s rejected: %s", path, e)
	}
	return string(body)
}

func (h *harness) get(t *testing.T, c *http.Client, path string) (int, string) {
	t.Helper()
	resp, err := c.Get(h.server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 240 {
		return s[:240] + "…"
	}
	return s
}

// ---------------------------------------------------------------------------
// Database helpers
// ---------------------------------------------------------------------------

func (h *harness) queryInt(t *testing.T, q string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := h.db.QueryRow(q, args...).Scan(&n); err != nil {
		if err == sql.ErrNoRows {
			return 0
		}
		t.Fatalf("query %q: %v", q, err)
	}
	return n
}

func waitFor(t *testing.T, what string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// ---------------------------------------------------------------------------
// Players
// ---------------------------------------------------------------------------

type player struct {
	name   string
	id     int64
	home   int64
	client *http.Client
}

type coords struct{ g, s, pos int }

// register creates an account through the real registration flow and resolves
// the resulting user + homeworld ids from the database.
func (h *harness) register(t *testing.T, name string) *player {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	c := &http.Client{Jar: jar}
	h.action(t, c, "/register", url.Values{
		"username": {name},
		"email":    {name + "@example.com"},
		"password": {"integration-pass-123"},
	})
	p := &player{name: name, client: c}
	if err := h.db.QueryRow(`SELECT id FROM users WHERE username = $1`, name).Scan(&p.id); err != nil {
		t.Fatalf("lookup user %q: %v", name, err)
	}
	if err := h.db.QueryRow(`SELECT id FROM planets WHERE user_id = $1 AND is_homeworld`, p.id).Scan(&p.home); err != nil {
		t.Fatalf("lookup homeworld for %q: %v", name, err)
	}
	return p
}

// seed writes resources and raises the caps so the next action never trips the
// affordability / capacity checks. It intentionally resets production timing.
func (h *harness) seed(t *testing.T, planetID, metal, lutinum, water, hydrogen int64) {
	t.Helper()
	_, err := h.db.Exec(`
		UPDATE planets SET
			metal_cap = $2, lutinum_cap = $2, water_cap = $2, hydrogen_cap = $2,
			metal = $3, lutinum = $4, water = $5, hydrogen = $6,
			protected_metal = 0, protected_lutinum = 0, protected_water = 0, protected_hydrogen = 0,
			last_resource_calc_at = NOW()
		WHERE id = $1
	`, planetID, int64(1_000_000_000_000), metal, lutinum, water, hydrogen)
	if err != nil {
		t.Fatalf("seed planet %d: %v", planetID, err)
	}
}

func (h *harness) topUp(t *testing.T, p *player) {
	t.Helper()
	h.seed(t, p.home, seedAmount, seedAmount, seedAmount, seedAmount)
}

// at returns a planet's coordinates.
func (h *harness) at(t *testing.T, planetID int64) coords {
	t.Helper()
	var c coords
	if err := h.db.QueryRow(`SELECT galaxy, system, position FROM planets WHERE id = $1`, planetID).
		Scan(&c.g, &c.s, &c.pos); err != nil {
		t.Fatalf("coords for %d: %v", planetID, err)
	}
	return c
}

type resources struct{ metal, lutinum, water, hydrogen int64 }

func (h *harness) resourcesOf(t *testing.T, planetID int64) resources {
	t.Helper()
	var r resources
	if err := h.db.QueryRow(`
		SELECT metal::bigint, lutinum::bigint, water::bigint, hydrogen::bigint
		FROM planets WHERE id = $1
	`, planetID).Scan(&r.metal, &r.lutinum, &r.water, &r.hydrogen); err != nil {
		t.Fatalf("resources of %d: %v", planetID, err)
	}
	return r
}

// ---------------------------------------------------------------------------
// Game actions (all through the real HTTP surface)
// ---------------------------------------------------------------------------

// build queues one building level and waits for it to land.
func (h *harness) build(t *testing.T, p *player, code string, wantLevel int64) {
	t.Helper()
	h.topUp(t, p)
	h.action(t, p.client, fmt.Sprintf("/buildings/%d/build", p.home), url.Values{"code": {code}})
	waitFor(t, fmt.Sprintf("building %s level %d", code, wantLevel), func() bool {
		return h.queryInt(t, `
			SELECT amount FROM planet_items
			WHERE planet_id = $1 AND kind = 'building' AND code = $2
		`, p.home, code) >= wantLevel
	})
}

// research queues one tech level (account-wide) and waits for it to land.
func (h *harness) research(t *testing.T, p *player, code string, wantLevel int64) {
	t.Helper()
	h.topUp(t, p)
	h.action(t, p.client, fmt.Sprintf("/research/%d/start", p.home), url.Values{"code": {code}})
	waitFor(t, fmt.Sprintf("research %s level %d", code, wantLevel), func() bool {
		return h.queryInt(t, `SELECT level FROM user_techs WHERE user_id = $1 AND code = $2`, p.id, code) >= wantLevel
	})
}

// buildUnits queues a batch of ships/defense and waits for the batch to land.
// path is the page route ("shipyard" or "defense").
func (h *harness) buildUnits(t *testing.T, p *player, path, code string, qty, wantCount int64) {
	t.Helper()
	h.topUp(t, p)
	h.action(t, p.client, fmt.Sprintf("/%s/%d/build", path, p.home), url.Values{
		"code": {code},
		"qty":  {strconv.FormatInt(qty, 10)},
	})
	waitFor(t, fmt.Sprintf("%d x %s", qty, code), func() bool {
		return h.queryInt(t, `
			SELECT amount FROM planet_items
			WHERE planet_id = $1 AND kind = 'ship' AND code = $2
		`, p.home, code) >= wantCount
	})
}

// dispatch sends a fleet from a player's homeworld to the given coordinates.
func (h *harness) dispatch(t *testing.T, p *player, mission string, c coords, ships map[string]int64, cargo url.Values) {
	t.Helper()
	form := url.Values{
		"mission": {mission},
		"g":       {strconv.Itoa(c.g)},
		"s":       {strconv.Itoa(c.s)},
		"p":       {strconv.Itoa(c.pos)},
		"speed":   {"100"},
	}
	for code, n := range ships {
		form.Set("ship_"+code, strconv.FormatInt(n, 10))
	}
	for k, vs := range cargo {
		form[k] = vs
	}
	h.action(t, p.client, fmt.Sprintf("/fleet/%d/dispatch", p.home), form)
}

// setTech / setBuilding write prerequisites directly so the colonisation leg of
// the loop does not have to grind the whole deep tech tree through the UI.
func (h *harness) setTech(t *testing.T, userID int64, code string, level int) {
	t.Helper()
	if _, err := h.db.Exec(`
		INSERT INTO user_techs (user_id, code, level) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, code) DO UPDATE SET level = EXCLUDED.level
	`, userID, code, level); err != nil {
		t.Fatalf("set tech %s: %v", code, err)
	}
}

func (h *harness) setBuilding(t *testing.T, planetID int64, code string, level int) {
	t.Helper()
	if _, err := h.db.Exec(`
		INSERT INTO planet_items (planet_id, kind, code, amount) VALUES ($1, 'building', $2, $3)
		ON CONFLICT (planet_id, kind, code) DO UPDATE SET amount = EXCLUDED.amount
	`, planetID, code, level); err != nil {
		t.Fatalf("set building %s: %v", code, err)
	}
}

// freeSlot finds an unoccupied coordinate suitable for a new colony.
func (h *harness) freeSlot(t *testing.T) coords {
	t.Helper()
	for s := 150; s <= 199; s++ {
		for pos := 5; pos <= 10; pos++ {
			if h.queryInt(t, `
				SELECT count(*) FROM planets WHERE galaxy = 1 AND system = $1 AND position = $2
			`, s, pos) == 0 {
				return coords{g: 1, s: s, pos: pos}
			}
		}
	}
	t.Fatal("no free colony slot available")
	return coords{}
}
