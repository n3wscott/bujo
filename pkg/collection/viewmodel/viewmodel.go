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
	Exists   bool
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
	now        time.Time
	useNow     bool
}

// BuildTree converts flat collection metadata into a hierarchical structure.
func BuildTree(metas []collection.Meta, opts ...Option) []*ParsedCollection {
	if len(metas) == 0 {
		if len(opts) == 0 {
			return nil
		}
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
	if config.useNow {
		now := config.now
		if now.IsZero() {
			now = time.Now()
		}
		ensureCurrentMonth(nodes, now, config)
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

	sortCollections(roots, config)
	for _, node := range nodes {
		if node.Type == collection.TypeDaily {
			node.Days = daySummaries(node.Children)
		}
	}
	return roots
}

// SortTree reorders parsed collections (and their descendants) using the same
// priority rules as BuildTree. It is useful after mutating a tree in-place.
func SortTree(nodes []*ParsedCollection) {
	sortCollections(nodes, nil)
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
		Exists:   true,
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

func sortCollections(nodes []*ParsedCollection, opts *buildOptions) {
	sortCollectionsWithParent(nodes, nil, opts)
}

func sortCollectionsWithParent(nodes []*ParsedCollection, parent *ParsedCollection, opts *buildOptions) {
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
		if parent == nil && opts != nil && opts.useNow {
			now := opts.now
			if now.IsZero() {
				now = time.Now()
			}
			groupA := rootGroup(nodes[i])
			groupB := rootGroup(nodes[j])
			if groupA != groupB {
				return groupA < groupB
			}
			if groupA == rootGroupDaily {
				return compareDailyMonths(nodes[i], nodes[j], now)
			}
		}
		return compareDefault(nodes[i], nodes[j])
	})
	for _, node := range nodes {
		if len(node.Children) == 0 {
			continue
		}
		sortCollectionsWithParent(node.Children, node, opts)
	}
}

func daySummaries(children []*ParsedCollection) []DaySummary {
	if len(children) == 0 {
		return nil
	}
	days := make([]DaySummary, 0, len(children))
	for _, child := range children {
		if child == nil || !child.Exists {
			continue
		}
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

const (
	rootGroupFuture = iota
	rootGroupDaily
	rootGroupOther
)

func rootGroup(node *ParsedCollection) int {
	if node == nil {
		return rootGroupOther
	}
	if strings.EqualFold(strings.TrimSpace(node.ID), "Future") {
		return rootGroupFuture
	}
	if node.Type == collection.TypeDaily {
		return rootGroupDaily
	}
	return rootGroupOther
}

func compareDefault(a, b *ParsedCollection) bool {
	if a == nil || b == nil {
		return a != nil
	}
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	if a.SortKey != b.SortKey {
		return a.SortKey < b.SortKey
	}
	return a.Name < b.Name
}

func compareDailyMonths(a, b *ParsedCollection, now time.Time) bool {
	ma := monthForCollection(a)
	mb := monthForCollection(b)
	if ma.IsZero() || mb.IsZero() {
		return compareDefault(a, b)
	}
	groupA, rankA := monthRank(now, ma)
	groupB, rankB := monthRank(now, mb)
	if groupA != groupB {
		return groupA < groupB
	}
	if rankA != rankB {
		return rankA < rankB
	}
	return compareDefault(a, b)
}

func monthForCollection(node *ParsedCollection) time.Time {
	if node == nil {
		return time.Time{}
	}
	if !node.Month.IsZero() {
		return node.Month
	}
	if collection.IsMonthName(node.Name) {
		if t, err := time.Parse(monthFormat, node.Name); err == nil {
			return t
		}
	}
	return time.Time{}
}

func monthRank(now, month time.Time) (int, int) {
	now = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	if now.Equal(month) {
		return 0, 0
	}
	if month.Before(now) {
		return 1, monthsBetween(month, now)
	}
	return 2, monthsBetween(now, month)
}

func monthsBetween(start, end time.Time) int {
	if end.Before(start) {
		start, end = end, start
	}
	yearDiff := end.Year() - start.Year()
	monthDiff := int(end.Month()) - int(start.Month())
	return yearDiff*12 + monthDiff
}

func ensureCurrentMonth(nodes map[string]*ParsedCollection, now time.Time, opts *buildOptions) {
	if nodes == nil {
		return
	}
	monthName := now.Format(monthFormat)
	if _, ok := nodes[monthName]; ok {
		return
	}
	meta := collection.Meta{Name: monthName, Type: collection.TypeDaily}
	node := newParsedCollection(meta, opts)
	node.Exists = false
	nodes[node.ID] = node
}

// WithNow enables calendar-aware ordering and ensures the current month appears
// in the parsed tree even when no entries exist yet.
func WithNow(now time.Time) Option {
	return func(opts *buildOptions) {
		if opts == nil {
			return
		}
		opts.now = now
		opts.useNow = true
	}
}
