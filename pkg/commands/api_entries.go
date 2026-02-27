package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

const apiLayoutUS = "January 2, 2006"

type apiEntrySelectorOptions struct {
	ID         string
	Message    string
	Collection string
	Type       string
	Exact      bool
	First      bool
}

func (o *apiEntrySelectorOptions) hasAnySelector() bool {
	if strings.TrimSpace(o.ID) != "" {
		return true
	}
	if strings.TrimSpace(o.Message) != "" {
		return true
	}
	if strings.TrimSpace(o.Collection) != "" {
		return true
	}
	typeFilter := strings.TrimSpace(o.Type)
	return typeFilter != "" && !strings.EqualFold(typeFilter, string(glyph.Any))
}

func addAPIEntries(topLevel *cobra.Command, opts *apiOptions) {
	cmd := &cobra.Command{
		Use:   "entries",
		Short: "Machine-oriented entry operations",
	}

	addAPIEntriesAdd(cmd, opts)
	addAPIEntriesList(cmd, opts)
	addAPIEntriesReady(cmd, opts)
	addAPIEntriesBlocked(cmd, opts)
	addAPIEntriesResolve(cmd, opts)
	addAPIEntriesComplete(cmd, opts)
	addAPIEntriesStrike(cmd, opts)
	addAPIEntriesMove(cmd, opts)
	addAPIEntriesParent(cmd, opts)
	addAPIEntriesLabels(cmd, opts)
	addAPIEntriesDependencies(cmd, opts)
	addAPIEntriesLock(cmd, opts)
	addAPIEntriesUnlock(cmd, opts)
	addAPIEntriesDelete(cmd, opts)

	topLevel.AddCommand(cmd)
}

func addAPIEntriesReady(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntriesDependencyState(
		topLevel,
		opts,
		"ready",
		"entries.ready",
		"List actionable entries whose dependencies are satisfied",
		`List actionable entries whose dependency set is fully satisfied.

Actionable entries include task, note, and event bullets that are not immutable.
Dependencies are considered satisfied when the dependency entry is completed,
struck irrelevant, or moved (collection/future).`,
		`  bujo api entries ready --journal /tmp/codex-bujo.db --all
  bujo api entries ready --journal /tmp/codex-bujo.db --collection today --label state:open`,
		false,
	)
}

func addAPIEntriesBlocked(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntriesDependencyState(
		topLevel,
		opts,
		"blocked",
		"entries.blocked",
		"List actionable entries with unmet dependencies",
		`List actionable entries that currently have unmet dependencies.

Actionable entries include task, note, and event bullets that are not immutable.
Each response item contains:
  - entry: the entry payload
  - unmet_depends_on: dependency IDs that are missing or not yet satisfied`,
		`  bujo api entries blocked --journal /tmp/codex-bujo.db --all
  bujo api entries blocked --journal /tmp/codex-bujo.db --collection today --label owner:codex`,
		true,
	)
}

