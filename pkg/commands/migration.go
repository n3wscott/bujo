package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
	"tableflip.dev/bujo/pkg/timeutil"
)

func addMigration(topLevel *cobra.Command) {
	migrationCmd := &cobra.Command{
		Use:   "migration",
		Short: "Inspect tasks eligible for migration",
	}

	addMigrationList(migrationCmd)
	topLevel.AddCommand(migrationCmd)
}

func addMigrationList(parent *cobra.Command) {
	var last string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List migration candidates using the specified time window",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			duration, label, err := timeutil.ParseWindow(last)
			if err != nil {
				return err
			}
			until := time.Now()
			since := until.Add(-duration)

			persistence, err := store.Load(nil)
			if err != nil {
				return err
			}
			service := &app.Service{Persistence: persistence}
			candidates, err := service.MigrationCandidates(context.Background(), since, until)
			if err != nil {
				return err
			}
			renderMigrationList(candidates, since, until, label)
			return nil
		},
	}

	cmd.Flags().StringVar(&last, "last", timeutil.DefaultWindow, "time window to include (for example 3d, 1w)")
	parent.AddCommand(cmd)
}

func renderMigrationList(candidates []app.MigrationCandidate, since, until time.Time, label string) {
	fmt.Printf("Migration candidates · last %s (%s → %s)\n",
		label,
		since.Local().Format("2006-01-02 15:04"),
		until.Local().Format("2006-01-02 15:04"),
	)

	if len(candidates) == 0 {
		fmt.Println("  No open tasks matched this window.")
		fmt.Println()
		return
	}

	grouped := make(map[string][]app.MigrationCandidate)
	for _, cand := range candidates {
		if cand.Entry == nil {
			continue
		}
		group := cand.Entry.Collection
		grouped[group] = append(grouped[group], cand)
	}

	collections := make([]string, 0, len(grouped))
	for col := range grouped {
		collections = append(collections, col)
	}
	sort.Strings(collections)

	for _, col := range collections {
		fmt.Printf("\n%s\n", col)
		list := grouped[col]
		for _, cand := range list {
			entry := cand.Entry
			bullet := entry.Bullet.Glyph().Symbol
			if strings.TrimSpace(bullet) == "" {
				bullet = entry.Bullet.String()
			}
			message := strings.TrimSpace(entry.Message)
			if message == "" {
				message = "<empty>"
			}
			signifier := entry.Signifier.Glyph().Symbol
			if strings.TrimSpace(signifier) == "" || entry.Signifier == glyph.None {
				signifier = " "
			}
			age := "unknown"
			if !cand.LastTouched.IsZero() {
				age = humanDuration(cand.LastTouched, until)
			}
			parent := ""
			if cand.Parent != nil {
				parent = strings.TrimSpace(cand.Parent.Message)
				if parent == "" {
					parent = cand.Parent.ID
				}
				if parent != "" {
					parent = " · parent: " + parent
				}
			}
			immutable := ""
			if entry.Immutable {
				immutable = " · immutable"
			}
			line := fmt.Sprintf("  %s%s %s", signifier, bullet, message)
			line = fmt.Sprintf("%s · last touched %s%s%s", line, age, parent, immutable)
			fmt.Println(line)
			fmt.Printf("      id:%s created:%s\n", entry.ID, entry.Created.Local().Format("2006-01-02 15:04"))
		}
	}

	fmt.Println()
}
