package gungnir

import "testing"

func TestUsageAddsWhatWasBilledAndKeepsItApartFromTheCost(t *testing.T) {
	a := Usage{CostUSD: 1.5, BilledUSD: 0.5, Turns: 2}
	b := Usage{CostUSD: 0.25, BilledUSD: 0.25, Turns: 1}
	got := a.Add(b)
	if got.CostUSD != 1.75 || got.BilledUSD != 0.75 || got.Turns != 3 {
		t.Errorf("Add() = %+v; want cost 1.75, billed 0.75, 3 turns", got)
	}
	if (Usage{CostUSD: 3}).BilledUSD != 0 {
		t.Error("a usage that says nothing of billing must not count its cost as billed")
	}
}
