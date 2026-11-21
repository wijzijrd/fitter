package service

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"

	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/progress"
)

// recurrenceToDB maps the wire enum to goals.recurrence; UNSPECIFIED is NONE.
func recurrenceToDB(r fitterv1.Recurrence) string {
	switch r {
	case fitterv1.Recurrence_RECURRENCE_DAILY:
		return "daily"
	case fitterv1.Recurrence_RECURRENCE_WEEKLY:
		return "weekly"
	case fitterv1.Recurrence_RECURRENCE_MONTHLY:
		return "monthly"
	default:
		return "none"
	}
}

func recurrenceToProto(s string) fitterv1.Recurrence {
	switch s {
	case "daily":
		return fitterv1.Recurrence_RECURRENCE_DAILY
	case "weekly":
		return fitterv1.Recurrence_RECURRENCE_WEEKLY
	case "monthly":
		return fitterv1.Recurrence_RECURRENCE_MONTHLY
	default:
		return fitterv1.Recurrence_RECURRENCE_NONE
	}
}

func recurrenceFromDB(s string) progress.Recurrence {
	switch s {
	case "daily":
		return progress.RecurDaily
	case "weekly":
		return progress.RecurWeekly
	case "monthly":
		return progress.RecurMonthly
	default:
		return progress.RecurNone
	}
}

// checkBuckets refuses recurring goals whose period would need more than
// progress.MaxBuckets buckets (e.g. a daily goal over several years).
func checkBuckets(r fitterv1.Recurrence, start, end int64) error {
	pr := recurrenceFromDB(recurrenceToDB(r))
	if pr == progress.RecurNone {
		return nil
	}
	if n := progress.CountBuckets(pr, start, end); n > progress.MaxBuckets {
		return connect.NewError(connect.CodeInvalidArgument,
			errors.New(fmt.Sprintf("a repeating goal can span at most %d periods; shorten the period or repeat less often", progress.MaxBuckets)))
	}
	return nil
}
