package engine

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"gwx/internal/game"
	"gwx/internal/store"
)

// defaultBuilderBatch caps how many Colony Builder accounts one tick drives, so
// a large population never stalls the queue/fleet resolvers. Configurable via
// BuilderStore / SetBuilderBatch.
const defaultBuilderBatch = 25

// defaultBuilderInterval is how often an enabled account is re-checked. A slot
// freeing between checks is simply picked up on the next one.
const defaultBuilderInterval = 30 * time.Second

// SetBuilderBatch overrides the per-tick Colony Builder cap. Values <= 0 keep
// the default.
func (e *EventEngine) SetBuilderBatch(n int) {
	if n > 0 {
		e.builderBatch = n
	}
}

// SetBuilderInterval overrides how often each enabled account is re-checked.
// Values <= 0 keep the default.
func (e *EventEngine) SetBuilderInterval(d time.Duration) {
	if d > 0 {
		e.builderInterval = d
	}
}

// processColonyBuilder is the automatic, template-driven builder's durable
// driver. It leases due accounts and, for each, fills the free construction
// slots of every owned planet toward the account's target-level template. Like
// every other phase it is restart- and multi-instance-safe (the lease uses
// FOR UPDATE SKIP LOCKED).
//
// It deliberately reuses QueueStore.StartBuilding, so cost, prerequisites,
// fields, the per-planet slot cap and vacation/sanction gating are all applied
// exactly as for a manual build.
func (e *EventEngine) processColonyBuilder(ctx context.Context) {
	builders := store.NewBuilderStore(e.db)
	due, err := builders.ClaimDueBuilders(ctx, e.builderBatch, e.builderInterval)
	if err != nil {
		log.Printf("[ENGINE] builder scan: %v", err)
		return
	}
	for _, b := range due {
		if err := e.runBuilder(ctx, b); err != nil {
			log.Printf("[ENGINE] builder %d: %v", b.UserID, err)
		}
	}
}

// runBuilder drives every owned planet of one account toward its template. It
// loads the account's techs once and each planet's structures once.
func (e *EventEngine) runBuilder(ctx context.Context, b store.Builder) error {
	if len(b.Template) == 0 {
		return nil
	}
	planets := store.NewPlanetStore(e.db)
	owned, err := planets.ListOwned(ctx, b.UserID)
	if err != nil {
		return err
	}
	if len(owned) == 0 {
		return nil
	}
	techs, err := planets.Techs(ctx, b.UserID)
	if err != nil {
		return err
	}
	queues := store.NewQueueStore(e.db)
	for _, p := range owned {
		if err := fillPlanet(ctx, queues, planets, b.Universe, b.UserID, p.ID, techs, b.Template); err != nil {
			return err
		}
	}
	return nil
}

// FillPlanetTowardTemplate queues building orders on one planet toward an
// account's template, using the exact logic the durable scheduler uses. It backs
// the UI's "queue now" action so a manual trigger and the background phase can
// never diverge.
func FillPlanetTowardTemplate(ctx context.Context, db *sql.DB, universe string, userID, planetID int64, template game.BuildTemplate) error {
	if len(template) == 0 {
		return nil
	}
	planets := store.NewPlanetStore(db)
	techs, err := planets.Techs(ctx, userID)
	if err != nil {
		return err
	}
	return fillPlanet(ctx, store.NewQueueStore(db), planets, universe, userID, planetID, techs, template)
}

// fillPlanet queues building orders toward a template until the targets are met,
// the queue is full, or nothing further is affordable. `expected` tracks levels
// this pass has already queued so the plan never runs past a target;
// StartBuilding handles the actual queue chaining and slot cap.
func fillPlanet(ctx context.Context, queues *store.QueueStore, planets *store.PlanetStore, universe string, userID, planetID int64, techs map[string]int, template game.BuildTemplate) error {
	structs, err := planets.Structures(ctx, planetID)
	if err != nil {
		return err
	}
	// Seed the working levels with completed levels plus whatever is already
	// queued, so a plan never queues past a target (including levels the player
	// queued by hand). StartBuilding's own `cur + queued + 1` targeting then
	// stays in step with this map as we add orders.
	queued, err := queues.QueuedBuildings(ctx, planetID)
	if err != nil {
		return err
	}
	expected := make(map[string]int, len(structs))
	for code, level := range structs {
		expected[code] = level
	}
	for code, n := range queued {
		expected[code] += n
	}

	for {
		plan := game.BuildPlan(expected, techs, template)
		if len(plan) == 0 {
			return nil
		}
		progressed := false
		for _, code := range plan {
			err := queues.StartBuilding(ctx, universe, userID, planetID, code)
			if err == nil {
				expected[code]++
				progressed = true
				break
			}
			if builderSoftFailure(err) {
				continue // try the next candidate (e.g. cheaper or buildable now)
			}
			return err
		}
		if !progressed {
			return nil // queue full / not affordable right now — retry next check
		}
	}
}

// builderSoftFailure reports whether a failed enqueue should let the builder try
// the next candidate or planet rather than aborting the account.
func builderSoftFailure(err error) bool {
	return errors.Is(err, store.ErrInsufficientResources) ||
		errors.Is(err, store.ErrNoFields) ||
		errors.Is(err, store.ErrQueueFull) ||
		errors.Is(err, store.ErrQueueBusy) ||
		errors.Is(err, store.ErrPrerequisites) ||
		errors.Is(err, store.ErrOnVacation) ||
		errors.Is(err, store.ErrSanctioned)
}
