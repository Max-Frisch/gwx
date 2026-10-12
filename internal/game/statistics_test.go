package game

import "testing"

func TestUnitValue(t *testing.T) {
	// Spy Probe: 50 Lutinum + 10 Hydrogen.
	if got := UnitValue("spy_probe", 3); got != 180 {
		t.Errorf("UnitValue(spy_probe,3) = %v, want 180", got)
	}
	if got := UnitValue("spy_probe", 0); got != 0 {
		t.Errorf("zero count = %v, want 0", got)
	}
	if got := UnitValue("does_not_exist", 5); got != 0 {
		t.Errorf("unknown code = %v, want 0", got)
	}
}

func TestLossValue(t *testing.T) {
	units := map[string]int64{"cougar": 1, "heavy_laser": 2}
	// Cougar 4100 + 2 × Laser Turret 2200 = 8500.
	if got := LossValue(units); got != 8500 {
		t.Errorf("LossValue = %v, want 8500", got)
	}
}

func TestCombatReportValues(t *testing.T) {
	rep := CombatReport{
		AttackerLosses: map[string]int64{"cougar": 1},
		DefenderLosses: map[string]int64{"heavy_laser": 2},
		Loot:           Loot{Metal: 1000, Hydrogen: 500},
	}
	// Cougar 4100 + 2 × Laser Turret 2200 = 8500.
	if got := rep.DestroyedValue(); got != 8500 {
		t.Errorf("DestroyedValue = %v, want 8500", got)
	}
	if got := rep.LootValue(); got != 1500 {
		t.Errorf("LootValue = %v, want 1500", got)
	}
}

func TestSimulateRecordsPerUnitLosses(t *testing.T) {
	res := Simulate(map[string]int64{"longeagle_x": 50}, map[string]int64{"light_laser": 5}, 0, 0, 6)
	if got := res.DefenderLosses["light_laser"]; got != 5 {
		t.Errorf("defenderLosses[light_laser] = %d, want 5", got)
	}
	if len(res.AttackerLosses) != 0 {
		t.Errorf("attackerLosses = %v, want empty", res.AttackerLosses)
	}
	if got := res.DefenderLost; got != 5 {
		t.Errorf("DefenderLost = %d, want 5", got)
	}
}

func TestAggregateBattle(t *testing.T) {
	reports := []CombatReport{
		{Role: "attacker", AttackerLosses: map[string]int64{"cougar": 1}, DefenderLosses: map[string]int64{"heavy_laser": 2}, Loot: Loot{Metal: 1000}},
		{Role: "defender", AttackerLosses: map[string]int64{"cougar": 5}, DefenderLosses: map[string]int64{"light_laser": 1}, Loot: Loot{Hydrogen: 500}},
		{Role: "attacker", AttackerLosses: map[string]int64{"longeagle_x": 2}},
		{Role: "attacker"},
	}
	bb := AggregateBattle(reports)

	if bb.Attacked != 3 || bb.Defended != 1 {
		t.Errorf("attacked/defended = %d/%d, want 3/1", bb.Attacked, bb.Defended)
	}
	if bb.Won != 2 || bb.Lost != 1 || bb.Tied != 1 {
		t.Errorf("won/lost/tied = %d/%d/%d, want 2/1/1", bb.Won, bb.Lost, bb.Tied)
	}

	ships := unitByCode(bb.Ships)
	if got := ships["cougar"]; got.Destroyed != 5 || got.Lost != 1 {
		t.Errorf("cougar destroyed/lost = %d/%d, want 5/1", got.Destroyed, got.Lost)
	}
	if got := ships["longeagle_x"]; got.Lost != 2 || got.Destroyed != 0 {
		t.Errorf("longeagle_x destroyed/lost = %d/%d, want 0/2", got.Destroyed, got.Lost)
	}
	if got := ships["cougar"].DestroyedValue(); got != 5*4100 {
		t.Errorf("cougar destroyed value = %v, want %v", got, 5*4100)
	}

	defense := unitByCode(bb.Defense)
	if got := defense["heavy_laser"]; got.Destroyed != 2 {
		t.Errorf("heavy_laser destroyed = %d, want 2", got.Destroyed)
	}
	if got := defense["light_laser"]; got.Lost != 1 {
		t.Errorf("light_laser lost = %d, want 1", got.Lost)
	}

	res := resourceByName(bb.Resources)
	if res["Iron"].Plundered != 1000 {
		t.Errorf("Iron plundered = %v, want 1000", res["Iron"].Plundered)
	}
	if res["Hydrogen"].Lost != 500 {
		t.Errorf("Hydrogen lost = %v, want 500", res["Hydrogen"].Lost)
	}

	// Every catalog ship appears, even at zero.
	if len(bb.Ships) != len(DefsOf(KindShip)) {
		t.Errorf("ships rows = %d, want %d", len(bb.Ships), len(DefsOf(KindShip)))
	}
}

func TestAggregateFarming(t *testing.T) {
	reports := []CombatReport{
		{Role: "attacker", Loot: Loot{Metal: 1000}},
		{Role: "defender", Loot: Loot{Metal: 9999}},
		{Role: "attacker"},
	}
	f := AggregateFarming(reports)
	if f.Raids != 2 {
		t.Errorf("raids = %d, want 2", f.Raids)
	}
	if f.Total != 1000 {
		t.Errorf("total = %v, want 1000", f.Total)
	}
	if f.AverageHaul != 500 {
		t.Errorf("average = %v, want 500", f.AverageHaul)
	}
}

func TestAggregateResources(t *testing.T) {
	reports := []CombatReport{
		{Role: "attacker", Loot: Loot{Metal: 1000, Water: 200}},
		{Role: "defender", Loot: Loot{Hydrogen: 500}},
	}
	rb := AggregateResources(reports)
	if rb.Plundered != 1200 {
		t.Errorf("plundered = %v, want 1200", rb.Plundered)
	}
	if rb.Lost != 500 {
		t.Errorf("lost = %v, want 500", rb.Lost)
	}
	res := resourceByName(rb.Resources)
	if res["Iron"].Plundered != 1000 || res["Water"].Plundered != 200 {
		t.Errorf("plundered breakdown = %+v", rb.Resources)
	}
	if res["Hydrogen"].Lost != 500 || res["Iron"].Balance() != 1000 {
		t.Errorf("lost/balance = %+v", rb.Resources)
	}
}

func unitByCode(rows []UnitStat) map[string]UnitStat {
	m := make(map[string]UnitStat, len(rows))
	for _, r := range rows {
		m[r.Code] = r
	}
	return m
}

func resourceByName(rows []ResourceStat) map[string]ResourceStat {
	m := make(map[string]ResourceStat, len(rows))
	for _, r := range rows {
		m[r.Name] = r
	}
	return m
}
