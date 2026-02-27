package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/entry"
)

const labelCommandUsageLine = "Usage: :label add <key:value> | :label remove <key> | :unlabel <key>"

func (m *Model) handleLabelCommand(arg string) tea.Cmd {
	fields := strings.Fields(strings.TrimSpace(arg))
	if len(fields) == 1 {
		if _, err := parseLabelAddArg(fields[0]); err == nil {
			return m.addLabelToSelectedBullet(fields[0])
		}
		m.setStatus(labelCommandUsageLine)
		return nil
	}
	if len(fields) < 2 {
		m.setStatus(labelCommandUsageLine)
		return nil
	}

	verb := strings.ToLower(fields[0])
	value := strings.Join(fields[1:], " ")
	switch verb {
	case "add":
		return m.addLabelToSelectedBullet(value)
	case "remove", "rm", "delete", "del":
		return m.removeLabelKeyFromSelectedBullet(value)
	default:
		m.setStatus(labelCommandUsageLine)
		return nil
	}
}

func (m *Model) addLabelToSelectedBullet(rawLabel string) tea.Cmd {
	label, err := parseLabelAddArg(rawLabel)
	if err != nil {
		m.setStatus("Invalid label: " + err.Error())
		return nil
	}
	collectionID, bulletID, bulletLabel, labels, ok := m.selectedBulletForLabelCommand("Add label")
	if !ok {
		return nil
	}
	if hasLabel(labels, label) {
		m.setStatus("Label already set: " + label)
		return nil
	}
	if m.service == nil {
		m.setStatus("Add label unavailable: service offline")
		return nil
	}
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := m.service.AddLabels(ctx, bulletID, []string{label}); err != nil {
		m.setStatus("Add label failed: " + err.Error())
		return nil
	}
	m.setStatus(fmt.Sprintf("Added label %s to %s", label, bulletLabel))
	return m.collectionSyncCmd(collectionID)
}

func (m *Model) removeLabelKeyFromSelectedBullet(rawKey string) tea.Cmd {
	key, err := parseLabelRemoveArg(rawKey)
	if err != nil {
		m.setStatus("Invalid label remove: " + err.Error())
		return nil
	}
	collectionID, bulletID, bulletLabel, labels, ok := m.selectedBulletForLabelCommand("Remove label")
	if !ok {
		return nil
	}
	remove := labelsWithKey(labels, key)
	if len(remove) == 0 {
		m.setStatus("Label key not set: " + key)
		return nil
	}
	if m.service == nil {
		m.setStatus("Remove label unavailable: service offline")
		return nil
	}
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := m.service.RemoveLabels(ctx, bulletID, remove); err != nil {
		m.setStatus("Remove label failed: " + err.Error())
		return nil
	}
	m.setStatus(fmt.Sprintf("Removed label key %s from %s", key, bulletLabel))
	return m.collectionSyncCmd(collectionID)
}

func (m *Model) selectedBulletForLabelCommand(action string) (collectionID, bulletID, bulletLabel string, labels []string, ok bool) {
	journal := m.journal()
	if journal == nil {
		m.setStatus(action + " unavailable: journal cache offline")
		return "", "", "", nil, false
	}
	section, bullet, selected := journal.CurrentSelection()
	if !selected {
		m.setStatus(action + " unavailable: select a task")
		return "", "", "", nil, false
	}
	collectionID = strings.TrimSpace(section.ID)
	bulletID = strings.TrimSpace(bullet.ID)
	if collectionID == "" || bulletID == "" {
		m.setStatus(action + " unavailable: select a task")
		return "", "", "", nil, false
	}
	bulletLabel = strings.TrimSpace(bullet.Label)
	if bulletLabel == "" {
		bulletLabel = bulletID
	}
	labels = entry.NormalizeLabels(append([]string(nil), bullet.Labels...))
	return collectionID, bulletID, bulletLabel, labels, true
}

func parseLabelAddArg(raw string) (string, error) {
	normalized := entry.NormalizeLabels([]string{raw})
	if len(normalized) != 1 {
		return "", fmt.Errorf("expected key:value")
	}
	label := normalized[0]
	parts := strings.SplitN(label, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("expected key:value")
	}
	if !isLabelPart(parts[0]) || !isLabelPart(parts[1]) {
		return "", fmt.Errorf("use [a-z0-9._-] for key and value")
	}
	return label, nil
}

func parseLabelRemoveArg(raw string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return "", fmt.Errorf("expected key")
	}
	if strings.Contains(key, ":") {
		return "", fmt.Errorf("expected key only (no :)")
	}
	if !isLabelPart(key) {
		return "", fmt.Errorf("use [a-z0-9._-] for key")
	}
	return key, nil
}

func isLabelPart(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '.', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func hasLabel(labels []string, label string) bool {
	for _, existing := range labels {
		if existing == label {
			return true
		}
	}
	return false
}

func labelsWithKey(labels []string, key string) []string {
	if key == "" {
		return nil
	}
	prefix := key + ":"
	matched := make([]string, 0, len(labels))
	for _, label := range labels {
		if strings.HasPrefix(label, prefix) {
			matched = append(matched, label)
		}
	}
	return matched
}
