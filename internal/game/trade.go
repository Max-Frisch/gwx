package game

import (
	"math"
	"time"
)

// Player-to-player trading schedule. Fees are a fraction of the traded goods,
// kept as named constants so the balance is easy to tune. The reference charges
// an up-front offer fee plus a completion fee on each side.
const (
	TradeOfferFee  = 0.02  // charged to the offer's creator when they post it
	TradeSellerFee = 0.015 // charged to the creator when an offer is accepted
	TradeBuyerFee  = 0.015 // charged to the accepter when an offer is accepted

	// Public offers live seven days, private ones six hours (reference table).
	TradePublicDuration  = 7 * 24 * time.Hour
	TradePrivateDuration = 6 * time.Hour

	// TradeStorageDuration is how long delivered goods wait in the trade storage
	// before they are forfeited. gwx keeps storage separate from the planet's
	// capped/protected resource pool, exactly so the protected floor does not
	// silently swallow a delivery.
	TradeStorageDuration = 7 * 24 * time.Hour
)

// Total is the combined amount of all four resources.
func (r Resources) Total() float64 {
	return r.Metal + r.Lutinum + r.Water + r.Hydrogen
}

// IsZero reports whether the basket holds no resources at all.
func (r Resources) IsZero() bool { return r.Total() == 0 }

// NonNegative reports whether no resource in the basket is negative.
func (r Resources) NonNegative() bool {
	return r.Metal >= 0 && r.Lutinum >= 0 && r.Water >= 0 && r.Hydrogen >= 0
}

// FeeFloor returns the part of a basket withheld as a fee at the given rate,
// floored to whole units per resource so the ledger never rounds a fee up.
func FeeFloor(r Resources, pct float64) Resources {
	f := func(v float64) float64 { return math.Floor(v * pct) }
	return Resources{Metal: f(r.Metal), Lutinum: f(r.Lutinum), Water: f(r.Water), Hydrogen: f(r.Hydrogen)}
}

// NetAfterFee returns the basket that remains once the fee is withheld.
func NetAfterFee(r Resources, pct float64) Resources {
	fee := FeeFloor(r, pct)
	return Resources{
		Metal:    r.Metal - fee.Metal,
		Lutinum:  r.Lutinum - fee.Lutinum,
		Water:    r.Water - fee.Water,
		Hydrogen: r.Hydrogen - fee.Hydrogen,
	}
}

// CanAfford reports whether have covers cost for every resource.
func CanAfford(have, cost Resources) bool {
	return have.Metal >= cost.Metal && have.Lutinum >= cost.Lutinum &&
		have.Water >= cost.Water && have.Hydrogen >= cost.Hydrogen
}

// Exchange is the settled result of accepting a barter offer.
type Exchange struct {
	AccepterPays Resources // the offer's wanted basket, debited from the accepter
	AccepterGets Resources // the offer's given basket, net of the seller fee
	CreatorGets  Resources // the wanted basket, net of the buyer fee
}

// Settle resolves a barter offer (give ↔ want) into what each side hands over
// and receives, applying the completion fees. It is pure so the exchange math is
// unit-testable without a database.
func Settle(give, want Resources) Exchange {
	return Exchange{
		AccepterPays: want,
		AccepterGets: NetAfterFee(give, TradeSellerFee),
		CreatorGets:  NetAfterFee(want, TradeBuyerFee),
	}
}
