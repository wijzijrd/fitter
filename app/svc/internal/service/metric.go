package service

import (
	"database/sql"
	"fmt"
	"time"

	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/sports"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// metricKeyByProto is the single source of truth for the proto
// ActivityMetric <-> internal/metrics.Key mapping. protoByMetricKey (its
// inverse) is derived from it so the two can never drift out of sync.
var metricKeyByProto = map[fitterv1.ActivityMetric]metrics.Key{
	fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE:               metrics.Distance,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_ELEVATION_GAIN:         metrics.ElevationGain,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_MOVING_TIME:            metrics.MovingTime,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_ACTIVITY_COUNT:         metrics.ActivityCount,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_ELAPSED_TIME:           metrics.ElapsedTime,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_AVERAGE_SPEED:          metrics.AverageSpeed,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_MAX_SPEED:              metrics.MaxSpeed,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_PACE:                   metrics.Pace,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_AVERAGE_HEARTRATE:      metrics.AverageHeartrate,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_MAX_HEARTRATE:          metrics.MaxHeartrate,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_AVERAGE_WATTS:          metrics.AverageWatts,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_WEIGHTED_AVERAGE_WATTS: metrics.WeightedAverageWatts,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_MAX_WATTS:              metrics.MaxWatts,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_KILOJOULES:             metrics.Kilojoules,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_CALORIES:               metrics.Calories,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_AVERAGE_CADENCE:        metrics.AverageCadence,
	fitterv1.ActivityMetric_ACTIVITY_METRIC_SUFFER_SCORE:           metrics.SufferScore,
}

var protoByMetricKey = func() map[metrics.Key]fitterv1.ActivityMetric {
	m := make(map[metrics.Key]fitterv1.ActivityMetric, len(metricKeyByProto))
	for p, k := range metricKeyByProto {
		m[k] = p
	}
	return m
}()

// metricToKey maps a proto ActivityMetric to its internal/metrics.Key,
// rejecting UNSPECIFIED and any unrecognized value.
func metricToKey(m fitterv1.ActivityMetric) (metrics.Key, error) {
	k, ok := metricKeyByProto[m]
	if !ok {
		return "", fmt.Errorf("unspecified or unknown activity metric")
	}
	return k, nil
}

// keyToMetric is metricToKey's inverse. An unrecognized key (should not
// happen for DB rows written through this package) maps to UNSPECIFIED
// rather than panicking.
func keyToMetric(k metrics.Key) fitterv1.ActivityMetric {
	if m, ok := protoByMetricKey[k]; ok {
		return m
	}
	return fitterv1.ActivityMetric_ACTIVITY_METRIC_UNSPECIFIED
}

// scopeToDB maps a proto ClauseScope to the goal_clauses.scope column value
// ("aggregate"/"single"), rejecting UNSPECIFIED and any unrecognized value.
func scopeToDB(s fitterv1.ClauseScope) (string, error) {
	switch s {
	case fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE:
		return "aggregate", nil
	case fitterv1.ClauseScope_CLAUSE_SCOPE_SINGLE:
		return "single", nil
	default:
		return "", fmt.Errorf("unspecified or unknown clause scope")
	}
}

// scopeFromDB is scopeToDB's inverse.
func scopeFromDB(s string) fitterv1.ClauseScope {
	switch s {
	case "aggregate":
		return fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE
	case "single":
		return fitterv1.ClauseScope_CLAUSE_SCOPE_SINGLE
	default:
		return fitterv1.ClauseScope_CLAUSE_SCOPE_UNSPECIFIED
	}
}

// comparatorToDB maps a proto Comparator to the goal_clauses.comparator
// column value ("gte"/"lte"), rejecting UNSPECIFIED and any unrecognized
// value.
func comparatorToDB(c fitterv1.Comparator) (string, error) {
	switch c {
	case fitterv1.Comparator_COMPARATOR_GTE:
		return "gte", nil
	case fitterv1.Comparator_COMPARATOR_LTE:
		return "lte", nil
	default:
		return "", fmt.Errorf("unspecified or unknown comparator")
	}
}

// comparatorFromDB is comparatorToDB's inverse.
func comparatorFromDB(c string) fitterv1.Comparator {
	switch c {
	case "gte":
		return fitterv1.Comparator_COMPARATOR_GTE
	case "lte":
		return fitterv1.Comparator_COMPARATOR_LTE
	default:
		return fitterv1.Comparator_COMPARATOR_UNSPECIFIED
	}
}

// unitToProto maps an internal/metrics.Unit to its proto MetricUnit, for
// CatalogService.ListMetrics.
func unitToProto(u metrics.Unit) fitterv1.MetricUnit {
	switch u {
	case metrics.UnitMeters:
		return fitterv1.MetricUnit_METRIC_UNIT_METERS
	case metrics.UnitSeconds:
		return fitterv1.MetricUnit_METRIC_UNIT_SECONDS
	case metrics.UnitMetersPerSecond:
		return fitterv1.MetricUnit_METRIC_UNIT_METERS_PER_SECOND
	case metrics.UnitSecondsPerKm:
		return fitterv1.MetricUnit_METRIC_UNIT_SECONDS_PER_KM
	case metrics.UnitBPM:
		return fitterv1.MetricUnit_METRIC_UNIT_BPM
	case metrics.UnitWatts:
		return fitterv1.MetricUnit_METRIC_UNIT_WATTS
	case metrics.UnitKilojoules:
		return fitterv1.MetricUnit_METRIC_UNIT_KILOJOULES
	case metrics.UnitKilocalories:
		return fitterv1.MetricUnit_METRIC_UNIT_KILOCALORIES
	case metrics.UnitRPM:
		return fitterv1.MetricUnit_METRIC_UNIT_RPM
	case metrics.UnitCount:
		return fitterv1.MetricUnit_METRIC_UNIT_COUNT
	case metrics.UnitScore:
		return fitterv1.MetricUnit_METRIC_UNIT_SCORE
	default:
		return fitterv1.MetricUnit_METRIC_UNIT_UNSPECIFIED
	}
}

// aggregationToProto maps an internal/metrics.Aggregation to its proto
// MetricAggregation, for CatalogService.ListMetrics.
func aggregationToProto(a metrics.Aggregation) fitterv1.MetricAggregation {
	switch a {
	case metrics.AggSum:
		return fitterv1.MetricAggregation_METRIC_AGGREGATION_SUM
	case metrics.AggWeightedAvg:
		return fitterv1.MetricAggregation_METRIC_AGGREGATION_AVERAGE
	case metrics.AggMax:
		return fitterv1.MetricAggregation_METRIC_AGGREGATION_MAX
	case metrics.AggCount:
		return fitterv1.MetricAggregation_METRIC_AGGREGATION_COUNT
	case metrics.AggDerived:
		return fitterv1.MetricAggregation_METRIC_AGGREGATION_DERIVED
	default:
		return fitterv1.MetricAggregation_METRIC_AGGREGATION_UNSPECIFIED
	}
}

// sportSelectorToDB maps a proto SportSelector to the goals.sport_group /
// goals.sport_type column pair. An unset selector (nil, or neither oneof
// branch present) maps to (NULL, NULL), meaning "all sports". A group key
// that doesn't name a known curated group (sports.ValidGroup) is rejected
// rather than silently stored — an unrecognized group would otherwise
// become a goal that quietly matches nothing.
func sportSelectorToDB(sel *fitterv1.SportSelector) (sportGroup, sportType sql.NullString, err error) {
	if sel == nil {
		return sql.NullString{}, sql.NullString{}, nil
	}
	switch v := sel.GetSelector().(type) {
	case *fitterv1.SportSelector_Group:
		if !sports.ValidGroup(v.Group) {
			return sql.NullString{}, sql.NullString{}, fmt.Errorf("unknown sport group %q", v.Group)
		}
		return sql.NullString{String: v.Group, Valid: true}, sql.NullString{}, nil
	case *fitterv1.SportSelector_SportType:
		return sql.NullString{}, sql.NullString{String: v.SportType, Valid: true}, nil
	default:
		return sql.NullString{}, sql.NullString{}, nil
	}
}

// sportSelectorFromDB is sportSelectorToDB's inverse.
func sportSelectorFromDB(sportGroup, sportType sql.NullString) *fitterv1.SportSelector {
	switch {
	case sportGroup.Valid:
		return &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_Group{Group: sportGroup.String}}
	case sportType.Valid:
		return &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_SportType{SportType: sportType.String}}
	default:
		return &fitterv1.SportSelector{}
	}
}

// nullFloatPtr converts a sql.NullFloat64 to *float64, nil when !Valid —
// used to populate optional proto double fields from nullable DB columns.
func nullFloatPtr(f sql.NullFloat64) *float64 {
	if !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

func unixToTimestamp(sec int64) *timestamppb.Timestamp {
	return timestamppb.New(time.Unix(sec, 0).UTC())
}

func timestampToUnix(ts *timestamppb.Timestamp) int64 {
	if ts == nil {
		return 0
	}
	return ts.AsTime().Unix()
}