func addAPIEntriesDependencyState(topLevel *cobra.Command, opts *apiOptions, use, action, short, long, example string, blocked bool) {
	var (
		collection         string
		kind               string
		query              string
		all                bool
		labelAllRaw        []string
		labelAnyRaw        []string
		withoutRaw         []string
		dependsOnAllRaw    []string
		dependsOnAnyRaw    []string
		withoutDependsRaw  []string
		labelAll           []string
		labelAny           []string
		withoutLabels      []string
		dependsOnAll       []string
		dependsOnAny       []string
		withoutDependsOnID []string
	)

	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			bullet, err := parseSelectorBullet(kind)
			if err != nil {
				return apiFailure(cmd, opts, action, expandUserPath(opts.Journal), err)
			}
			labelAll = entry.NormalizeLabels(labelAllRaw)
			labelAny = entry.NormalizeLabels(labelAnyRaw)
			withoutLabels = entry.NormalizeLabels(withoutRaw)
			dependsOnAll = entry.NormalizeDependsOnIDs(dependsOnAllRaw)
			dependsOnAny = entry.NormalizeDependsOnIDs(dependsOnAnyRaw)
			withoutDependsOnID = entry.NormalizeDependsOnIDs(withoutDependsRaw)

			return runAPI(cmd, opts, action, func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				allEntries := runtime.Persistence.ListAll(ctx)
				var candidates []*entry.Entry
				resolvedCollection := ""
				if all {
					candidates = allEntries
				} else {
					resolvedCollection = resolveAPICollection(collection)
					candidates = runtime.Persistence.List(ctx, resolvedCollection)
				}
				candidates = filterEntriesByFilters(candidates, apiEntryListFilter{
					Bullet:           bullet,
					Query:            query,
					LabelsAll:        labelAll,
					LabelsAny:        labelAny,
					WithoutLabels:    withoutLabels,
					DependsOnAll:     dependsOnAll,
					DependsOnAny:     dependsOnAny,
					WithoutDependsOn: withoutDependsOnID,
				})
				readyEntries, blockedEntries := partitionByDependencyState(candidates, allEntries)

				fields := map[string]any{}
				if blocked {
					fields["count"] = len(blockedEntries)
					fields["entries"] = toAPIBlockedEntryPayloads(blockedEntries)
				} else {
					fields["count"] = len(readyEntries)
					fields["entries"] = toAPIEntryPayloads(readyEntries)
				}
				if resolvedCollection != "" {
					fields["collection"] = resolvedCollection
				}
				if bullet != glyph.Any {
					fields["type"] = string(bullet)
				}
				if strings.TrimSpace(query) != "" {
					fields["query"] = strings.TrimSpace(query)
				}
				if len(labelAll) > 0 {
					fields["label"] = labelAll
				}
				if len(labelAny) > 0 {
					fields["label_any"] = labelAny
				}
				if len(withoutLabels) > 0 {
					fields["without_label"] = withoutLabels
				}
				if len(dependsOnAll) > 0 {
					fields["depends_on"] = dependsOnAll
				}
				if len(dependsOnAny) > 0 {
					fields["depends_on_any"] = dependsOnAny
				}
				if len(withoutDependsOnID) > 0 {
					fields["without_depends_on"] = withoutDependsOnID
				}
				return fields, nil
			})
		},
	}

	cmd.Flags().StringVar(&collection, "collection", "today", "Collection name")
	cmd.Flags().StringVar(&kind, "type", string(glyph.Any), "Filter by entry type")
	cmd.Flags().StringVar(&query, "query", "", "Case-insensitive message substring filter")
	cmd.Flags().BoolVar(&all, "all", false, "List across all collections")
	cmd.Flags().StringSliceVar(&labelAllRaw, "label", nil, "Filter entries that contain all labels (repeatable)")
	cmd.Flags().StringSliceVar(&labelAnyRaw, "label-any", nil, "Filter entries that contain any label (repeatable)")
	cmd.Flags().StringSliceVar(&withoutRaw, "without-label", nil, "Filter entries that do not contain these labels (repeatable)")
	cmd.Flags().StringSliceVar(&dependsOnAllRaw, "depends-on", nil, "Filter entries that depend on all listed entry IDs (repeatable)")
	cmd.Flags().StringSliceVar(&dependsOnAnyRaw, "depends-on-any", nil, "Filter entries that depend on any listed entry ID (repeatable)")
	cmd.Flags().StringSliceVar(&withoutDependsRaw, "without-depends-on", nil, "Filter entries that do not depend on these entry IDs (repeatable)")

	topLevel.AddCommand(cmd)
}

func addAPIEntriesAdd(topLevel *cobra.Command, opts *apiOptions) {
	var (
		collection string
		message    string
		kind       string
		labels     []string
		dependsOn  []string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			msg := strings.TrimSpace(message)
			if msg == "" {
				msg = strings.TrimSpace(strings.Join(args, " "))
			}
			if msg == "" {
				return apiFailure(cmd, opts, "entries.add", expandUserPath(opts.Journal), newAPIError("invalid_argument", "message is required"))
			}

			bullet, err := parseAddBullet(kind)
			if err != nil {
				return apiFailure(cmd, opts, "entries.add", expandUserPath(opts.Journal), err)
			}

			return runAPI(cmd, opts, "entries.add", func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				resolvedCollection := resolveAPICollection(collection)
				e, err := runtime.Service.Add(ctx, resolvedCollection, bullet, msg, glyph.None)
				if err != nil {
					return nil, err
				}
				normalizedLabels := entry.NormalizeLabels(labels)
				if len(normalizedLabels) > 0 {
					e, err = runtime.Service.SetLabels(ctx, e.ID, normalizedLabels)
					if err != nil {
						return nil, err
					}
				}
				normalizedDependsOn := entry.NormalizeDependsOnIDs(dependsOn)
				if len(normalizedDependsOn) > 0 {
					e, err = runtime.Service.SetDependsOn(ctx, e.ID, normalizedDependsOn)
					if err != nil {
						return nil, err
					}
				}
				return map[string]any{
					"entry": toAPIEntryPayload(e),
				}, nil
			})
		},
	}

	cmd.Flags().StringVar(&collection, "collection", "today", "Collection name")
	cmd.Flags().StringVar(&message, "message", "", "Entry message")
	cmd.Flags().StringVar(&kind, "type", string(glyph.Task), "Entry type: task, note, or event")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "Labels to apply to the entry (repeatable)")
	cmd.Flags().StringSliceVar(&dependsOn, "depends-on", nil, "Dependency entry IDs (repeatable)")

	topLevel.AddCommand(cmd)
}

