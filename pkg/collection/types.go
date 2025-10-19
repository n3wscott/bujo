// Package collection defines metadata helpers for bujo collections.
package collection

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Type identifies how a collection should behave in the UI.
type Type string

const (
	// TypeGeneric is the default free-form collection.
	TypeGeneric Type = "generic"
	// TypeMonthly groups child month collections (e.g. Future log).
	TypeMonthly Type = "monthly"
	// TypeDaily groups day collections inside a month.
	TypeDaily Type = "daily"
	// TypeTracking is a numeric tracker collection.
	TypeTracking Type = "tracking"
)

// AllTypes returns the list of supported collection types.
func AllTypes() []Type {
	return []Type{
		TypeGeneric,
		TypeMonthly,
		TypeDaily,
		TypeTracking,
	}
}

// ParseType converts a string to a Type or returns an error for unknown values.
func ParseType(raw string) (Type, error) {
	t := Type(strings.ToLower(strings.TrimSpace(raw)))
	if t == "" {
		return TypeGeneric, nil
	}
	for _, candidate := range AllTypes() {
		if candidate == t {
			return candidate, nil
		}
	}
	return TypeGeneric, fmt.Errorf("collection: unknown type %q", raw)
}

// MustType parses the input and panics on error. Intended for tests/config.
func MustType(raw string) Type {
	t, err := ParseType(raw)
	if err != nil {
		panic(err)
	}
	return t
}

var (
	monthFormat      = "January 2006"
	dayFormat        = "January 2, 2006"
	monthNamePattern = regexp.MustCompile(`^[A-Za-z]+ \d{4}$`)
)

// IsMonthName reports whether value looks like "October 2025".
func IsMonthName(name string) bool {
	if !monthNamePattern.MatchString(name) {
		return false
	}
	_, err := time.Parse(monthFormat, name)
	return err == nil
}

// IsDayName reports whether value looks like "October 11, 2025".
func IsDayName(name string) bool {
	if strings.Count(name, ",") != 1 {
		return false
	}
	_, err := time.Parse(dayFormat, name)
	return err == nil
}

// ValidateChildName checks if a child collection name is valid for the parent type.
func ValidateChildName(parentType Type, parentName string, childName string) error {
	switch parentType {
	case TypeMonthly:
		if IsMonthName(childName) {
			return nil
		}
		return fmt.Errorf("collection: %q is monthly and only accepts month children, got %q", parentName, childName)
	case TypeDaily:
		if IsDayName(childName) && childMatchesParentMonth(parentName, childName) {
			return nil
		}
		return fmt.Errorf("collection: %q is daily and only accepts day children, got %q", parentName, childName)
	default:
		return nil
	}
}

func childMatchesParentMonth(parentName, childName string) bool {
	parent, err := time.Parse(monthFormat, parentName)
	if err != nil {
		return false
	}
	child, err := time.Parse(dayFormat, childName)
	if err != nil {
		return false
	}
	return parent.Year() == child.Year() && parent.Month() == child.Month()
}

// GuessType inspects the collection name (and optionally parent) to infer a type.
func GuessType(name string, parent Type) Type {
	switch {
	case parent == TypeMonthly:
		return TypeDaily
	case parent == TypeDaily:
		return TypeGeneric
	case IsMonthName(name):
		return TypeDaily
	case IsDayName(name):
		return TypeGeneric
	default:
		return TypeGeneric
	}
}

// ValidateTypeTransition ensures runtime changes obey rules. For now only forbids
// switching to unknown types.
func ValidateTypeTransition(current, next Type) error {
	if next == "" {
		return errors.New("collection: type cannot be empty")
	}
	for _, candidate := range AllTypes() {
		if candidate == next {
			return nil
		}
	}
	return fmt.Errorf("collection: unsupported type %q", next)
}
