package invoice

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Date is a calendar date without time zone. The zero value is an absent
// date.
type Date struct {
	year  int
	month time.Month
	day   int
}

// NewDate returns the date y-m-d. It does not normalize out-of-range values;
// use Valid to check them.
func NewDate(year int, month time.Month, day int) Date {
	return Date{year: year, month: month, day: day}
}

// DateOf returns the calendar date of t in its own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{year: y, month: m, day: d}
}

// ParseDate parses a date in one of the UNTDID 2379 formats used by CII:
// "102" (CCYYMMDD), "610" (CCYYMM, read as the first day of the month) or
// "616" (CCYYWW, an ISO 8601 week, read as its Monday; ZUGFeRD 1.0 uses it
// for invoicing periods). An empty format means "102". ISO 8601 dates
// (2006-01-02) are also accepted whatever the format, as some producers
// write them.
func ParseDate(s, format string) (Date, error) {
	s = strings.TrimSpace(s)
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return ParseISODate(s)
	}
	switch format {
	case "", "102":
		if len(s) != 8 || !digits(s) {
			return Date{}, fmt.Errorf("invalid date %q: format 102 expects CCYYMMDD", s)
		}
		y, _ := strconv.Atoi(s[:4])
		m, _ := strconv.Atoi(s[4:6])
		d, _ := strconv.Atoi(s[6:])
		dt := Date{year: y, month: time.Month(m), day: d}
		if !dt.Valid() {
			return Date{}, fmt.Errorf("invalid date %q: no such day", s)
		}
		return dt, nil
	case "610":
		if len(s) != 6 || !digits(s) {
			return Date{}, fmt.Errorf("invalid date %q: format 610 expects CCYYMM", s)
		}
		y, _ := strconv.Atoi(s[:4])
		m, _ := strconv.Atoi(s[4:])
		dt := Date{year: y, month: time.Month(m), day: 1}
		if !dt.Valid() {
			return Date{}, fmt.Errorf("invalid date %q: no such month", s)
		}
		return dt, nil
	case "616":
		if len(s) != 6 || !digits(s) {
			return Date{}, fmt.Errorf("invalid date %q: format 616 expects CCYYWW", s)
		}
		y, _ := strconv.Atoi(s[:4])
		w, _ := strconv.Atoi(s[4:])
		// January 4 is always in week 1.
		jan4 := time.Date(y, time.January, 4, 0, 0, 0, 0, time.UTC)
		monday := jan4.AddDate(0, 0, -((int(jan4.Weekday())+6)%7)+7*(w-1))
		if yy, ww := monday.ISOWeek(); w < 1 || yy != y || ww != w {
			return Date{}, fmt.Errorf("invalid date %q: no such week", s)
		}
		return DateOf(monday), nil
	}
	return Date{}, fmt.Errorf("unsupported date format %q", format)
}

// ParseISODate parses an ISO 8601 calendar date (2006-01-02).
func ParseISODate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, strings.TrimSpace(s))
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: %w", s, err)
	}
	return DateOf(t), nil
}

// MustDate parses an ISO 8601 date and panics on error. It is meant for
// constants and tests.
func MustDate(s string) Date {
	d, err := ParseISODate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func digits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// IsZero reports whether d is absent.
func (d Date) IsZero() bool { return d.year == 0 && d.month == 0 && d.day == 0 }

// Valid reports whether d is a real calendar day.
func (d Date) Valid() bool {
	if d.year < 1 || d.year > 9999 || d.month < 1 || d.month > 12 || d.day < 1 {
		return false
	}
	return d.day <= time.Date(d.year, d.month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// Year returns the year.
func (d Date) Year() int { return d.year }

// Month returns the month.
func (d Date) Month() time.Month { return d.month }

// Day returns the day of the month.
func (d Date) Day() int { return d.day }

// Time returns midnight of d in loc (UTC when loc is nil).
func (d Date) Time(loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, loc)
}

// AddDays returns d shifted by n days.
func (d Date) AddDays(n int) Date { return DateOf(d.Time(nil).AddDate(0, 0, n)) }

// Compare returns -1, 0 or +1 depending on whether d is before, equal to or
// after e.
func (d Date) Compare(e Date) int {
	switch {
	case d.year != e.year:
		return cmpInt(d.year, e.year)
	case d.month != e.month:
		return cmpInt(int(d.month), int(e.month))
	}
	return cmpInt(d.day, e.day)
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Before reports whether d is before e.
func (d Date) Before(e Date) bool { return d.Compare(e) < 0 }

// After reports whether d is after e.
func (d Date) After(e Date) bool { return d.Compare(e) > 0 }

// String returns the ISO 8601 form (2006-01-02), or "" for an absent date.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.year, int(d.month), d.day)
}

// Format102 returns the CCYYMMDD form used by CII, or "" for an absent date.
func (d Date) Format102() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d%02d%02d", d.year, int(d.month), d.day)
}

// MarshalText implements encoding.TextMarshaler (ISO 8601).
func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler. It accepts ISO 8601 and
// CCYYMMDD; empty text gives an absent date.
func (d *Date) UnmarshalText(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" {
		*d = Date{}
		return nil
	}
	v, err := ParseDate(s, "102")
	if err != nil {
		return err
	}
	*d = v
	return nil
}
