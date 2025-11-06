package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	viewmodel "tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/timeutil"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
	"tableflip.dev/bujo/pkg/tui/uiutil"
)

type migrationWindow struct {
	HasWindow bool
	Duration  time.Duration
	Label     string
	Since     time.Time
	Until     time.Time
}

func resolveMigrationWindow(now time.Time, spec string) (migrationWindow, error) {
	if now.IsZero() {
		now = time.Now()
	}
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return migrationWindow{
			HasWindow: false,
			Duration:  0,
			Label:     "all open tasks",
			Since:     time.Time{},
			Until:     now,
		}, nil
	}
	duration, label, err := timeutil.ParseWindow(trimmed)
	if err != nil {
		return migrationWindow{}, err
	}
	since := now.Add(-duration)
	return migrationWindow{
		HasWindow: true,
		Duration:  duration,
		Label:     "last " + label,
		Since:     since,
		Until:     now,
	}, nil
}

type migrationBullet struct {
	Candidate     app.MigrationCandidate
	SectionID     string
	SectionTitle  string
	SectionType   collection.Type
	CollectionRef events.CollectionRef
	ParentLabel   string
}

type migrationData struct {
	now         time.Time
	order       []string
	buckets     map[string][]*migrationBullet
	bulletsByID map[string]*migrationBullet
	sections    []collectiondetail.Section
}

func newMigrationData(now time.Time) *migrationData {
	if now.IsZero() {
		now = time.Now()
	}
	return &migrationData{
		now:         now,
		buckets:     make(map[string][]*migrationBullet),
		bulletsByID: make(map[string]*migrationBullet),
	}
}

func (d *migrationData) cloneSections() []collectiondetail.Section {
	if len(d.sections) == 0 {
		return nil
	}
	out := make([]collectiondetail.Section, len(d.sections))
	for i := range d.sections {
		sec := d.sections[i]
		clone := collectiondetail.Section{
			ID:          sec.ID,
			Title:       sec.Title,
			Subtitle:    sec.Subtitle,
			Placeholder: sec.Placeholder,
			Bullets:     make([]collectiondetail.Bullet, len(sec.Bullets)),
		}
		copy(clone.Bullets, sec.Bullets)
		out[i] = clone
	}
	return out
}

func (d *migrationData) Sections() []collectiondetail.Section {
	return d.cloneSections()
}

func (d *migrationData) Bullet(id string) (*migrationBullet, bool) {
	b, ok := d.bulletsByID[strings.TrimSpace(id)]
	return b, ok
}

func (d *migrationData) Remove(id string) bool {
	id = strings.TrimSpace(id)
	bullet, ok := d.bulletsByID[id]
	if !ok {
		return false
	}
	list := d.buckets[bullet.SectionID]
	if len(list) == 0 {
		return false
	}
	updated := make([]*migrationBullet, 0, len(list)-1)
	for _, candidate := range list {
		if candidate.Candidate.Entry == nil || strings.TrimSpace(candidate.Candidate.Entry.ID) == id {
			continue
		}
		updated = append(updated, candidate)
	}
	if len(updated) == 0 {
		delete(d.buckets, bullet.SectionID)
		d.removeOrderEntry(bullet.SectionID)
	} else {
		d.buckets[bullet.SectionID] = updated
	}
	delete(d.bulletsByID, id)
	d.rebuildSections()
	return true
}

func (d *migrationData) removeOrderEntry(collectionID string) {
	if len(d.order) == 0 {
		return
	}
	target := strings.TrimSpace(collectionID)
	next := d.order[:0]
	for _, id := range d.order {
		if strings.EqualFold(id, target) {
			continue
		}
		next = append(next, id)
	}
	d.order = next
}

func (d *migrationData) IsEmpty() bool {
	return len(d.bulletsByID) == 0
}

