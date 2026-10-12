package game

import (
	"math"
	"testing"
)

func TestFeeFloorAndNet(t *testing.T) {
	r := Resources{Metal: 1000, Lutinum: 500, Water: 25, Hydrogen: 3}
	fee := FeeFloor(r, 0.02)
	if fee.Metal != 20 || fee.Lutinum != 10 || fee.Water != 0 || fee.Hydrogen != 0 {
		t.Fatalf("FeeFloor = %+v", fee)
	}
	net := NetAfterFee(r, 0.02)
	if net.Metal != 980 || net.Lutinum != 490 || net.Water != 25 || net.Hydrogen != 3 {
		t.Fatalf("NetAfterFee = %+v", net)
	}
}

func TestSettleAppliesBothFees(t *testing.T) {
	// Give 1000 iron, want 500 lutinum. Seller fee 1.5% on the give,
	// buyer fee 1.5% on the want, each floored per resource.
	ex := Settle(Resources{Metal: 1000}, Resources{Lutinum: 500})
	if ex.AccepterPays.Lutinum != 500 {
		t.Errorf("accepter pays = %v, want 500 lutinum", ex.AccepterPays.Lutinum)
	}
	if ex.AccepterGets.Metal != 985 {
		t.Errorf("accepter gets = %v, want 985 iron (1000 - 1.5%%)", ex.AccepterGets.Metal)
	}
	if ex.CreatorGets.Lutinum != 493 {
		t.Errorf("creator gets = %v, want 493 lutinum (500 - floor(1.5%%))", ex.CreatorGets.Lutinum)
	}
}

func TestCanAffordInsufficientPath(t *testing.T) {
	have := Resources{Metal: 100, Lutinum: 50, Water: 10, Hydrogen: 0}
	if !CanAfford(have, Resources{Metal: 100, Lutinum: 50, Water: 10}) {
		t.Error("exact stock should be affordable")
	}
	if CanAfford(have, Resources{Metal: 101}) {
		t.Error("over-stock metal must be rejected")
	}
	if CanAfford(have, Resources{Hydrogen: 1}) {
		t.Error("missing hydrogen must be rejected")
	}
}

func TestSettleOfEscrowConservesFee(t *testing.T) {
	give := Resources{Metal: 10000, Lutinum: 2000}
	ex := Settle(give, Resources{Water: 1000})
	// Accepter receives the give minus the seller fee; nothing is created.
	if ex.AccepterGets.Total() >= give.Total() {
		t.Errorf("net %v must be below gross %v", ex.AccepterGets.Total(), give.Total())
	}
	if math.Abs((give.Total()-ex.AccepterGets.Total())-FeeFloor(give, TradeSellerFee).Total()) > 1e-9 {
		t.Error("withheld fee should equal the difference")
	}
}
