package service

import (
	"testing"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/metrics"
)

func TestPooledContributions(t *testing.T) {
	members := []db.ListCampaignMembersRow{
		{UserID: "u1", UserName: "Sam"},
		{UserID: "u2", UserName: "Priya"},
		{UserID: "u3", UserName: "Ana"},
	}
	acts := []metrics.Activity{
		{UserID: "u1", DistanceM: 10_000},
		{UserID: "u2", DistanceM: 30_000},
		{UserID: "u2", DistanceM: 5_000},
		{UserID: "gone", DistanceM: 99_000}, // left the campaign: not a member, not counted
	}

	dist, _ := metrics.Lookup(metrics.Distance)
	got := pooledContributions(dist, acts, members)
	want := []struct {
		id string
		v  float64
	}{{"u2", 35_000}, {"u1", 10_000}, {"u3", 0}}
	if len(got) != len(want) {
		t.Fatalf("got %d contributions, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].UserId != w.id || got[i].Value != w.v {
			t.Errorf("contribution %d = %s/%v, want %s/%v", i, got[i].UserId, got[i].Value, w.id, w.v)
		}
	}

	// An average has no per-member share.
	avg, _ := metrics.Lookup(metrics.AverageSpeed)
	if got := pooledContributions(avg, acts, members); got != nil {
		t.Errorf("average metric returned %d contributions, want none", len(got))
	}
}
