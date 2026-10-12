package game

import "testing"

func TestLossResources(t *testing.T) {
	// Jackal: 250 Iron + 75 Hydrogen; Laser Turret: 2000 Iron + 200 Lutinum.
	r := LossResources(map[string]int64{"jackal": 2, "heavy_laser": 1})
	if r.Metal != 2500 || r.Lutinum != 200 || r.Water != 0 || r.Hydrogen != 150 {
		t.Errorf("LossResources = %+v, want {2500 200 0 150}", r)
	}
}

func TestRecyclerCargo(t *testing.T) {
	if got := RecyclerCargo(map[string]int64{"recycler": 2, "jackal": 5}, 0); got != 20000 {
		t.Errorf("RecyclerCargo = %d, want 20000", got)
	}
	if got := RecyclerCargo(map[string]int64{"recycler": 2}, 5); got != 30000 {
		t.Errorf("RecyclerCargo +cargo tech = %d, want 30000", got)
	}
	if got := RecyclerCargo(map[string]int64{"jackal": 5}, 0); got != 0 {
		t.Errorf("RecyclerCargo without recyclers = %d, want 0", got)
	}
}

func TestRecoverDebris(t *testing.T) {
	// Both sides wiped: Laser Turret (2000 Iron, 200 Lutinum) + Jackal
	// (250 Iron, 75 Hydrogen). At 20% that is 450 Iron, 40 Lutinum, 15 Hydrogen.
	got := RecoverDebris(
		map[string]int64{"jackal": 1},
		map[string]int64{"heavy_laser": 1},
		0, 100000,
	).Rounded()
	if got.Metal != 450 || got.Lutinum != 40 || got.Water != 0 || got.Hydrogen != 15 {
		t.Errorf("RecoverDebris = %+v, want {450 40 0 15}", got)
	}

	// Recycling Technology raises the share: 30% of Laser Turret = 600 / 60.
	got = RecoverDebris(nil, map[string]int64{"heavy_laser": 1}, 10, 100000).Rounded()
	if got.Metal != 600 || got.Lutinum != 60 {
		t.Errorf("RecoverDebris lvl10 = %+v, want metal 600 lutinum 60", got)
	}

	// No recyclers recover nothing.
	if got := RecoverDebris(map[string]int64{"jackal": 1}, nil, 5, 0); got.Total() != 0 {
		t.Errorf("RecoverDebris without cargo = %+v, want zero", got)
	}
}

func TestRecoverDebrisCappedByCargo(t *testing.T) {
	// 20% of one Rocket Launcher (15000 Iron, 2000 Lutinum, 2000 Water) is
	// 3800 units of debris; a 1000-unit hold can only carry the overflow-scaled
	// share.
	got := RecoverDebris(nil, map[string]int64{"rocket_launcher": 1}, 0, 1000).Rounded()
	if got.Total() > 1000 || got.Total() < 998 {
		t.Errorf("capped total = %v, want ~1000 (never above)", got.Total())
	}
	// Proportions preserved (15000:2000:2000 ≈ 789:105:105 after rounding).
	if got.Metal != 789 || got.Lutinum != 105 || got.Water != 105 {
		t.Errorf("capped breakdown = %+v", got)
	}
}

func TestAggregateBattleRecycled(t *testing.T) {
	reports := []CombatReport{
		{Role: "attacker", RecoveredBy: "attacker", Recovered: Loot{Metal: 50}},
		{Role: "attacker", RecoveredBy: "defender", Recovered: Loot{Metal: 100}},
		{Role: "defender", RecoveredBy: "defender", Recovered: Loot{Hydrogen: 30}},
		{Role: "defender", RecoveredBy: "attacker", Recovered: Loot{Hydrogen: 70}},
		{Role: "attacker"},
	}
	bb := AggregateBattle(reports)
	res := resourceByName(bb.Recycled)
	if res["Iron"].Plundered != 50 || res["Iron"].Lost != 100 {
		t.Errorf("Iron recycled gained/lost = %v/%v, want 50/100", res["Iron"].Plundered, res["Iron"].Lost)
	}
	if res["Hydrogen"].Plundered != 30 || res["Hydrogen"].Lost != 70 {
		t.Errorf("Hydrogen recycled gained/lost = %v/%v, want 30/70", res["Hydrogen"].Plundered, res["Hydrogen"].Lost)
	}
}
