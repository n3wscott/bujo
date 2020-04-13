package entry

import (
	"encoding/json"
	"fmt"
	"time"
)

func ParseTime(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

type Timestamp struct {
	time.Time
}

func (t Timestamp) SameDay(then time.Time) bool {
	if t.Local().Day() == then.Local().Day() &&
		t.Local().Month() == then.Local().Month() &&
		t.Local().Year() == then.Local().Year() {
		return true
	}
	return false
}

func (t Timestamp) SameMonth(then time.Time) bool {
	if t.Local().Month() == then.Local().Month() &&
		t.Local().Year() == then.Local().Year() {
		return true
	}
	return false
}

func (t *Timestamp) MarshalJSON() ([]byte, error) {
	if t == nil || t.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(fmt.Sprintf("%q", t)), nil
}

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

func FormatTime(v time.Time) string {
	return v.UTC().Format(time.RFC3339Nano)
}
