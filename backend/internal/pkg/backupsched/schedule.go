// Package backupsched parses the BACKUP_SCHEDULE expression used by the
// built-in backup scheduler. Two forms are supported: a standard 5-field cron
// expression ("0 3 * * *") and a fixed interval ("@every 6h" or a plain Go
// duration like "6h" / "90m", minimum 1 minute). It has no external
// dependencies so both config validation and the backup module can use it.
package backupsched

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule yields fire times for the built-in backup scheduler.
type Schedule interface {
	// Next returns the first fire time strictly after t.
	Next(t time.Time) time.Time
	// String returns a normalized description safe for logs.
	String() string
}

const minInterval = time.Minute

// Parse accepts a 5-field cron expression, "@every <duration>", or a plain
// Go duration string.
func Parse(expr string) (Schedule, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("backup schedule is empty")
	}
	if rest, ok := strings.CutPrefix(expr, "@every "); ok {
		return parseInterval(rest)
	}
	if !strings.ContainsAny(expr, " \t") {
		if d, err := time.ParseDuration(expr); err == nil {
			return newInterval(d)
		}
	}
	return parseCron(expr)
}

type intervalSchedule struct{ every time.Duration }

func parseInterval(raw string) (Schedule, error) {
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("backup schedule interval %q invalid: %w", raw, err)
	}
	return newInterval(d)
}

func newInterval(d time.Duration) (Schedule, error) {
	if d < minInterval {
		return nil, fmt.Errorf("backup schedule interval must be at least %s", minInterval)
	}
	return intervalSchedule{every: d}, nil
}

func (s intervalSchedule) Next(t time.Time) time.Time { return t.Add(s.every) }

func (s intervalSchedule) String() string { return "@every " + s.every.String() }

// cronSchedule is a standard 5-field cron: minute hour day-of-month month
// day-of-week. Fields accept "*", single values, ranges (a-b), steps (*/n,
// a-b/n) and comma lists. Day-of-month and day-of-week combine with OR when
// both are restricted, matching classic cron semantics.
type cronSchedule struct {
	expr                          string
	minute, hour, dom, month, dow uint64
	domRestricted, dowRestricted  bool
}

type cronField struct {
	min, max int
	name     string
}

var cronFields = []cronField{
	{0, 59, "minute"},
	{0, 23, "hour"},
	{1, 31, "day-of-month"},
	{1, 12, "month"},
	{0, 6, "day-of-week"},
}

func parseCron(expr string) (Schedule, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("backup schedule cron %q must have 5 fields (minute hour day month weekday)", expr)
	}
	var bits [5]uint64
	var restricted [5]bool
	for i, part := range parts {
		b, r, err := parseCronField(part, cronFields[i])
		if err != nil {
			return nil, err
		}
		bits[i] = b
		restricted[i] = r
	}
	return &cronSchedule{
		expr:   strings.Join(parts, " "),
		minute: bits[0], hour: bits[1], dom: bits[2], month: bits[3], dow: bits[4],
		domRestricted: restricted[2], dowRestricted: restricted[4],
	}, nil
}

// parseCronField returns a bitmask of allowed values plus whether the field
// restricts anything (i.e. is not "*" or "*/1").
func parseCronField(part string, f cronField) (uint64, bool, error) {
	var bits uint64
	restricted := false
	for _, item := range strings.Split(part, ",") {
		body, step := item, 1
		if idx := strings.Index(item, "/"); idx >= 0 {
			body = item[:idx]
			n, err := strconv.Atoi(item[idx+1:])
			if err != nil || n < 1 {
				return 0, false, fmt.Errorf("backup schedule %s field %q: invalid step", f.name, item)
			}
			step = n
		}
		lo, hi := f.min, f.max
		if body != "*" {
			var err error
			if a, b, ok := strings.Cut(body, "-"); ok {
				lo, err = parseCronValue(a, f)
				if err != nil {
					return 0, false, err
				}
				hi, err = parseCronValue(b, f)
				if err != nil {
					return 0, false, err
				}
				if hi < lo {
					return 0, false, fmt.Errorf("backup schedule %s field %q: descending range", f.name, item)
				}
			} else {
				lo, err = parseCronValue(body, f)
				if err != nil {
					return 0, false, err
				}
				hi = lo
				if step > 1 {
					hi = f.max
				}
			}
			restricted = true
		} else if step > 1 {
			restricted = true
		}
		for v := lo; v <= hi; v += step {
			bits |= 1 << uint(v)
		}
	}
	return bits, restricted, nil
}

func parseCronValue(raw string, f cronField) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("backup schedule %s field %q: not a number", f.name, raw)
	}
	if f.name == "day-of-week" && v == 7 {
		v = 0 // both 0 and 7 mean Sunday
	}
	if v < f.min || v > f.max {
		return 0, fmt.Errorf("backup schedule %s field %q: out of range %d-%d", f.name, raw, f.min, f.max)
	}
	return v, nil
}

func (s *cronSchedule) String() string { return s.expr }

// Next scans forward minute by minute (bounded to ~13 months) for the first
// matching time strictly after t. Backup schedules fire at most every minute
// so the linear scan is fine.
func (s *cronSchedule) Next(t time.Time) time.Time {
	cur := t.Truncate(time.Minute).Add(time.Minute)
	limit := t.AddDate(1, 1, 0)
	for cur.Before(limit) {
		if s.matches(cur) {
			return cur
		}
		cur = cur.Add(time.Minute)
	}
	return time.Time{}
}

func (s *cronSchedule) matches(t time.Time) bool {
	if s.minute&(1<<uint(t.Minute())) == 0 || s.hour&(1<<uint(t.Hour())) == 0 || s.month&(1<<uint(int(t.Month()))) == 0 {
		return false
	}
	domOK := s.dom&(1<<uint(t.Day())) != 0
	dowOK := s.dow&(1<<uint(int(t.Weekday()))) != 0
	if s.domRestricted && s.dowRestricted {
		return domOK || dowOK
	}
	return domOK && dowOK
}
