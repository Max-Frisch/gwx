package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"gwx/internal/engine"
	"gwx/internal/game"
	"gwx/internal/store"
)

// TestNPCBrainRaids wires the opt-in offense: an NPC holding a probe and
// warships, with an eligible target in the same universe, eventually scouts and
// launches an ATTACK. The combat report on arrival is the durable signal; the
// espionage report it acted on is consumed.
func TestNPCBrainRaids(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.engine.SetNPCRaid(engine.NPCRaidConfig{Enabled: true, ChanceScale: 1, MaxPerTick: 5})

	h.register(t, "raid-target") // eligible human target in the same universe

	npcs := store.NewNPCStore(h.db)
	userID, homeID, err := npcs.Create(ctx, h.universe, game.DefaultNPCProfile())
	if err != nil {
		t.Fatalf("create npc: %v", err)
	}
	h.seed(t, homeID, seedAmount, seedAmount, seedAmount, seedAmount)
	h.giveUnit(t, homeID, "ship", "spy_probe", 2)
	h.giveUnit(t, homeID, "ship", "jackal", 20)

	waitFor(t, "npc raid attack", func() bool {
		h.forceNPCDecision(t, userID)
		return h.queryInt(t, `SELECT COUNT(*) FROM reports WHERE user_id = $1 AND kind = 'combat'`, userID) > 0
	})

	// The spy reports that fed the raid are consumed (each decision deletes the
	// pending espionage report, attack or not), so the inbox never fills up.
	waitFor(t, "espionage reports consumed", func() bool {
		h.forceNPCDecision(t, userID)
		return h.queryInt(t, `SELECT COUNT(*) FROM reports WHERE user_id = $1 AND kind = 'espionage'`, userID) == 0
	})
}

// TestNPCBrainNoRaidWhenDisabled pins the default: with raiding off, the brain
// never scouts or attacks, no matter how many decisions it is given.
func TestNPCBrainNoRaidWhenDisabled(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	h.register(t, "raid-off-target")

	npcs := store.NewNPCStore(h.db)
	userID, homeID, err := npcs.Create(ctx, h.universe, game.DefaultNPCProfile())
	if err != nil {
		t.Fatalf("create npc: %v", err)
	}
	h.seed(t, homeID, seedAmount, seedAmount, seedAmount, seedAmount)
	h.giveUnit(t, homeID, "ship", "spy_probe", 2)
	h.giveUnit(t, homeID, "ship", "jackal", 20)

	for i := 0; i < 200; i++ {
		h.forceNPCDecision(t, userID)
		time.Sleep(3 * time.Millisecond)
	}
	if n := h.queryInt(t, `SELECT COUNT(*) FROM fleets WHERE user_id = $1 AND mission = 'ATTACK'`, userID); n != 0 {
		t.Fatalf("ATTACK fleet created while raiding disabled: %d", n)
	}
	if n := h.queryInt(t, `SELECT COUNT(*) FROM fleets WHERE user_id = $1 AND mission = 'ESPIONAGE'`, userID); n != 0 {
		t.Fatalf("ESPIONAGE fleet created while raiding disabled: %d", n)
	}
}

// TestRandomRaidTargetExcludesCallerAndVacation checks the target pool: the
// caller is never targeted, and a vacation-active owner is skipped.
func TestRandomRaidTargetExcludesCallerAndVacation(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	a := h.register(t, "raid-store-a")
	b := h.register(t, "raid-store-b")

	npcs := store.NewNPCStore(h.db)
	bCoord := h.at(t, b.home)

	planetID, g, s, pos, ok, err := npcs.RandomRaidTarget(ctx, h.universe, a.id)
	if err != nil {
		t.Fatalf("RandomRaidTarget: %v", err)
	}
	if !ok {
		t.Fatal("expected an eligible target (account b)")
	}
	if planetID != b.home || g != bCoord.g || s != bCoord.s || pos != bCoord.pos {
		t.Errorf("target = %d (%d:%d:%d), want account b home %d (%d:%d:%d)",
			planetID, g, s, pos, b.home, bCoord.g, bCoord.s, bCoord.pos)
	}

	// Put the only other account on vacation: no eligible target remains.
	if _, err := h.db.Exec(`
		UPDATE users SET vacation_since = NOW(), vacation_until = NOW() + INTERVAL '1 day'
		WHERE id = $1
	`, b.id); err != nil {
		t.Fatalf("vacation b: %v", err)
	}
	if _, _, _, _, ok, err = npcs.RandomRaidTarget(ctx, h.universe, a.id); err != nil {
		t.Fatalf("RandomRaidTarget (vacation): %v", err)
	}
	if ok {
		t.Error("target found though the only other account is on vacation")
	}
}

// TestReportStoreConsumeRemovesMessage checks that consuming an espionage
// report deletes both the report and the inbox message pointing at it.
func TestReportStoreConsumeRemovesMessage(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	u := h.register(t, "consume-user")
	var universeUUID string
	if err := h.db.QueryRow(`SELECT id::text FROM universes WHERE code_name = $1`, h.universe).
		Scan(&universeUUID); err != nil {
		t.Fatalf("universe uuid: %v", err)
	}

	reports := store.NewReportStore(h.db)
	if err := reports.Insert(ctx, universeUUID, u.id, "espionage", "Espionage: 1:1:1",
		map[string]any{"target": "1:1:1"}); err != nil {
		t.Fatalf("insert report: %v", err)
	}
	rep, err := reports.LatestByKind(ctx, u.id, "espionage")
	if err != nil {
		t.Fatalf("LatestByKind: %v", err)
	}
	if _, err := h.db.Exec(`
		INSERT INTO messages (universe_id, user_id, category, kind, title, body, report_id)
		VALUES ($1, $2, 'report', 'spy', 'Espionage: 1:1:1', '', $3)
	`, universeUUID, u.id, rep.ID); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	if err := reports.Consume(ctx, u.id, rep.ID); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if n := h.queryInt(t, `SELECT COUNT(*) FROM reports WHERE id = $1`, rep.ID); n != 0 {
		t.Errorf("report survived Consume: %d", n)
	}
	if n := h.queryInt(t, `SELECT COUNT(*) FROM messages WHERE report_id = $1`, rep.ID); n != 0 {
		t.Errorf("message survived Consume: %d", n)
	}
	if _, err := reports.LatestByKind(ctx, u.id, "espionage"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("LatestByKind after Consume = %v, want ErrNotFound", err)
	}
}
