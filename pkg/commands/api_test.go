package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
)

func runAPICommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	root := &cobra.Command{Use: "bujo"}
	addAPI(root)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)

	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func decodeAPIResponse(t *testing.T, output string) map[string]any {
	t.Helper()

	var resp map[string]any
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("failed to decode api response %q: %v", output, err)
	}
	return resp
}

func getString(t *testing.T, payload map[string]any, key string) string {
	t.Helper()
	v, ok := payload[key]
	if !ok {
		t.Fatalf("missing key %q in payload: %#v", key, payload)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("key %q is not string: %#v", key, v)
	}
	return s
}

func getBool(t *testing.T, payload map[string]any, key string) bool {
	t.Helper()
	v, ok := payload[key]
	if !ok {
		t.Fatalf("missing key %q in payload: %#v", key, payload)
	}
	b, ok := v.(bool)
	if !ok {
		t.Fatalf("key %q is not bool: %#v", key, v)
	}
	return b
}

func getInt(t *testing.T, payload map[string]any, key string) int {
	t.Helper()
	v, ok := payload[key]
	if !ok {
		t.Fatalf("missing key %q in payload: %#v", key, payload)
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("key %q is not numeric: %#v", key, v)
	}
	return int(f)
}

func getMap(t *testing.T, payload map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := payload[key]
	if !ok {
		t.Fatalf("missing key %q in payload: %#v", key, payload)
	}
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("key %q is not object: %#v", key, v)
	}
	return m
}

func loadServiceForJournal(t *testing.T, journal string) *app.Service {
	t.Helper()
	p, err := store.Load(apiConfig{path: journal})
	if err != nil {
		t.Fatalf("failed to load persistence: %v", err)
	}
	return &app.Service{Persistence: p}
}

func seedEntry(t *testing.T, journal, collection, message string, bullet glyph.Bullet) string {
	t.Helper()
	svc := loadServiceForJournal(t, journal)
	e, err := svc.Add(context.Background(), collection, bullet, message, glyph.None)
	if err != nil {
		t.Fatalf("failed to seed entry: %v", err)
	}
	return e.ID
}

func TestAPIInfoReturnsResolvedJournal(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	output, err := runAPICommand(t, "api", "info", "--journal", journal)
	if err != nil {
		t.Fatalf("expected command to succeed: %v", err)
	}

	resp := decodeAPIResponse(t, output)
	if !getBool(t, resp, "ok") {
		t.Fatalf("expected ok response: %#v", resp)
	}
	if got := getString(t, resp, "action"); got != "info" {
		t.Fatalf("expected action info, got %q", got)
	}
	if got := getString(t, resp, "journal"); got != journal {
		t.Fatalf("expected journal %q, got %q", journal, got)
	}
	if got := getString(t, resp, "journal_source"); got != "flag" {
		t.Fatalf("expected journal_source=flag, got %q", got)
	}
}

func TestAPICollectionsEnsureCreatesCollection(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	output, err := runAPICommand(t,
		"api", "collections", "ensure",
		"--journal", journal,
		"--name", "project/alpha",
	)
	if err != nil {
		t.Fatalf("expected command to succeed: %v", err)
	}

	resp := decodeAPIResponse(t, output)
	if got := getString(t, resp, "action"); got != "collections.ensure" {
		t.Fatalf("expected action collections.ensure, got %q", got)
	}
	collection := getMap(t, resp, "collection")
	if got := getString(t, collection, "name"); got != "project/alpha" {
		t.Fatalf("expected collection name project/alpha, got %q", got)
	}
}

func TestAPIEntriesAddAndList(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	addOutput, err := runAPICommand(t,
		"api", "entries", "add",
		"--journal", journal,
		"--message", "follow up with infra",
		"--type", "task",
	)
	if err != nil {
		t.Fatalf("expected add command to succeed: %v", err)
	}
	addResp := decodeAPIResponse(t, addOutput)
	entry := getMap(t, addResp, "entry")
	entryID := getString(t, entry, "id")
	if entryID == "" {
		t.Fatalf("expected entry id in add response")
	}

	listOutput, err := runAPICommand(t,
		"api", "entries", "list",
		"--journal", journal,
		"--collection", "today",
	)
	if err != nil {
		t.Fatalf("expected list command to succeed: %v", err)
	}
	listResp := decodeAPIResponse(t, listOutput)
	if got := getInt(t, listResp, "count"); got != 1 {
		t.Fatalf("expected count=1, got %d", got)
	}
}

