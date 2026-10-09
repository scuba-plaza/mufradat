package claude

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	epochPattern = regexp.MustCompile(`\|(\d{9,11})\b`)
	clockPattern = regexp.MustCompile(`(?i)resets?\s+(?:at\s+)?(\d{1,2})(?::(\d{2}))?\s*(am|pm)?(?:\s*\(([^)]+)\))?`)
)

func parseReset(msg string, now time.Time) time.Time {
	if m := epochPattern.FindStringSubmatch(msg); m != nil {
		if sec, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			return time.Unix(sec, 0).UTC()
		}
	}
	m := clockPattern.FindStringSubmatch(msg)
	if m == nil {
		return time.Time{}
	}
	hour, _ := strconv.Atoi(m[1])
	minute := 0
	if m[2] != "" {
		minute, _ = strconv.Atoi(m[2])
	}
	switch strings.ToLower(m[3]) {
	case "pm":
		if hour < 12 {
			hour += 12
		}
	case "am":
		if hour == 12 {
			hour = 0
		}
	}
	if hour > 23 || minute > 59 {
		return time.Time{}
	}
	loc := now.Location()
	if m[4] != "" {
		if l, err := time.LoadLocation(strings.TrimSpace(m[4])); err == nil {
			loc = l
		}
	}
	local := now.In(loc)
	at := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	if !at.After(now) {
		at = at.AddDate(0, 0, 1)
	}
	return at.UTC()
}
