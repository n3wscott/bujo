package commands

import (
	"sort"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandSubcommandContract(t *testing.T) {
	root := New()
	got := commandNames(root)
	want := []string{
		"add",
		"api",
		"collections",
		"complete",
		"completion",
		"get",
		"info",
		"key",
		"log",
		"migration",
		"report",
		"strike",
		"track",
		"ui",
		"upgrade",
		"version",
	}
	assertStringSlicesEqual(t, got, want)
}

func TestAPICommandSubcommandContract(t *testing.T) {
	root := New()
	api := mustSubcommand(t, root, "api")
	got := commandNames(api)
	want := []string{"collections", "entries", "info", "report"}
	assertStringSlicesEqual(t, got, want)
}

func TestAPIEntriesSubcommandContract(t *testing.T) {
	root := New()
	entries := mustSubcommand(t, mustSubcommand(t, root, "api"), "entries")
	got := commandNames(entries)
	want := []string{"add", "complete", "delete", "dependencies", "labels", "list", "lock", "move", "parent", "resolve", "strike", "unlock"}
	assertStringSlicesEqual(t, got, want)
}

func TestAPIEntriesParentSubcommandContract(t *testing.T) {
	root := New()
	parent := mustSubcommand(t, mustSubcommand(t, mustSubcommand(t, root, "api"), "entries"), "parent")
	got := commandNames(parent)
	want := []string{"set", "unset"}
	assertStringSlicesEqual(t, got, want)
}

func TestAPIEntriesLabelsSubcommandContract(t *testing.T) {
	root := New()
	labels := mustSubcommand(t, mustSubcommand(t, mustSubcommand(t, root, "api"), "entries"), "labels")
	got := commandNames(labels)
	want := []string{"add", "clear", "remove", "set"}
	assertStringSlicesEqual(t, got, want)
}

func TestAPIEntriesDependenciesSubcommandContract(t *testing.T) {
	root := New()
	deps := mustSubcommand(t, mustSubcommand(t, mustSubcommand(t, root, "api"), "entries"), "dependencies")
	got := commandNames(deps)
	want := []string{"add", "clear", "remove", "set"}
	assertStringSlicesEqual(t, got, want)
}

func commandNames(cmd *cobra.Command) []string {
	names := make([]string, 0, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		if child == nil || child.Hidden || child.Name() == "help" || child.IsAdditionalHelpTopicCommand() {
			continue
		}
		names = append(names, child.Name())
	}
	sort.Strings(names)
	return names
}

func mustSubcommand(t *testing.T, cmd *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, child := range cmd.Commands() {
		if child != nil && child.Name() == name {
			return child
		}
	}
	t.Fatalf("missing subcommand %q under %q", name, cmd.CommandPath())
	return nil
}

func assertStringSlicesEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected command count: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected commands: got=%v want=%v", got, want)
		}
	}
}