func TestAPIEntriesResolveAmbiguousRequiresFirst(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	collection := resolveAPICollection("today")
	seedEntry(t, journal, collection, "duplicate message one", glyph.Task)
	seedEntry(t, journal, collection, "duplicate message two", glyph.Task)

	ambiguousOutput, err := runAPICommand(t,
		"api", "entries", "resolve",
		"--journal", journal,
		"--message", "duplicate message",
		"--collection", "today",
	)
	if err == nil {
		t.Fatalf("expected ambiguous resolve to fail")
	}
	ambiguousResp := decodeAPIResponse(t, ambiguousOutput)
	if getBool(t, ambiguousResp, "ok") {
		t.Fatalf("expected non-ok response for ambiguous match")
	}
	ambiguousErr := getMap(t, ambiguousResp, "error")
	if got := getString(t, ambiguousErr, "code"); got != "ambiguous_match" {
		t.Fatalf("expected ambiguous_match code, got %q", got)
	}

	resolvedOutput, err := runAPICommand(t,
		"api", "entries", "resolve",
		"--journal", journal,
		"--message", "duplicate message",
		"--collection", "today",
		"--first",
	)
	if err != nil {
		t.Fatalf("expected resolve with --first to succeed: %v", err)
	}
	resolvedResp := decodeAPIResponse(t, resolvedOutput)
	if !getBool(t, resolvedResp, "ok") {
		t.Fatalf("expected ok resolve response")
	}
	if got := getInt(t, resolvedResp, "matched_count"); got != 2 {
		t.Fatalf("expected matched_count=2, got %d", got)
	}
}

func TestAPIEntriesCompleteAndStrikeSelectors(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	collection := resolveAPICollection("today")
	seedEntry(t, journal, collection, "complete me", glyph.Task)
	strikeID := seedEntry(t, journal, collection, "strike me", glyph.Task)

	completeOutput, err := runAPICommand(t,
		"api", "entries", "complete",
		"--journal", journal,
		"--message", "complete me",
		"--collection", "today",
		"--exact",
	)
	if err != nil {
		t.Fatalf("expected complete command to succeed: %v", err)
	}
	completeResp := decodeAPIResponse(t, completeOutput)
	completeEntry := getMap(t, completeResp, "entry")
	if got := getString(t, completeEntry, "bullet"); got != "comp" {
		t.Fatalf("expected bullet comp, got %q", got)
	}

	strikeOutput, err := runAPICommand(t,
		"api", "entries", "strike",
		"--journal", journal,
		"--id", strikeID,
	)
	if err != nil {
		t.Fatalf("expected strike command to succeed: %v", err)
	}
	strikeResp := decodeAPIResponse(t, strikeOutput)
	strikeEntry := getMap(t, strikeResp, "entry")
	if got := getString(t, strikeEntry, "bullet"); got != "irev" {
		t.Fatalf("expected bullet irev, got %q", got)
	}
}

func TestAPIEntriesMove(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	collection := resolveAPICollection("today")
	id := seedEntry(t, journal, collection, "move me", glyph.Task)

	output, err := runAPICommand(t,
		"api", "entries", "move",
		"--journal", journal,
		"--id", id,
		"--target", "project/alpha",
	)
	if err != nil {
		t.Fatalf("expected move command to succeed: %v", err)
	}

	resp := decodeAPIResponse(t, output)
	entry := getMap(t, resp, "entry")
	if got := getString(t, entry, "collection"); got != "project/alpha" {
		t.Fatalf("expected moved collection project/alpha, got %q", got)
	}
	if got := getString(t, resp, "source_id"); got != id {
		t.Fatalf("expected source_id %q, got %q", id, got)
	}
}

func TestAPIEntriesParentSetAndUnset(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	collection := resolveAPICollection("today")
	parentID := seedEntry(t, journal, collection, "parent", glyph.Task)
	childID := seedEntry(t, journal, collection, "child", glyph.Task)

	setOutput, err := runAPICommand(t,
		"api", "entries", "parent", "set",
		"--journal", journal,
		"--id", childID,
		"--parent-id", parentID,
	)
	if err != nil {
		t.Fatalf("expected parent set command to succeed: %v", err)
	}
	setResp := decodeAPIResponse(t, setOutput)
	setEntry := getMap(t, setResp, "entry")
	if got := getString(t, setEntry, "parent_id"); got != parentID {
		t.Fatalf("expected parent_id %q, got %q", parentID, got)
	}

	unsetOutput, err := runAPICommand(t,
		"api", "entries", "parent", "unset",
		"--journal", journal,
		"--id", childID,
	)
	if err != nil {
		t.Fatalf("expected parent unset command to succeed: %v", err)
	}
	unsetResp := decodeAPIResponse(t, unsetOutput)
	unsetEntry := getMap(t, unsetResp, "entry")
	if _, ok := unsetEntry["parent_id"]; ok {
		t.Fatalf("expected parent_id to be omitted after unset, got %#v", unsetEntry)
	}
}

