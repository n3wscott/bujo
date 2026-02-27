package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	readmeCommandsStartMarker = "<!-- BEGIN GENERATED COMMANDS -->"
	readmeCommandsEndMarker   = "<!-- END GENERATED COMMANDS -->"
)

// GeneratedCommandsSection returns the generated markdown section for README.
func GeneratedCommandsSection() string {
	root := New()
	var b strings.Builder
	b.WriteString("Generated from the Cobra command tree (`go run ./cmd/gendocs --check`).\n\n")
	b.WriteString("```text\n")
	writeCommandTree(&b, root, 0)
	b.WriteString("```\n")
	return b.String()
}

// ReplaceGeneratedCommandsSection swaps the generated command section bounded by
// README markers.
func ReplaceGeneratedCommandsSection(readme string) (string, error) {
	start := strings.Index(readme, readmeCommandsStartMarker)
	if start < 0 {
		return "", fmt.Errorf("missing marker %q", readmeCommandsStartMarker)
	}
	end := strings.Index(readme, readmeCommandsEndMarker)
	if end < 0 {
		return "", fmt.Errorf("missing marker %q", readmeCommandsEndMarker)
	}
	if end < start {
		return "", fmt.Errorf("marker order is invalid")
	}

	before := readme[:start+len(readmeCommandsStartMarker)]
	after := readme[end:]
	section := "\n\n" + GeneratedCommandsSection() + "\n"
	return before + section + after, nil
}

func writeCommandTree(b *strings.Builder, cmd *cobra.Command, depth int) {
	indent := strings.Repeat("  ", depth)
	line := cmd.CommandPath()
	short := strings.TrimSpace(cmd.Short)
	if short != "" {
		line = fmt.Sprintf("%s - %s", line, short)
	}
	b.WriteString(indent)
	b.WriteString(line)
	b.WriteString("\n")

	children := visibleChildren(cmd)
	for _, child := range children {
		writeCommandTree(b, child, depth+1)
	}
}

func visibleChildren(cmd *cobra.Command) []*cobra.Command {
	children := make([]*cobra.Command, 0, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		if child == nil || child.Hidden || child.Name() == "help" || child.IsAdditionalHelpTopicCommand() {
			continue
		}
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].CommandPath() < children[j].CommandPath()
	})
	return children
}