func addAPIEntriesList(topLevel *cobra.Command, opts *apiOptions) {
	var (
		collection         string
		kind               string
		query              string
		all                bool
		labelAllRaw        []string
		labelAnyRaw        []string
		withoutRaw         []string
		dependsOnAllRaw    []string
		dependsOnAnyRaw    []string
		withoutDependsRaw  []string
		labelAll           []string
		labelAny           []string
		withoutLabels      []string
		dependsOnAll       []string
		dependsOnAny       []string
		withoutDependsOnID []string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List entries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			bullet, err := parseSelectorBullet(kind)
			if err != nil {
				return apiFailure(cmd, opts, "entries.list", expandUserPath(opts.Journal), err)
			}
			labelAll = entry.NormalizeLabels(labelAllRaw)
			labelAny = entry.NormalizeLabels(labelAnyRaw)
			withoutLabels = entry.NormalizeLabels(withoutRaw)
			dependsOnAll = entry.NormalizeDependsOnIDs(dependsOnAllRaw)
			dependsOnAny = entry.NormalizeDependsOnIDs(dependsOnAnyRaw)
			withoutDependsOnID = entry.NormalizeDependsOnIDs(withoutDependsRaw)

			return runAPI(cmd, opts, "entries.list", func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				var entries []*entry.Entry
				resolvedCollection := ""
				if all {
					entries = runtime.Persistence.ListAll(ctx)
				} else {
					resolvedCollection = resolveAPICollection(collection)
					entries = runtime.Persistence.List(ctx, resolvedCollection)
				}
				entries = filterEntriesByFilters(entries, apiEntryListFilter{
					Bullet:           bullet,
					Query:            query,
					LabelsAll:        labelAll,
					LabelsAny:        labelAny,
					WithoutLabels:    withoutLabels,
					DependsOnAll:     dependsOnAll,
					DependsOnAny:     dependsOnAny,
					WithoutDependsOn: withoutDependsOnID,
				})

				fields := map[string]any{
					"count":   len(entries),
					"entries": toAPIEntryPayloads(entries),
				}
				if resolvedCollection != "" {
					fields["collection"] = resolvedCollection
				}
				if bullet != glyph.Any {
					fields["type"] = string(bullet)
				}
				if strings.TrimSpace(query) != "" {
					fields["query"] = strings.TrimSpace(query)
				}
				if len(labelAll) > 0 {
					fields["label"] = labelAll
				}
				if len(labelAny) > 0 {
					fields["label_any"] = labelAny
				}
				if len(withoutLabels) > 0 {
					fields["without_label"] = withoutLabels
				}
				if len(dependsOnAll) > 0 {
					fields["depends_on"] = dependsOnAll
				}
				if len(dependsOnAny) > 0 {
					fields["depends_on_any"] = dependsOnAny
				}
				if len(withoutDependsOnID) > 0 {
					fields["without_depends_on"] = withoutDependsOnID
				}
				return fields, nil
			})
		},
	}

	cmd.Flags().StringVar(&collection, "collection", "today", "Collection name")
	cmd.Flags().StringVar(&kind, "type", string(glyph.Any), "Filter by entry type")
	cmd.Flags().StringVar(&query, "query", "", "Case-insensitive message substring filter")
	cmd.Flags().BoolVar(&all, "all", false, "List across all collections")
	cmd.Flags().StringSliceVar(&labelAllRaw, "label", nil, "Filter entries that contain all labels (repeatable)")
	cmd.Flags().StringSliceVar(&labelAnyRaw, "label-any", nil, "Filter entries that contain any label (repeatable)")
	cmd.Flags().StringSliceVar(&withoutRaw, "without-label", nil, "Filter entries that do not contain these labels (repeatable)")
	cmd.Flags().StringSliceVar(&dependsOnAllRaw, "depends-on", nil, "Filter entries that depend on all listed entry IDs (repeatable)")
	cmd.Flags().StringSliceVar(&dependsOnAnyRaw, "depends-on-any", nil, "Filter entries that depend on any listed entry ID (repeatable)")
	cmd.Flags().StringSliceVar(&withoutDependsRaw, "without-depends-on", nil, "Filter entries that do not depend on these entry IDs (repeatable)")

	topLevel.AddCommand(cmd)
}

func addAPIEntriesResolve(topLevel *cobra.Command, opts *apiOptions) {
	selector := &apiEntrySelectorOptions{}
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve a selector to one entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAPI(cmd, opts, "entries.resolve", func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				e, matchCount, err := resolveEntrySelector(ctx, runtime, *selector)
				if err != nil {
					return nil, err
				}
				fields := map[string]any{"entry": toAPIEntryPayload(e)}
				if matchCount > 1 {
					fields["matched_count"] = matchCount
					fields["selection"] = "first"
				}
				return fields, nil
			})
		},
	}
	addEntrySelectorFlags(cmd, selector, "")
	topLevel.AddCommand(cmd)
}

func addAPIEntriesComplete(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryMutationBySelector(topLevel, opts, "complete", "entries.complete", func(ctx context.Context, runtime apiRuntime, selected *entry.Entry) (*entry.Entry, map[string]any, error) {
		e, err := runtime.Service.Complete(ctx, selected.ID)
		if err != nil {
			return nil, nil, err
		}
		return e, nil, nil
	})
}