func TestAPIEntriesLockUnlockDelete(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	collection := resolveAPICollection("today")
	id := seedEntry(t, journal, collection, "immutable toggles", glyph.Task)

	lockOutput, err := runAPICommand(t,
		"api", "entries", "lock",
		"--journal", journal,
		"--id", id,
	)
	if err != nil {
		t.Fatalf("expected lock command to succeed: %v", err)
	}
	lockResp := decodeAPIResponse(t, lockOutput)
	lockEntry := getMap(t, lockResp, "entry")
	if !getBool(t, lockEntry, "immutable") {
		t.Fatalf("expected immutable=true after lock")
	}

	unlockOutput, err := runAPICommand(t,
		"api", "entries", "unlock",
		"--journal", journal,
		"--id", id,
	)
	if err != nil {
		t.Fatalf("expected unlock command to succeed: %v", err)
	}
	unlockResp := decodeAPIResponse(t, unlockOutput)
	unlockEntry := getMap(t, unlockResp, "entry")
	if getBool(t, unlockEntry, "immutable") {
		t.Fatalf("expected immutable=false after unlock")
	}

	deleteOutput, err := runAPICommand(t,
		"api", "entries", "delete",
		"--journal", journal,
		"--id", id,
	)
	if err != nil {
		t.Fatalf("expected delete command to succeed: %v", err)
	}
	deleteResp := decodeAPIResponse(t, deleteOutput)
	if !getBool(t, deleteResp, "deleted") {
		t.Fatalf("expected deleted=true in delete response")
	}

	listOutput, err := runAPICommand(t,
		"api", "entries", "list",
		"--journal", journal,
		"--collection", "today",
	)
	if err != nil {
		t.Fatalf("expected list command to succeed: %v", err)
	}
	listResp := decodeAPIResponse(t, listOutput)
	if got := getInt(t, listResp, "count"); got != 0 {
		t.Fatalf("expected count=0 after delete, got %d", got)
	}
}

func TestAPIReportReturnsCompletedEntries(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	collection := resolveAPICollection("today")
	id := seedEntry(t, journal, collection, "report me", glyph.Task)
	svc := loadServiceForJournal(t, journal)
	if _, err := svc.Complete(context.Background(), id); err != nil {
		t.Fatalf("failed to complete seeded entry: %v", err)
	}

	output, err := runAPICommand(t,
		"api", "report",
		"--journal", journal,
		"--last", "1w",
	)
	if err != nil {
		t.Fatalf("expected report command to succeed: %v", err)
	}

	resp := decodeAPIResponse(t, output)
	if got := getString(t, resp, "action"); got != "report" {
		t.Fatalf("expected action report, got %q", got)
	}
	if got := getInt(t, resp, "total"); got < 1 {
		t.Fatalf("expected total >= 1, got %d", got)
	}
}

func TestAPIEntriesResolveRequiresSelector(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	output, err := runAPICommand(t,
		"api", "entries", "resolve",
		"--journal", journal,
	)
	if err == nil {
		t.Fatalf("expected resolve without selector to fail")
	}
	resp := decodeAPIResponse(t, output)
	apiErr := getMap(t, resp, "error")
	if got := getString(t, apiErr, "code"); got != "invalid_argument" {
		t.Fatalf("expected invalid_argument code, got %q", got)
	}
}

func TestAPIEntriesCompleteMissingIDReturnsStructuredError(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.db")
	output, err := runAPICommand(t,
		"api", "entries", "complete",
		"--journal", journal,
		"--id", "deadbeefdeadbeef",
	)
	if err == nil {
		t.Fatalf("expected command error for missing entry id")
	}
	resp := decodeAPIResponse(t, output)
	apiErr := getMap(t, resp, "error")
	if got := getString(t, apiErr, "code"); got != "entry_not_found" {
		t.Fatalf("expected entry_not_found code, got %q", got)
	}
}
