package ingest_test

import (
	"context"
	"testing"
	"time"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/dbtest"
	"github.com/fitter/svc/internal/ingest"
	"github.com/fitter/svc/internal/strava"
)

func mustCreateUser(t *testing.T, q *db.Queries) db.User {
	t.Helper()
	u, err := q.UpsertUserByStravaID(context.Background(), db.UpsertUserByStravaIDParams{
		ID:              "user-1",
		StravaAthleteID: 42,
		Name:            "Test Athlete",
		AvatarUrl:       "https://example.com/avatar.png",
		AccessToken:     "access-token",
		RefreshToken:    "refresh-token",
		TokenExpiresAt:  time.Now().Add(time.Hour).Unix(),
		CreatedAt:       time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func f64(v float64) *float64 { return &v }

func TestUpsertActivity_NilNullableFields(t *testing.T) {
	_, q := dbtest.New(t)
	u := mustCreateUser(t, q)

	a := strava.Activity{
		ID:                 1001,
		Name:               "Nil Fields Swim",
		Type:               "Swim",
		SportType:          "Swim",
		Distance:           1500,
		TotalElevationGain: 0,
		MovingTime:         1800,
		ElapsedTime:        1850,
		StartDate:          "2024-01-15T08:00:00Z",
		AverageSpeed:       0.83,
		MaxSpeed:           1.1,
		// Every nullable field left nil.
	}

	if err := ingest.UpsertActivity(context.Background(), q, u.ID, a); err != nil {
		t.Fatalf("UpsertActivity: %v", err)
	}

	row, err := q.GetLatestActivityStartDateForUser(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetLatestActivityStartDateForUser: %v", err)
	}
	if row == 0 {
		t.Fatal("expected a stored activity, got none")
	}

	activities, err := q.ListActivitiesForUserInRange(context.Background(), db.ListActivitiesForUserInRangeParams{
		UserID:      u.ID,
		StartDate:   0,
		StartDate_2: time.Now().Add(24 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("ListActivitiesForUserInRange: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	got := activities[0]

	if got.SportType != "Swim" {
		t.Errorf("SportType = %q, want %q", got.SportType, "Swim")
	}
	for name, nf := range map[string]struct {
		valid bool
	}{
		"AverageHeartrate": {got.AverageHeartrate.Valid},
		"MaxHeartrate":     {got.MaxHeartrate.Valid},
		"AverageWatts":     {got.AverageWatts.Valid},
		"MaxWatts":         {got.MaxWatts.Valid},
		"Kilojoules":       {got.Kilojoules.Valid},
		"Calories":         {got.Calories.Valid},
		"AverageCadence":   {got.AverageCadence.Valid},
		"SufferScore":      {got.SufferScore.Valid},
	} {
		if nf.valid {
			t.Errorf("%s: expected NULL (nil pointer on the strava.Activity), got Valid=true", name)
		}
	}
}

func TestUpsertActivity_WithValues(t *testing.T) {
	_, q := dbtest.New(t)
	u := mustCreateUser(t, q)

	a := strava.Activity{
		ID:                   2001,
		Name:                 "Hard Ride",
		Type:                 "Ride",
		SportType:            "Ride",
		Distance:             40000,
		TotalElevationGain:   350,
		MovingTime:           5400,
		ElapsedTime:          5600,
		StartDate:            "2024-02-01T09:00:00Z",
		AverageSpeed:         7.4,
		MaxSpeed:             15.2,
		HasHeartrate:         true,
		AverageHeartrate:     f64(145),
		MaxHeartrate:         f64(178),
		DeviceWatts:          true,
		AverageWatts:         f64(210),
		WeightedAverageWatts: f64(220),
		MaxWatts:             f64(480),
		Kilojoules:           f64(1134),
		Calories:             f64(1200),
		AverageCadence:       f64(88),
		SufferScore:          f64(95),
	}

	if err := ingest.UpsertActivity(context.Background(), q, u.ID, a); err != nil {
		t.Fatalf("UpsertActivity: %v", err)
	}

	activities, err := q.ListActivitiesForUserInRange(context.Background(), db.ListActivitiesForUserInRangeParams{
		UserID:      u.ID,
		StartDate:   0,
		StartDate_2: time.Now().Add(24 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("ListActivitiesForUserInRange: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	got := activities[0]

	if !got.AverageHeartrate.Valid || got.AverageHeartrate.Float64 != 145 {
		t.Errorf("AverageHeartrate = %+v, want Valid=true Float64=145", got.AverageHeartrate)
	}
	if !got.MaxHeartrate.Valid || got.MaxHeartrate.Float64 != 178 {
		t.Errorf("MaxHeartrate = %+v, want Valid=true Float64=178", got.MaxHeartrate)
	}
	if !got.AverageWatts.Valid || got.AverageWatts.Float64 != 210 {
		t.Errorf("AverageWatts = %+v, want Valid=true Float64=210", got.AverageWatts)
	}
	if !got.WeightedAverageWatts.Valid || got.WeightedAverageWatts.Float64 != 220 {
		t.Errorf("WeightedAverageWatts = %+v, want Valid=true Float64=220", got.WeightedAverageWatts)
	}
	if !got.MaxWatts.Valid || got.MaxWatts.Float64 != 480 {
		t.Errorf("MaxWatts = %+v, want Valid=true Float64=480", got.MaxWatts)
	}
	if !got.Kilojoules.Valid || got.Kilojoules.Float64 != 1134 {
		t.Errorf("Kilojoules = %+v, want Valid=true Float64=1134", got.Kilojoules)
	}
	if !got.Calories.Valid || got.Calories.Float64 != 1200 {
		t.Errorf("Calories = %+v, want Valid=true Float64=1200", got.Calories)
	}
	if !got.AverageCadence.Valid || got.AverageCadence.Float64 != 88 {
		t.Errorf("AverageCadence = %+v, want Valid=true Float64=88", got.AverageCadence)
	}
	if !got.SufferScore.Valid || got.SufferScore.Float64 != 95 {
		t.Errorf("SufferScore = %+v, want Valid=true Float64=95", got.SufferScore)
	}
	if got.DeviceWatts != 1 {
		t.Errorf("DeviceWatts = %d, want 1", got.DeviceWatts)
	}
}

// TestUpsertActivity_SameStravaIDUpdatesNotDuplicates confirms upserting the
// same strava_activity_id twice updates the one row rather than creating a
// second — this is the bug the schema restructure (Task 4) fixed by making
// strava_activity_id the primary key instead of a separate minted UUID.
func TestUpsertActivity_SameStravaIDUpdatesNotDuplicates(t *testing.T) {
	_, q := dbtest.New(t)
	u := mustCreateUser(t, q)

	base := strava.Activity{
		ID:           3001,
		Name:         "Evening Run",
		Type:         "Run",
		SportType:    "Run",
		Distance:     8000,
		MovingTime:   2400,
		ElapsedTime:  2450,
		StartDate:    "2024-03-01T18:00:00Z",
		AverageSpeed: 3.3,
		MaxSpeed:     4.5,
	}
	if err := ingest.UpsertActivity(context.Background(), q, u.ID, base); err != nil {
		t.Fatalf("first UpsertActivity: %v", err)
	}

	updated := base
	updated.Name = "Evening Run (renamed)"
	updated.Distance = 8200
	if err := ingest.UpsertActivity(context.Background(), q, u.ID, updated); err != nil {
		t.Fatalf("second UpsertActivity: %v", err)
	}

	activities, err := q.ListActivitiesForUserInRange(context.Background(), db.ListActivitiesForUserInRangeParams{
		UserID:      u.ID,
		StartDate:   0,
		StartDate_2: time.Now().Add(24 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("ListActivitiesForUserInRange: %v", err)
	}
	if count := len(activities); count != 1 {
		t.Fatalf("expected exactly 1 row after upserting the same strava_activity_id twice, got %d", count)
	}
	if activities[0].Name != "Evening Run (renamed)" {
		t.Errorf("Name = %q, want the updated name", activities[0].Name)
	}
	if activities[0].DistanceM != 8200 {
		t.Errorf("DistanceM = %v, want 8200 (the updated value)", activities[0].DistanceM)
	}
}

// TestUpsertActivity_CoalescePreservesCaloriesAcrossNilResync is the
// load-bearing COALESCE test: a detail-endpoint fetch (webhook path) sets
// calories; a later summary-endpoint resync (list/backfill path, which never
// returns calories) must not null it back out.
func TestUpsertActivity_CoalescePreservesCaloriesAcrossNilResync(t *testing.T) {
	_, q := dbtest.New(t)
	u := mustCreateUser(t, q)

	withCalories := strava.Activity{
		ID:           4001,
		Name:         "Lunch Run",
		Type:         "Run",
		SportType:    "Run",
		Distance:     10000,
		MovingTime:   3000,
		ElapsedTime:  3050,
		StartDate:    "2024-04-01T12:00:00Z",
		AverageSpeed: 3.3,
		MaxSpeed:     4.0,
		Calories:     f64(650),
	}
	if err := ingest.UpsertActivity(context.Background(), q, u.ID, withCalories); err != nil {
		t.Fatalf("first UpsertActivity (with calories): %v", err)
	}

	// Simulate a later list-endpoint resync of the same activity: identical
	// except Calories is nil, because /athlete/activities never returns it.
	resynced := withCalories
	resynced.Calories = nil
	resynced.Name = "Lunch Run (resynced)"
	if err := ingest.UpsertActivity(context.Background(), q, u.ID, resynced); err != nil {
		t.Fatalf("second UpsertActivity (resync, nil calories): %v", err)
	}

	activities, err := q.ListActivitiesForUserInRange(context.Background(), db.ListActivitiesForUserInRangeParams{
		UserID:      u.ID,
		StartDate:   0,
		StartDate_2: time.Now().Add(24 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("ListActivitiesForUserInRange: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 row, got %d", len(activities))
	}
	got := activities[0]
	if got.Name != "Lunch Run (resynced)" {
		t.Errorf("Name = %q, want the resynced name (non-nullable fields must still update)", got.Name)
	}
	if !got.Calories.Valid || got.Calories.Float64 != 650 {
		t.Errorf("Calories = %+v, want the ORIGINAL value (650) to survive the nil resync via COALESCE", got.Calories)
	}
}

func TestUpsertActivity_SportTypeFallsBackToType(t *testing.T) {
	_, q := dbtest.New(t)
	u := mustCreateUser(t, q)

	a := strava.Activity{
		ID:           5001,
		Name:         "Old Activity Predating SportType",
		Type:         "Run",
		SportType:    "", // very old activities predate this field
		Distance:     5000,
		MovingTime:   1500,
		ElapsedTime:  1520,
		StartDate:    "2015-01-01T06:00:00Z",
		AverageSpeed: 3.3,
		MaxSpeed:     4.0,
	}
	if err := ingest.UpsertActivity(context.Background(), q, u.ID, a); err != nil {
		t.Fatalf("UpsertActivity: %v", err)
	}

	activities, err := q.ListActivitiesForUserInRange(context.Background(), db.ListActivitiesForUserInRangeParams{
		UserID:      u.ID,
		StartDate:   0,
		StartDate_2: time.Now().Add(365 * 24 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("ListActivitiesForUserInRange: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	if activities[0].SportType != "Run" {
		t.Errorf("SportType = %q, want fallback to Type %q", activities[0].SportType, "Run")
	}
}

func TestUpsertActivity_BadStartDateReturnsError(t *testing.T) {
	_, q := dbtest.New(t)
	u := mustCreateUser(t, q)

	a := strava.Activity{
		ID:        6001,
		Name:      "Bad Date",
		Type:      "Run",
		SportType: "Run",
		StartDate: "not-a-real-date",
	}
	err := ingest.UpsertActivity(context.Background(), q, u.ID, a)
	if err == nil {
		t.Fatal("expected an error for an unparseable start_date, got nil")
	}
}