func addAPIEntriesStrike(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryMutationBySelector(topLevel, opts, "strike", "entries.strike", func(ctx context.Context, runtime apiRuntime, selected *entry.Entry) (*entry.Entry, map[string]any, error) {
		e, err := runtime.Service.Strike(ctx, selected.ID)
		if err != nil {
			return nil, nil, err
		}
		return e, nil, nil
	})
}

func addAPIEntriesMove(topLevel *cobra.Command, opts *apiOptions) {
	selector := &apiEntrySelectorOptions{}
	var target string

	cmd := &cobra.Command{
		Use:   "move",
		Short: "Move an entry to another collection",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			localSelector := *selector
			if strings.TrimSpace(localSelector.ID) == "" && len(args) > 0 {
				localSelector.ID = strings.TrimSpace(args[0])
			}

			moveTarget := resolveAPITargetCollection(target)
			if moveTarget == "" {
				return apiFailure(cmd, opts, "entries.move", expandUserPath(opts.Journal), newAPIError("invalid_argument", "target collection is required (--target)"))
			}

			return runAPI(cmd, opts, "entries.move", func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				selected, _, err := resolveEntrySelector(ctx, runtime, localSelector)
				if err != nil {
					return nil, err
				}
				moved, err := runtime.Service.Move(ctx, selected.ID, moveTarget)
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"entry":     toAPIEntryPayload(moved),
					"source_id": selected.ID,
				}, nil
			})
		},
	}

	addEntrySelectorFlags(cmd, selector, "")
	cmd.Flags().StringVar(&target, "target", "", "Destination collection")
	topLevel.AddCommand(cmd)
}

func addAPIEntriesParent(topLevel *cobra.Command, opts *apiOptions) {
	cmd := &cobra.Command{
		Use:   "parent",
		Short: "Manage entry parent relationships",
	}

	addAPIEntriesParentSet(cmd, opts)
	addAPIEntriesParentUnset(cmd, opts)

	topLevel.AddCommand(cmd)
}

func addAPIEntriesParentSet(topLevel *cobra.Command, opts *apiOptions) {
	childSelector := &apiEntrySelectorOptions{}
	parentSelector := &apiEntrySelectorOptions{}

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set an entry parent",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			localChild := *childSelector
			localParent := *parentSelector
			if strings.TrimSpace(localChild.ID) == "" && len(args) > 0 {
				localChild.ID = strings.TrimSpace(args[0])
			}
			if strings.TrimSpace(localParent.ID) == "" && len(args) > 1 {
				localParent.ID = strings.TrimSpace(args[1])
			}

			if !localParent.hasAnySelector() {
				return apiFailure(cmd, opts, "entries.parent.set", expandUserPath(opts.Journal), newAPIError("invalid_argument", "parent selector is required"))
			}

			return runAPI(cmd, opts, "entries.parent.set", func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				child, _, err := resolveEntrySelector(ctx, runtime, localChild)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(localParent.Collection) == "" {
					localParent.Collection = child.Collection
				}
				parent, _, err := resolveEntrySelector(ctx, runtime, localParent)
				if err != nil {
					return nil, err
				}
				updated, err := runtime.Service.SetParent(ctx, child.ID, parent.ID)
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"entry":     toAPIEntryPayload(updated),
					"parent_id": parent.ID,
				}, nil
			})
		},
	}

	addEntrySelectorFlags(cmd, childSelector, "")
	addEntrySelectorFlags(cmd, parentSelector, "parent")
	topLevel.AddCommand(cmd)
}

func addAPIEntriesParentUnset(topLevel *cobra.Command, opts *apiOptions) {
	childSelector := &apiEntrySelectorOptions{}

	cmd := &cobra.Command{
		Use:   "unset",
		Short: "Remove an entry parent",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			localSelector := *childSelector
			if strings.TrimSpace(localSelector.ID) == "" && len(args) > 0 {
				localSelector.ID = strings.TrimSpace(args[0])
			}
			return runAPI(cmd, opts, "entries.parent.unset", func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				child, _, err := resolveEntrySelector(ctx, runtime, localSelector)
				if err != nil {
					return nil, err
				}
				updated, err := runtime.Service.SetParent(ctx, child.ID, "")
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"entry": toAPIEntryPayload(updated),
				}, nil
			})
		},
	}

	addEntrySelectorFlags(cmd, childSelector, "")
	topLevel.AddCommand(cmd)
}

func addAPIEntriesLabels(topLevel *cobra.Command, opts *apiOptions) {
	cmd := &cobra.Command{
		Use:   "labels",
		Short: "Manage entry labels",
	}

	addAPIEntriesLabelsAdd(cmd, opts)
	addAPIEntriesLabelsRemove(cmd, opts)
	addAPIEntriesLabelsSet(cmd, opts)
	addAPIEntriesLabelsClear(cmd, opts)

	topLevel.AddCommand(cmd)
}

