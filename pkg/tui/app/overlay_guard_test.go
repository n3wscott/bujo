package app

import "testing"

func TestActiveOverlayKindPriority(t *testing.T) {
	m := NewWithOptions(Options{})
	m.helpVisible = true
	m.reportVisible = true
	m.addVisible = true

	if got := m.activeOverlayKind(); got != overlayKindHelp {
		t.Fatalf("expected help to win priority, got %v", got)
	}
}

func TestOverlayBlocksCommand(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Model)
		expected bool
	}{
		{
			name: "report",
			setup: func(m *Model) {
				m.reportVisible = true
			},
			expected: false,
		},
		{
			name: "help",
			setup: func(m *Model) {
				m.helpVisible = true
			},
			expected: false,
		},
		{
			name: "add",
			setup: func(m *Model) {
				m.addVisible = true
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := NewWithOptions(Options{})
			tc.setup(m)
			if got := m.overlayBlocksCommand(); got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
