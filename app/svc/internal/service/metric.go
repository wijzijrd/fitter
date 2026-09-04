package service

import (
	"fmt"
	"time"

	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/db"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	metricDistance  = "distance"
	metricElevation = "elevation"
	metricTime      = "time"
	metricCount     = "count"
)

func metricToDB(m fitterv1.ActivityMetric) (string, error) {
	switch m {
	case fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE:
		return metricDistance, nil
	case fitterv1.ActivityMetric_ACTIVITY_METRIC_ELEVATION:
		return metricElevation, nil
	case fitterv1.ActivityMetric_ACTIVITY_METRIC_TIME:
		return metricTime, nil
	case fitterv1.ActivityMetric_ACTIVITY_METRIC_COUNT:
		return metricCount, nil
	default:
		return "", fmt.Errorf("unspecified or unknown activity metric")
	}
}

func metricFromDB(s string) fitterv1.ActivityMetric {
	switch s {
	case metricDistance:
		return fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE
	case metricElevation:
		return fitterv1.ActivityMetric_ACTIVITY_METRIC_ELEVATION
	case metricTime:
		return fitterv1.ActivityMetric_ACTIVITY_METRIC_TIME
	case metricCount:
		return fitterv1.ActivityMetric_ACTIVITY_METRIC_COUNT
	default:
		return fitterv1.ActivityMetric_ACTIVITY_METRIC_UNSPECIFIED
	}
}

func campaignModeToDB(m fitterv1.CampaignMode) (string, error) {
	switch m {
	case fitterv1.CampaignMode_CAMPAIGN_MODE_POOLED:
		return "pooled", nil
	case fitterv1.CampaignMode_CAMPAIGN_MODE_PER_MEMBER:
		return "per_member", nil
	default:
		return "", fmt.Errorf("unspecified or unknown campaign mode")
	}
}

func campaignModeFromDB(s string) fitterv1.CampaignMode {
	switch s {
	case "pooled":
		return fitterv1.CampaignMode_CAMPAIGN_MODE_POOLED
	case "per_member":
		return fitterv1.CampaignMode_CAMPAIGN_MODE_PER_MEMBER
	default:
		return fitterv1.CampaignMode_CAMPAIGN_MODE_UNSPECIFIED
	}
}

// activityMetricValue extracts the value of a single activity for the given metric.
// "count" is always 1 per activity — summing it across activities yields an activity count.
func activityMetricValue(a db.Activity, metric string) float64 {
	switch metric {
	case metricDistance:
		return a.DistanceM
	case metricElevation:
		return a.ElevationM
	case metricTime:
		return float64(a.MovingTimeS)
	case metricCount:
		return 1
	default:
		return 0
	}
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