func addAPIEntriesDependencies(topLevel *cobra.Command, opts *apiOptions) {
	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Manage entry dependencies",
	}

	addAPIEntriesDependenciesAdd(cmd, opts)
	addAPIEntriesDependenciesRemove(cmd, opts)
	addAPIEntriesDependenciesSet(cmd, opts)
	addAPIEntriesDependenciesClear(cmd, opts)

	topLevel.AddCommand(cmd)
}

func addAPIEntriesLabelsAdd(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryLabelMutation(topLevel, opts, "add", "entries.labels.add", true, func(ctx context.Context, runtime apiRuntime, selected *entry.Entry, labels []string) (*entry.Entry, error) {
		return runtime.Service.AddLabels(ctx, selected.ID, labels)
	})
}

func addAPIEntriesLabelsRemove(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryLabelMutation(topLevel, opts, "remove", "entries.labels.remove", true, func(ctx context.Context, runtime apiRuntime, selected *entry.Entry, labels []string) (*entry.Entry, error) {
		return runtime.Service.RemoveLabels(ctx, selected.ID, labels)
	})
}

func addAPIEntriesLabelsSet(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryLabelMutation(topLevel, opts, "set", "entries.labels.set", false, func(ctx context.Context, runtime apiRuntime, selected *entry.Entry, labels []string) (*entry.Entry, error) {
		return runtime.Service.SetLabels(ctx, selected.ID, labels)
	})
}

func addAPIEntriesLabelsClear(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryMutationBySelector(topLevel, opts, "clear", "entries.labels.clear", func(ctx context.Context, runtime apiRuntime, selected *entry.Entry) (*entry.Entry, map[string]any, error) {
		e, err := runtime.Service.ClearLabels(ctx, selected.ID)
		if err != nil {
			return nil, nil, err
		}
		return e, nil, nil
	})
}

func addAPIEntriesDependenciesAdd(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryDependsOnMutation(topLevel, opts, "add", "entries.dependencies.add", true, func(ctx context.Context, runtime apiRuntime, selected *entry.Entry, dependsOn []string) (*entry.Entry, error) {
		return runtime.Service.AddDependsOn(ctx, selected.ID, dependsOn)
	})
}

func addAPIEntriesDependenciesRemove(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryDependsOnMutation(topLevel, opts, "remove", "entries.dependencies.remove", true, func(ctx context.Context, runtime apiRuntime, selected *entry.Entry, dependsOn []string) (*entry.Entry, error) {
		return runtime.Service.RemoveDependsOn(ctx, selected.ID, dependsOn)
	})
}

func addAPIEntriesDependenciesSet(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryDependsOnMutation(topLevel, opts, "set", "entries.dependencies.set", false, func(ctx context.Context, runtime apiRuntime, selected *entry.Entry, dependsOn []string) (*entry.Entry, error) {
		return runtime.Service.SetDependsOn(ctx, selected.ID, dependsOn)
	})
}

func addAPIEntriesDependenciesClear(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryMutationBySelector(topLevel, opts, "clear", "entries.dependencies.clear", func(ctx context.Context, runtime apiRuntime, selected *entry.Entry) (*entry.Entry, map[string]any, error) {
		e, err := runtime.Service.ClearDependsOn(ctx, selected.ID)
		if err != nil {
			return nil, nil, err
		}
		return e, nil, nil
	})
}

func addAPIEntryLabelMutation(topLevel *cobra.Command, opts *apiOptions, use, action string, requireLabels bool, mutate func(context.Context, apiRuntime, *entry.Entry, []string) (*entry.Entry, error)) {
	selector := &apiEntrySelectorOptions{}
	var labels []string

	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("Mutate entry labels: %s", use),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			localSelector := *selector
			if strings.TrimSpace(localSelector.ID) == "" && len(args) > 0 {
				localSelector.ID = strings.TrimSpace(args[0])
			}
			normalized := entry.NormalizeLabels(labels)
			if requireLabels && len(normalized) == 0 {
				return apiFailure(cmd, opts, action, expandUserPath(opts.Journal), newAPIError("invalid_argument", "at least one --label is required"))
			}

			return runAPI(cmd, opts, action, func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				selected, matchCount, err := resolveEntrySelector(ctx, runtime, localSelector)
				if err != nil {
					return nil, err
				}
				updated, err := mutate(ctx, runtime, selected, normalized)
				if err != nil {
					return nil, err
				}
				fields := map[string]any{
					"entry": toAPIEntryPayload(updated),
				}
				if matchCount > 1 {
					fields["matched_count"] = matchCount
					fields["selection"] = "first"
				}
				return fields, nil
			})
		},
	}

	addEntrySelectorFlags(cmd, selector, "")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "Labels (repeatable)")
	topLevel.AddCommand(cmd)
}