func (d *migrationData) rebuildSections() {
	if len(d.order) == 0 || len(d.buckets) == 0 {
		d.sections = nil
		return
	}
	sections := make([]collectiondetail.Section, 0, len(d.buckets))
	for _, collectionID := range d.order {
		list := d.buckets[collectionID]
		if len(list) == 0 {
			continue
		}
		section := collectiondetail.Section{
			ID:    collectionID,
			Title: list[0].SectionTitle,
		}
		if list[0].SectionType != "" {
			section.Subtitle = string(list[0].SectionType)
		}
		section.Bullets = make([]collectiondetail.Bullet, 0, len(list))
		for _, bullet := range list {
			if bullet == nil || bullet.Candidate.Entry == nil {
				continue
			}
			section.Bullets = append(section.Bullets, migrationBulletToDetail(d.now, bullet))
		}
		if len(section.Bullets) == 0 {
			continue
		}
		sections = append(sections, section)
	}
	d.sections = sections
}

func migrationBulletToDetail(now time.Time, bullet *migrationBullet) collectiondetail.Bullet {
	entry := bullet.Candidate.Entry
	label := uiutil.EntryLabel(entry)
	if strings.TrimSpace(label) == "" {
		label = strings.TrimSpace(entry.Message)
	}
	note := describeLastTouched(now, bullet.Candidate.LastTouched)
	if bullet.ParentLabel != "" {
		note = fmt.Sprintf("%s — parent: %s", note, bullet.ParentLabel)
	}
	return collectiondetail.Bullet{
		ID:        entry.ID,
		Label:     label,
		Note:      note,
		Bullet:    entry.Bullet,
		Signifier: entry.Signifier,
		Created:   entry.Created.Time,
		Locked:    entry.Immutable,
	}
}

func buildMigrationData(now time.Time, candidates []app.MigrationCandidate, parsed []*viewmodel.ParsedCollection) *migrationData {
	data := newMigrationData(now)
	if len(candidates) == 0 {
		data.sections = nil
		return data
	}
	index := indexParsedCollections(parsed)
	order := make([]string, 0)
	buckets := make(map[string][]*migrationBullet)
	for _, cand := range candidates {
		entry := cand.Entry
		if entry == nil || strings.TrimSpace(entry.ID) == "" {
			continue
		}
		collectionID := strings.TrimSpace(entry.Collection)
		if collectionID == "" {
			collectionID = "(unfiled)"
		}
		if _, ok := buckets[collectionID]; !ok {
			order = append(order, collectionID)
			buckets[collectionID] = make([]*migrationBullet, 0, 4)
		}
		parentLabel := ""
		if cand.Parent != nil {
			parentLabel = entryLabelOrMessage(cand.Parent)
		}
		sectionTitle, sectionType := sectionMetadata(collectionID, index)
		ref := collectionRefFor(collectionID, index)
		item := &migrationBullet{
			Candidate:     cand,
			SectionID:     collectionID,
			SectionTitle:  sectionTitle,
			SectionType:   sectionType,
			CollectionRef: ref,
			ParentLabel:   parentLabel,
		}
		buckets[collectionID] = append(buckets[collectionID], item)
		data.bulletsByID[strings.TrimSpace(entry.ID)] = item
	}
	for _, entries := range buckets {
		sort.SliceStable(entries, func(i, j int) bool {
			left := entries[i].Candidate.LastTouched
			right := entries[j].Candidate.LastTouched
			if left.Equal(right) {
				return strings.TrimSpace(entries[i].Candidate.Entry.Message) < strings.TrimSpace(entries[j].Candidate.Entry.Message)
			}
			return left.After(right)
		})
	}
	data.order = order
	data.buckets = buckets
	data.rebuildSections()
	return data
}

func indexParsedCollections(parsed []*viewmodel.ParsedCollection) map[string]*viewmodel.ParsedCollection {
	index := make(map[string]*viewmodel.ParsedCollection)
	var walk func(list []*viewmodel.ParsedCollection)
	walk = func(list []*viewmodel.ParsedCollection) {
		for _, node := range list {
			if node == nil {
				continue
			}
			id := strings.TrimSpace(node.ID)
			if id == "" {
				continue
			}
			index[id] = node
			if len(node.Children) > 0 {
				walk(node.Children)
			}
		}
	}
	walk(parsed)
	return index
}

