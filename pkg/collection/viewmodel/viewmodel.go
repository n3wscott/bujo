package viewmodel

import (
	"sort"
	"strings"
	"time"

	"tableflip.dev/bujo/pkg/collection"
)

const (
	monthFormat = "January 2006"
	dayFormat   = "January 2, 2006"
)

// ParsedCollection describes a collection enriched with hierarchy and
// type-specific metadata so UI layers can reason about structure without
// re-parsing names all over the codebase.
type ParsedCollection struct {
	ID       string
	Name     string
	Type     collection.Type
	ParentID string
	Depth    int

	Priority int
	SortKey  string

	Month time.Time
	Day   time.Time

	Days     []DaySummary
	Children []*ParsedCollection
}

// DaySummary captures child day collections for daily parents.
type DaySummary struct {
	ID   string
	Name string
	Date time.Time
}

// Option customises BuildTree behaviour.
type Option func(*buildOptions)

// WithPriorities overrides the computed priority for specific collections.
func WithPriorities(m map[string]int) Option {
	return func(opts *buildOptions) {
		if len(m) == 0 {
			return
		}
		opts.priorities = make(map[string]int, len(m))
		for k, v := range m {
			opts.priorities[strings.TrimSpace(k)] = v
		}
	}
}

type buildOptions struct {
	priorities map[string]int
}

// BuildTree converts flat collection metadata into a hierarchical structure.
func BuildTree(metas []collection.Meta, opts ...Option) []*ParsedCollection {
	if len(metas) == 0 {
		return nil
	}
	config := &buildOptions{}
	for _, opt := range opts {
		opt(config)
	}

	nodes := make(map[string]*ParsedCollection, len(metas))
	for _, meta := range metas {
		node := newParsedCollection(meta, config)
		nodes[node.ID] = node
	}

	var roots []*ParsedCollection
	for _, node := range nodes {
		if node.ParentID == "" {
			roots = append(roots, node)
			continue
		}
		parent, ok := nodes[node.ParentID]
		if !ok {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}

	sortCollections(roots)
	for _, node := range nodes {
		if node.Type == collection.TypeDaily {
			node.Days = daySummaries(node.Children)
		}
	}
	return roots
}

func newParsedCollection(meta collection.Meta, opts *buildOptions) *ParsedCollection {
	fullName := strings.TrimSpace(meta.Name)
	if fullName == "" {
		fullName = "Unnamed"
	}
	parts := strings.Split(fullName, "/")
	name := parts[len(parts)-1]
	parentID := ""
	if len(parts) > 1 {
		parentID = strings.Join(parts[:len(parts)-1], "/")
	}
	depth := len(parts) - 1
	typ := meta.Type
	if typ == "" {
		typ = collection.TypeGeneric
	}
	node := &ParsedCollection{
		ID:       fullName,
		Name:     name,
		Type:     typ,
		ParentID: parentID,
		Depth:    depth,
		Priority: defaultPriority(typ, depth),
		SortKey:  defaultSortKey(name),
	}
	if opts != nil && opts.priorities != nil {
		if priority, ok := opts.priorities[fullName]; ok {
			node.Priority = priority
		}
	}
	if collection.IsMonthName(name) {
		if t, err := time.Parse(monthFormat, name); err == nil {
			node.Month = t
		}
	}
	if collection.IsDayName(name) {
		if t, err := time.Parse(dayFormat, name); err == nil {
			node.Day = t
		}
	}
	return node
}

func sortCollections(nodes []*ParsedCollection) {
	sortCollectionsWithParent(nodes, nil)
}

func sortCollectionsWithParent(nodes []*ParsedCollection, parent *ParsedCollection) {
	sort.Slice(nodes, func(i, j int) bool {
		if parent != nil && parent.Type == collection.TypeDaily {
			di := nodes[i].Day
			dj := nodes[j].Day
			if !di.IsZero() || !dj.IsZero() {
				if di.Equal(dj) {
					return nodes[i].Name < nodes[j].Name
				}
				if di.IsZero() {
					return false
				}
				if dj.IsZero() {
					return true
				}
				return di.Before(dj)
			}
		}
		if nodes[i].Priority != nodes[j].Priority {
			return nodes[i].Priority < nodes[j].Priority
		}
		if nodes[i].SortKey != nodes[j].SortKey {
			return nodes[i].SortKey < nodes[j].SortKey
		}
		return nodes[i].Name < nodes[j].Name
	})
	for _, node := range nodes {
		if len(node.Children) == 0 {
			continue
		}
		sortCollectionsWithParent(node.Children, node)
	}
}

func daySummaries(children []*ParsedCollection) []DaySummary {
	if len(children) == 0 {
		return nil
	}
	days := make([]DaySummary, 0, len(children))
	for _, child := range children {
		if child.Day.IsZero() {
			continue
		}
		days = append(days, DaySummary{
			ID:   child.ID,
			Name: child.Name,
			Date: child.Day,
		})
	}
	sort.Slice(days, func(i, j int) bool {
		if days[i].Date.Equal(days[j].Date) {
			return days[i].Name < days[j].Name
		}
		return days[i].Date.Before(days[j].Date)
	})
	return days
}

func defaultPriority(typ collection.Type, depth int) int {
	switch typ {
	case collection.TypeMonthly:
		return 10 + depth
	case collection.TypeDaily:
		return 20 + depth
	case collection.TypeTracking:
		return 30 + depth
	default:
		return 40 + depth
	}
}

func defaultSortKey(name string) string {
	return strings.ToLower(name)
}