func addAPIEntryDependsOnMutation(topLevel *cobra.Command, opts *apiOptions, use, action string, requireIDs bool, mutate func(context.Context, apiRuntime, *entry.Entry, []string) (*entry.Entry, error)) {
	selector := &apiEntrySelectorOptions{}
	var dependsOn []string

	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("Mutate entry dependencies: %s", use),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			localSelector := *selector
			if strings.TrimSpace(localSelector.ID) == "" && len(args) > 0 {
				localSelector.ID = strings.TrimSpace(args[0])
			}
			normalized := entry.NormalizeDependsOnIDs(dependsOn)
			if requireIDs && len(normalized) == 0 {
				return apiFailure(cmd, opts, action, expandUserPath(opts.Journal), newAPIError("invalid_argument", "at least one --depends-on is required"))
			}

			return runAPI(cmd, opts, action, func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				selected, matchCount, err := resolveEntrySelector(ctx, runtime, localSelector)
				if err != nil {
					return nil, err
				}
				updated, err := mutate(ctx, runtime, selected, normalized)
				if err != nil {
					return nil, err
				}
				fields := map[string]any{
					"entry": toAPIEntryPayload(updated),
				}
				if matchCount > 1 {
					fields["matched_count"] = matchCount
					fields["selection"] = "first"
				}
				return fields, nil
			})
		},
	}

	addEntrySelectorFlags(cmd, selector, "")
	cmd.Flags().StringSliceVar(&dependsOn, "depends-on", nil, "Dependency entry IDs (repeatable)")
	topLevel.AddCommand(cmd)
}

func addAPIEntriesLock(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryMutationBySelector(topLevel, opts, "lock", "entries.lock", func(ctx context.Context, runtime apiRuntime, selected *entry.Entry) (*entry.Entry, map[string]any, error) {
		e, err := runtime.Service.Lock(ctx, selected.ID)
		if err != nil {
			return nil, nil, err
		}
		return e, nil, nil
	})
}

func addAPIEntriesUnlock(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryMutationBySelector(topLevel, opts, "unlock", "entries.unlock", func(ctx context.Context, runtime apiRuntime, selected *entry.Entry) (*entry.Entry, map[string]any, error) {
		e, err := runtime.Service.Unlock(ctx, selected.ID)
		if err != nil {
			return nil, nil, err
		}
		return e, nil, nil
	})
}

func addAPIEntriesDelete(topLevel *cobra.Command, opts *apiOptions) {
	addAPIEntryMutationBySelector(topLevel, opts, "delete", "entries.delete", func(ctx context.Context, runtime apiRuntime, selected *entry.Entry) (*entry.Entry, map[string]any, error) {
		if err := runtime.Service.Delete(ctx, selected.ID); err != nil {
			return nil, nil, err
		}
		return selected, map[string]any{"deleted": true}, nil
	})
}

func addAPIEntryMutationBySelector(topLevel *cobra.Command, opts *apiOptions, use string, action string, mutate func(context.Context, apiRuntime, *entry.Entry) (*entry.Entry, map[string]any, error)) {
	selector := &apiEntrySelectorOptions{}

	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("Mutate an entry: %s", use),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			localSelector := *selector
			if strings.TrimSpace(localSelector.ID) == "" && len(args) > 0 {
				localSelector.ID = strings.TrimSpace(args[0])
			}

			return runAPI(cmd, opts, action, func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				selected, matchCount, err := resolveEntrySelector(ctx, runtime, localSelector)
				if err != nil {
					return nil, err
				}
				mutated, extra, err := mutate(ctx, runtime, selected)
				if err != nil {
					return nil, err
				}
				fields := map[string]any{
					"entry": toAPIEntryPayload(mutated),
				}
				if matchCount > 1 {
					fields["matched_count"] = matchCount
					fields["selection"] = "first"
				}
				for k, v := range extra {
					fields[k] = v
				}
				return fields, nil
			})
		},
	}

	addEntrySelectorFlags(cmd, selector, "")
	topLevel.AddCommand(cmd)
}

func addEntrySelectorFlags(cmd *cobra.Command, selector *apiEntrySelectorOptions, prefix string) {
	idFlag := selectorFlagName(prefix, "id")
	msgFlag := selectorFlagName(prefix, "message")
	collectionFlag := selectorFlagName(prefix, "collection")
	typeFlag := selectorFlagName(prefix, "type")
	exactFlag := selectorFlagName(prefix, "exact")
	firstFlag := selectorFlagName(prefix, "first")

	cmd.Flags().StringVar(&selector.ID, idFlag, "", "Match entry by ID")
	cmd.Flags().StringVar(&selector.Message, msgFlag, "", "Match entry by message")
	cmd.Flags().StringVar(&selector.Collection, collectionFlag, "", "Match entry by collection (supports 'today')")
	cmd.Flags().StringVar(&selector.Type, typeFlag, string(glyph.Any), "Match entry by type")
	cmd.Flags().BoolVar(&selector.Exact, exactFlag, false, "Require exact message match")
	cmd.Flags().BoolVar(&selector.First, firstFlag, false, "Allow first match when selector is ambiguous")
}

