package entry

import (
	"encoding/json"
	"fmt"
	"time"
)

// ParseTime parses RFC3339 timestamps used in persisted entries.
func ParseTime(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// Timestamp wraps time.Time with helper methods for journaling logic.
type Timestamp struct {
	time.Time
}

// SameDay reports whether the timestamp falls on the same local day as then.
func (t Timestamp) SameDay(then time.Time) bool {
	if t.Local().Day() == then.Local().Day() &&
		t.Local().Month() == then.Local().Month() &&
		t.Local().Year() == then.Local().Year() {
		return true
	}
	return false
}

// SameMonth reports whether the timestamp falls in the same month/year as then.
func (t Timestamp) SameMonth(then time.Time) bool {
	if t.Local().Month() == then.Local().Month() &&
		t.Local().Year() == then.Local().Year() {
		return true
	}
	return false
}

// MarshalJSON serializes the timestamp into RFC3339 JSON.
func (t *Timestamp) MarshalJSON() ([]byte, error) {
	if t == nil || t.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(fmt.Sprintf("%q", t)), nil
}

// UnmarshalJSON parses a timestamp from its JSON encoding.
func (t *Timestamp) UnmarshalJSON(b []byte) error {
	var timestamp string
	if err := json.Unmarshal(b, &timestamp); err != nil {
		return err
	}
	var err error
	t.Time, err = ParseTime(timestamp)
	return err
}

func (t Timestamp) String() string {
	return t.UTC().Format(time.RFC3339)
}

// FormatTime formats the time as RFC3339Nano.
func FormatTime(v time.Time) string {
	return v.UTC().Format(time.RFC3339Nano)
}
