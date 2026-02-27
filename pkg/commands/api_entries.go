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
	addAPIEntriesResolve(cmd, opts)
	addAPIEntriesComplete(cmd, opts)
	addAPIEntriesStrike(cmd, opts)
	addAPIEntriesMove(cmd, opts)
	addAPIEntriesParent(cmd, opts)
	addAPIEntriesLock(cmd, opts)
	addAPIEntriesUnlock(cmd, opts)
	addAPIEntriesDelete(cmd, opts)

	topLevel.AddCommand(cmd)
}

func addAPIEntriesAdd(topLevel *cobra.Command, opts *apiOptions) {
	var (
		collection string
		message    string
		kind       string
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
				return map[string]any{
					"entry": toAPIEntryPayload(e),
				}, nil
			})
		},
	}

	cmd.Flags().StringVar(&collection, "collection", "today", "Collection name")
	cmd.Flags().StringVar(&message, "message", "", "Entry message")
	cmd.Flags().StringVar(&kind, "type", string(glyph.Task), "Entry type: task, note, or event")

	topLevel.AddCommand(cmd)
}

func addAPIEntriesList(topLevel *cobra.Command, opts *apiOptions) {
	var (
		collection string
		kind       string
		all        bool
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

			return runAPI(cmd, opts, "entries.list", func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				var entries []*entry.Entry
				resolvedCollection := ""
				if all {
					entries = runtime.Persistence.ListAll(ctx)
				} else {
					resolvedCollection = resolveAPICollection(collection)
					entries = runtime.Persistence.List(ctx, resolvedCollection)
				}
				entries = filterEntriesByBullet(entries, bullet)

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
				return fields, nil
			})
		},
	}

	cmd.Flags().StringVar(&collection, "collection", "today", "Collection name")
	cmd.Flags().StringVar(&kind, "type", string(glyph.Any), "Filter by entry type")
	cmd.Flags().BoolVar(&all, "all", false, "List across all collections")

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
		deleted := *selected
		if err := runtime.Service.Delete(ctx, selected.ID); err != nil {
			return nil, nil, err
		}
		return &deleted, map[string]any{"deleted": true}, nil
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

func filterEntriesByBullet(entries []*entry.Entry, bullet glyph.Bullet) []*entry.Entry {
	if bullet == glyph.Any {
		return entries
	}
	filtered := make([]*entry.Entry, 0, len(entries))
	for _, e := range entries {
		if e != nil && e.Bullet == bullet {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