func selectorFlagName(prefix, name string) string {
	if strings.TrimSpace(prefix) == "" {
		return name
	}
	return prefix + "-" + name
}

func resolveEntrySelector(ctx context.Context, runtime apiRuntime, selector apiEntrySelectorOptions) (*entry.Entry, int, error) {
	if !selector.hasAnySelector() {
		return nil, 0, newAPIError("invalid_argument", "selector required: use --id, --message, --collection, or --type")
	}

	bullet, err := parseSelectorBullet(selector.Type)
	if err != nil {
		return nil, 0, err
	}

	candidates := runtime.Persistence.ListAll(ctx)
	matches := make([]*entry.Entry, 0, len(candidates))
	idFilter := strings.TrimSpace(selector.ID)
	messageFilter := strings.TrimSpace(selector.Message)
	collectionFilter := strings.TrimSpace(selector.Collection)
	if collectionFilter != "" {
		collectionFilter = resolveAPICollection(collectionFilter)
	}

	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if idFilter != "" && candidate.ID != idFilter {
			continue
		}
		if collectionFilter != "" && !strings.EqualFold(candidate.Collection, collectionFilter) {
			continue
		}
		if bullet != glyph.Any && candidate.Bullet != bullet {
			continue
		}
		if messageFilter != "" {
			candidateMessage := strings.TrimSpace(candidate.Message)
			if selector.Exact {
				if !strings.EqualFold(candidateMessage, messageFilter) {
					continue
				}
			} else {
				if !strings.Contains(strings.ToLower(candidateMessage), strings.ToLower(messageFilter)) {
					continue
				}
			}
		}
		matches = append(matches, candidate)
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Collection != matches[j].Collection {
			return matches[i].Collection < matches[j].Collection
		}
		return matches[i].ID < matches[j].ID
	})

	switch len(matches) {
	case 0:
		return nil, 0, newAPIError("entry_not_found", "no entry matched selector")
	case 1:
		return matches[0], 1, nil
	default:
		if !selector.First {
			return nil, len(matches), newAPIError("ambiguous_match", fmt.Sprintf("%d entries matched selector; pass --first to accept first match", len(matches)))
		}
		return matches[0], len(matches), nil
	}
}

func parseAddBullet(kind string) (glyph.Bullet, error) {
	bullet, err := glyph.BulletForAlias(kind)
	if err != nil {
		return glyph.Any, newAPIError("invalid_argument", err.Error())
	}
	switch bullet {
	case glyph.Task, glyph.Note, glyph.Event:
		return bullet, nil
	default:
		return glyph.Any, newAPIError("invalid_argument", "type must be one of: task, note, event")
	}
}

func parseSelectorBullet(kind string) (glyph.Bullet, error) {
	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		return glyph.Any, nil
	}
	bullet, err := glyph.BulletForAlias(trimmed)
	if err != nil {
		return glyph.Any, newAPIError("invalid_argument", err.Error())
	}
	return bullet, nil
}

func resolveAPICollection(collection string) string {
	trimmed := strings.TrimSpace(collection)
	if trimmed == "" || strings.EqualFold(trimmed, "today") {
		return time.Now().Format(apiLayoutUS)
	}
	return trimmed
}

func resolveAPITargetCollection(collection string) string {
	trimmed := strings.TrimSpace(collection)
	if trimmed == "" {
		return ""
	}
	if strings.EqualFold(trimmed, "today") {
		return resolveAPICollection(trimmed)
	}
	return trimmed
}

type apiEntryListFilter struct {
	Bullet           glyph.Bullet
	Query            string
	LabelsAll        []string
	LabelsAny        []string
	WithoutLabels    []string
	DependsOnAll     []string
	DependsOnAny     []string
	WithoutDependsOn []string
}

