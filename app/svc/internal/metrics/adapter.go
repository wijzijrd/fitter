package metrics

import (
	"database/sql"

	"github.com/fitter/svc/internal/db"
)

// nullFloat converts a sql.NullFloat64 to *float64, nil when !Valid.
func nullFloat(f sql.NullFloat64) *float64 {
	if !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

// FromDB converts a db.Activity (the sqlc-generated row type) into the
// engine's own narrow Activity view. This is the only file in this package
// allowed to import internal/db — see the package doc in metrics.go for why
// the rest of the package deliberately avoids that dependency.
func FromDB(a db.Activity) Activity {
	return Activity{
		StravaActivityID: a.StravaActivityID,
		UserID:           a.UserID,
		Type:             a.Type,
		SportType:        a.SportType,
		StartDate:        a.StartDate,
		DistanceM:        a.DistanceM,
		ElevationM:       a.ElevationM,
		MovingTimeS:      float64(a.MovingTimeS),
		ElapsedTimeS:     float64(a.ElapsedTimeS),
		AverageSpeedMS:   a.AverageSpeedMs,
		MaxSpeedMS:       a.MaxSpeedMs,
		AverageHeartrate: nullFloat(a.AverageHeartrate),
		MaxHeartrate:     nullFloat(a.MaxHeartrate),
		AverageWatts:     nullFloat(a.AverageWatts),
		WeightedAvgWatts: nullFloat(a.WeightedAverageWatts),
		MaxWatts:         nullFloat(a.MaxWatts),
		Kilojoules:       nullFloat(a.Kilojoules),
		Calories:         nullFloat(a.Calories),
		AverageCadence:   nullFloat(a.AverageCadence),
		SufferScore:      nullFloat(a.SufferScore),
	}
}
