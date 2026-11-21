package service

import (
	"errors"
	"fmt"
	"time"

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
	// Years outside this range are nonsense for a fitness goal and would only
	// feed absurd values to the bucket maths.
	if lo, hi := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC).Unix(); start < lo || end > hi || end < start {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("a repeating goal's period must fall between 2000 and 2100"))
	}
	if n := progress.CountBuckets(pr, start, end); n == 0 || n > progress.MaxBuckets {
		return connect.NewError(connect.CodeInvalidArgument,
			errors.New(fmt.Sprintf("a repeating goal can span at most %d periods; shorten the period or repeat less often", progress.MaxBuckets)))
	}
	return nil
}
