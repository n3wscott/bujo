package newapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/timeutil"
	"tableflip.dev/bujo/pkg/tui/components/command"
)

func newReportOverlay(service *app.Service, window time.Duration, label string) *reportOverlay {
	if window <= 0 {
		if dur, _, err := timeutil.ParseWindow(timeutil.DefaultWindow); err == nil {
			window = dur
		} else {
			window = 7 * 24 * time.Hour
		}
	}
	if label == "" {
		label = timeutil.DefaultWindow
	}
	return &reportOverlay{
		service:     service,
		loading:     true,
		window:      window,
		windowLabel: label,
	}
}

type reportOverlay struct {
 service *app.Service
 width   int
 height  int

 loading bool
 result  app.ReportResult
 err     error
 since   time.Time
 until   time.Time
 window      time.Duration
 windowLabel string
}

func (o *reportOverlay) Init() tea.Cmd {
	o.loading = true
	return o.load()
}

func (o *reportOverlay) load() tea.Cmd {
 svc := o.service
 since := time.Now().Add(-o.window)
 until := time.Now()
 o.since = since
 o.until = until
	return func() tea.Msg {
		if svc == nil {
			return reportLoadedMsg{err: fmt.Errorf("service unavailable")}
		}
		res, err := svc.Report(context.Background(), since, until)
		return reportLoadedMsg{result: res, err: err}
	}
}

func (o *reportOverlay) Update(msg tea.Msg) (command.Overlay, tea.Cmd) {
	switch v := msg.(type) {
	case reportLoadedMsg:
		o.loading = false
		o.err = v.err
		if v.err == nil {
			o.result = v.result
			if !v.result.Since.IsZero() {
				o.since = v.result.Since
			}
			if !v.result.Until.IsZero() {
				o.until = v.result.Until
			}
		}
		return o, nil
	case tea.KeyMsg:
		switch v.String() {
		case "esc", "q":
			return nil, func() tea.Msg { return reportClosedMsg{} }
		}
	}
	return o, nil
}

func (o *reportOverlay) View() (string, *tea.Cursor) {
	width := o.width
	if width <= 0 {
		width = 60
	}
	builder := &strings.Builder{}
	title := "Completion Report"
	window := o.windowLabel
	builder.WriteString(title)
	if window != "" {
		builder.WriteString(" · last ")
		builder.WriteString(window)
	}
	builder.WriteString("\n")
	if !o.since.IsZero() && !o.until.IsZero() {
		builder.WriteString(fmt.Sprintf("Window: %s → %s\n", o.since.Format("2006-01-02"), o.until.Format("2006-01-02")))
	}

	if o.loading {
		builder.WriteString("Loading report…")
		return builder.String(), nil
	}
	if o.err != nil {
		builder.WriteString("Error: ")
		builder.WriteString(o.err.Error())
		return builder.String(), nil
	}

	builder.WriteString(fmt.Sprintf("Total completed: %d\n", o.result.Total))
	if len(o.result.Sections) == 0 {
		builder.WriteString("No completed entries in the selected window.")
		return builder.String(), nil
	}

	for _, section := range o.result.Sections {
		builder.WriteString(fmt.Sprintf("\n%s (%d)\n", section.Collection, len(section.Entries)))
		limit := 10
		for i, item := range section.Entries {
			if i >= limit {
				builder.WriteString("  …\n")
				break
			}
			label := ""
			if item.Entry != nil {
				label = item.Entry.Message
			}
			when := ""
			if !item.CompletedAt.IsZero() {
				when = item.CompletedAt.Format("Jan 2 15:04")
			}
			builder.WriteString(fmt.Sprintf("  - %s", label))
			if when != "" {
				builder.WriteString(fmt.Sprintf(" (%s)", when))
			}
			builder.WriteString("\n")
		}
	}
	return builder.String(), nil
}

func (o *reportOverlay) SetSize(width, height int) {
	if width <= 0 {
		width = 60
	}
	if height <= 0 {
		height = 10
	}
	o.width = width
	o.height = height
}