func filterEntriesByFilters(entries []*entry.Entry, filter apiEntryListFilter) []*entry.Entry {
	if filter.Bullet == glyph.Any &&
		strings.TrimSpace(filter.Query) == "" &&
		len(filter.LabelsAll) == 0 &&
		len(filter.LabelsAny) == 0 &&
		len(filter.WithoutLabels) == 0 &&
		len(filter.DependsOnAll) == 0 &&
		len(filter.DependsOnAny) == 0 &&
		len(filter.WithoutDependsOn) == 0 {
		return entries
	}
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	filtered := make([]*entry.Entry, 0, len(entries))
	for _, e := range entries {
		if e == nil {
			continue
		}
		if filter.Bullet != glyph.Any && e.Bullet != filter.Bullet {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(strings.TrimSpace(e.Message)), query) {
			continue
		}
		if len(filter.LabelsAll) == 0 &&
			len(filter.LabelsAny) == 0 &&
			len(filter.WithoutLabels) == 0 &&
			len(filter.DependsOnAll) == 0 &&
			len(filter.DependsOnAny) == 0 &&
			len(filter.WithoutDependsOn) == 0 {
			filtered = append(filtered, e)
			continue
		}
		candidateLabels := entry.NormalizeLabels(e.Labels)
		labelSet := make(map[string]struct{}, len(candidateLabels))
		for _, label := range candidateLabels {
			labelSet[label] = struct{}{}
		}
		if !labelsContainAll(labelSet, filter.LabelsAll) {
			continue
		}
		if !labelsContainAny(labelSet, filter.LabelsAny) {
			continue
		}
		if len(filter.WithoutLabels) > 0 && labelsContainAny(labelSet, filter.WithoutLabels) {
			continue
		}
		dependsOnIDs := entry.NormalizeDependsOnIDs(e.DependsOn)
		depSet := make(map[string]struct{}, len(dependsOnIDs))
		for _, id := range dependsOnIDs {
			depSet[id] = struct{}{}
		}
		if !setContainsAll(depSet, filter.DependsOnAll) {
			continue
		}
		if !setContainsAny(depSet, filter.DependsOnAny) {
			continue
		}
		if len(filter.WithoutDependsOn) > 0 && setContainsAny(depSet, filter.WithoutDependsOn) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func labelsContainAll(candidate map[string]struct{}, required []string) bool {
	return setContainsAll(candidate, required)
}

func labelsContainAny(candidate map[string]struct{}, labels []string) bool {
	return setContainsAny(candidate, labels)
}

func setContainsAll(candidate map[string]struct{}, required []string) bool {
	if len(required) == 0 {
		return true
	}
	for _, item := range required {
		if _, ok := candidate[item]; !ok {
			return false
		}
	}
	return true
}

func setContainsAny(candidate map[string]struct{}, values []string) bool {
	if len(values) == 0 {
		return true
	}
	for _, item := range values {
		if _, ok := candidate[item]; ok {
			return true
		}
	}
	return false
}

func partitionByDependencyState(candidates []*entry.Entry, allEntries []*entry.Entry) ([]*entry.Entry, []apiBlockedEntryPayload) {
	indexed := indexEntriesByID(allEntries)
	ready := make([]*entry.Entry, 0, len(candidates))
	blocked := make([]apiBlockedEntryPayload, 0, len(candidates))
	for _, candidate := range candidates {
		if !isDependencyActionable(candidate) {
			continue
		}
		unmet := unmetDependencyIDs(candidate, indexed)
		if len(unmet) == 0 {
			ready = append(ready, candidate)
			continue
		}
		blocked = append(blocked, apiBlockedEntryPayload{
			Entry:          toAPIEntryPayload(candidate),
			UnmetDependsOn: unmet,
		})
	}
	sort.SliceStable(ready, func(i, j int) bool {
		if ready[i].Collection != ready[j].Collection {
			return ready[i].Collection < ready[j].Collection
		}
		return ready[i].ID < ready[j].ID
	})
	sort.SliceStable(blocked, func(i, j int) bool {
		if blocked[i].Entry.Collection != blocked[j].Entry.Collection {
			return blocked[i].Entry.Collection < blocked[j].Entry.Collection
		}
		return blocked[i].Entry.ID < blocked[j].Entry.ID
	})
	return ready, blocked
}

func isDependencyActionable(e *entry.Entry) bool {
	if e == nil || e.ID == "" {
		return false
	}
	if e.Immutable {
		return false
	}
	switch e.Bullet {
	case glyph.Task, glyph.Note, glyph.Event:
		return true
	default:
		return false
	}
}

func unmetDependencyIDs(e *entry.Entry, indexed map[string]*entry.Entry) []string {
	dependsOn := entry.NormalizeDependsOnIDs(e.DependsOn)
	if len(dependsOn) == 0 {
		return nil
	}
	unmet := make([]string, 0, len(dependsOn))
	for _, depID := range dependsOn {
		dep := indexed[depID]
		if !isDependencySatisfied(dep) {
			unmet = append(unmet, depID)
		}
	}
	if len(unmet) == 0 {
		return nil
	}
	return unmet
}

func isDependencySatisfied(dep *entry.Entry) bool {
	if dep == nil {
		return false
	}
	switch dep.Bullet {
	case glyph.Completed, glyph.Irrelevant, glyph.MovedCollection, glyph.MovedFuture:
		return true
	default:
		return false
	}
}

func indexEntriesByID(entries []*entry.Entry) map[string]*entry.Entry {
	indexed := make(map[string]*entry.Entry, len(entries))
	for _, e := range entries {
		if e == nil || strings.TrimSpace(e.ID) == "" {
			continue
		}
		indexed[e.ID] = e
	}
	return indexed
}