func sectionMetadata(collectionID string, index map[string]*viewmodel.ParsedCollection) (string, collection.Type) {
	if node, ok := index[collectionID]; ok && node != nil {
		title := node.Name
		if formatted := uiutil.FormattedCollectionName(node.ID); formatted != "" {
			title = formatted
		} else if friendly := uiutil.FriendlyCollectionName(node.ID); friendly != "" {
			title = friendly
		}
		if strings.TrimSpace(title) == "" {
			title = node.ID
		}
		return title, node.Type
	}
	segments := strings.Split(strings.TrimSpace(collectionID), "/")
	last := segments[len(segments)-1]
	if formatted := uiutil.FormattedCollectionName(collectionID); formatted != "" {
		last = formatted
	} else if friendly := uiutil.FriendlyCollectionName(collectionID); friendly != "" {
		last = friendly
	}
	return last, collection.GuessType(last, collection.TypeGeneric)
}

func collectionRefFor(collectionID string, index map[string]*viewmodel.ParsedCollection) events.CollectionRef {
	if node, ok := index[collectionID]; ok && node != nil {
		return events.RefFromParsed(node)
	}
	segments := strings.Split(strings.TrimSpace(collectionID), "/")
	last := segments[len(segments)-1]
	parent := ""
	if len(segments) > 1 {
		parent = strings.Join(segments[:len(segments)-1], "/")
	}
	ref := events.CollectionRef{
		ID:       collectionID,
		Name:     last,
		ParentID: parent,
		Type:     collection.GuessType(last, collection.TypeGeneric),
	}
	if collection.IsMonthName(last) {
		if month, err := time.Parse("January 2006", last); err == nil {
			ref.Month = month
		}
	}
	if collection.IsDayName(last) {
		if day, err := time.Parse("January 2, 2006", last); err == nil {
			ref.Day = day
			if ref.Month.IsZero() {
				ref.Month = time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
			}
			if parent == "" {
				ref.ParentID = fmt.Sprintf("%s %d", day.Month().String(), day.Year())
			}
		}
	}
	return ref
}

func entryLabelOrMessage(e *entry.Entry) string {
	if e == nil {
		return ""
	}
	label := uiutil.EntryLabel(e)
	if strings.TrimSpace(label) != "" {
		return label
	}
	return strings.TrimSpace(e.Message)
}

func describeLastTouched(now, ts time.Time) string {
	if ts.IsZero() {
		return "last touched unknown"
	}
	if now.IsZero() {
		now = time.Now()
	}
	diff := now.Sub(ts)
	switch {
	case diff < time.Minute:
		return "last touched just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "last touched 1 minute ago"
		}
		return fmt.Sprintf("last touched %d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "last touched 1 hour ago"
		}
		return fmt.Sprintf("last touched %d hours ago", hours)
	case diff < 48*time.Hour:
		return "last touched yesterday"
	case diff < 14*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("last touched %d days ago", days)
	default:
		return "last touched " + ts.Format("2006-01-02")
	}
}

func includeNextMonthCollection(list []*viewmodel.ParsedCollection, now time.Time) []*viewmodel.ParsedCollection {
	if len(list) == 0 {
		return list
	}
	if now.IsZero() {
		now = time.Now()
	}
	start := startOfMonth(now)
	if start.IsZero() {
		start = startOfMonth(time.Now())
	}
	next := start.AddDate(0, 1, 0)
	if next.Sub(now) > 14*24*time.Hour {
		return list
	}
	targetID := next.Format("January 2006")
	for _, node := range list {
		if node == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(node.ID), targetID) {
			return list
		}
	}
	stub := &viewmodel.ParsedCollection{
		ID:     targetID,
		Name:   targetID,
		Type:   collection.TypeDaily,
		Exists: false,
		Month:  next,
	}
	return append(list, stub)
}

func appendNewCollectionOption(list []*viewmodel.ParsedCollection) []*viewmodel.ParsedCollection {
	for _, node := range list {
		if node != nil && strings.EqualFold(strings.TrimSpace(node.ID), migrationNewCollectionID) {
			return list
		}
	}
	option := &viewmodel.ParsedCollection{
		ID:       migrationNewCollectionID,
		Name:     migrationNewCollectionLabel,
		Type:     collection.TypeGeneric,
		Exists:   false,
		Priority: math.MaxInt32,
		SortKey:  "zzzzzz-new-collection",
	}
	return append(list, option)
}
