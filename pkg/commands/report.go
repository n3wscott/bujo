package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
	"tableflip.dev/bujo/pkg/timeutil"
)

func addReport(topLevel *cobra.Command) {
	var last string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Display recently completed entries grouped by collection",
		Long: `Report lists completed entries grouped by collection within the specified time window.

Examples:
  bujo report
  bujo report --last 3d
  bujo report --last 1w2d`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			duration, label, err := timeutil.ParseWindow(last)
			if err != nil {
				return err
			}
			until := time.Now()
			since := until.Add(-duration)

			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			svc := &app.Service{
				Persistence: p,
			}
			result, err := svc.Report(context.Background(), since, until)
			if err != nil {
				return err
			}
			renderReport(result, label)
			return nil
		},
	}

	cmd.Flags().StringVar(&last, "last", timeutil.DefaultWindow, "time window to include (for example 3d, 1w)")
	topLevel.AddCommand(cmd)
}

func renderReport(result app.ReportResult, label string) {
	since := result.Since.Local().Format("2006-01-02 15:04")
	until := result.Until.Local().Format("2006-01-02 15:04")
	fmt.Printf("Report · last %s (%s → %s)\n", label, since, until)

	if result.Total == 0 {
		fmt.Println("  No completed entries found in this window.")
		fmt.Println()
		return
	}

	for _, section := range result.Sections {
		fmt.Printf("\n%s\n", section.Collection)
		included := make(map[string]*entry.Entry, len(section.Entries))
		for i := range section.Entries {
			included[section.Entries[i].Entry.ID] = section.Entries[i].Entry
		}
		for _, item := range section.Entries {
			indent := strings.Repeat("  ", depthForEntry(item.Entry, included))
			signifier := item.Entry.Signifier.String()
			if signifier == glyph.None.String() {
				signifier = " "
			}
			bullet := item.Entry.Bullet.Glyph().Symbol
			if bullet == "" {
				bullet = item.Entry.Bullet.String()
			}
			message := item.Entry.Message
			if strings.TrimSpace(message) == "" {
				message = "<empty>"
			}
			line := fmt.Sprintf("  %s%s %s %s", indent, signifier, bullet, message)
			if item.Completed {
				line = fmt.Sprintf("%s  (completed %s)", line, item.CompletedAt.Local().Format("2006-01-02 15:04"))
			}
			fmt.Println(line)
		}
	}

	fmt.Println()
	// TODO: support --output formats (text/json/markdown) for saving reports.
}

func depthForEntry(e *entry.Entry, included map[string]*entry.Entry) int {
	if e == nil {
		return 0
	}
	depth := 0
	visited := make(map[string]bool)
	parentID := e.ParentID
	for parentID != "" {
		if visited[parentID] {
			break
		}
		visited[parentID] = true
		parent, ok := included[parentID]
		if !ok {
			break
		}
		depth++
		parentID = parent.ParentID
	}
	return depth
}
