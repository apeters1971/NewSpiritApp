package store

import "time"

func dateEnd(start time.Time, end *time.Time) time.Time {
	if end != nil && !end.IsZero() {
		return *end
	}
	return start
}

func startOfChoirDay(t time.Time) time.Time {
	t = t.In(choirZone())
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, choirZone())
}

func DateIsCurrent(d Date) bool {
	if d.Status == StatusCancelled {
		return false
	}
	floor := startOfChoirDay(now())
	if d.PollOpen && len(d.Options) > 0 {
		for _, o := range d.Options {
			if !dateEnd(o.StartsAt, o.EndsAt).Before(floor) {
				return true
			}
		}
		return false
	}
	return !dateEnd(d.StartsAt, d.EndsAt).Before(floor)
}
