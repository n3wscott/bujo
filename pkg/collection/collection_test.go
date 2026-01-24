package collection

import (
	"testing"
)

func TestParseTypeDefaultsAndErrors(t *testing.T) {
	if typ, err := ParseType(""); err != nil || typ != TypeGeneric {
		t.Fatalf("expected empty type to default to generic")
	}
	if _, err := ParseType("unknown"); err == nil {
		t.Fatalf("expected error for unknown type")
	}
}

func TestGuessTypeUsesParentAndNames(t *testing.T) {
	if typ := GuessType("Future", TypeMonthly); typ != TypeDaily {
		t.Fatalf("expected monthly parent to imply daily child")
	}
	if typ := GuessType("January 2024", TypeGeneric); typ != TypeDaily {
		t.Fatalf("expected month name to imply daily type")
	}
	if typ := GuessType("January 2, 2024", TypeGeneric); typ != TypeGeneric {
		t.Fatalf("expected day name to imply generic type")
	}
}

func TestValidateChildName(t *testing.T) {
	if err := ValidateChildName(TypeMonthly, "Future", "January 2024"); err != nil {
		t.Fatalf("expected monthly child to validate")
	}
	if err := ValidateChildName(TypeDaily, "January 2024", "January 2, 2024"); err != nil {
		t.Fatalf("expected daily child to validate")
	}
	if err := ValidateChildName(TypeDaily, "January 2024", "February 2, 2024"); err == nil {
		t.Fatalf("expected daily child month mismatch to fail")
	}
}

func TestValidateTypeTransition(t *testing.T) {
	if err := ValidateTypeTransition(TypeGeneric, ""); err == nil {
		t.Fatalf("expected empty type to error")
	}
	if err := ValidateTypeTransition(TypeGeneric, TypeTracking); err != nil {
		t.Fatalf("expected tracking type to be accepted")
	}
}

func TestUnmarshalListLegacyFormat(t *testing.T) {
	data := []byte(`["Inbox","Future"]`)
	metas, err := UnmarshalList(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metas) != 2 || metas[0].Type != TypeGeneric {
		t.Fatalf("expected legacy format to default types to generic")
	}
}
