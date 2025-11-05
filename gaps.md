Command Audit

  - :quit / :exit / :q: supported in both UIs (pkg/tui/app/app.go:120, pkg/tui/newapp/app.go:394).
  - :today: supported in both (old jumps via selectToday, new via jumpToToday; pkg/tui/app/app.go:1448, pkg/tui/
    newapp/app.go:404).
  - :future: supported in both (pkg/tui/app/app.go:1453, pkg/tui/newapp/app.go:411).
  - :report [window]: both call the report overlay (pkg/tui/app/app.go:1458, pkg/tui/newapp/app.go:386 + 1864).
  - :help: both surface help overlays (pkg/tui/app/app.go:1476, pkg/tui/newapp/app.go:398).
  - :lock / :unlock: parity for entry immutability (pkg/tui/app/app.go:2185, pkg/tui/newapp/app.go:408).
  - :debug: new UI only, toggles the event viewer (pkg/tui/newapp/app.go:404); the legacy UI has no equivalent
    command.
  - :migrate, :new-collection, :type, :mkdir, :show-hidden, :delete, :delete-collection, :sweep: implemented in
    the legacy command switch (pkg/tui/app/app.go:1468–1514) but currently fall through to “Unhandled command” in
    the new UI (pkg/tui/newapp/app.go:417). These commands also appear (except :sweep) in the legacy suggestion
    list (pkg/tui/app/app.go:118), while the new suggestion list is trimmed to seven entries (pkg/tui/newapp/
    app.go:210).

TODO command catalogue

  - :quit / :exit / :q — Tear down watches/services, exit the TUI, and propagate tea.Quit to callers.
  - :today — Focus the navigation pane on the current date, expand the month if needed, and scroll the detail pane
    to the real “Today” collection.
  - :future — Jump focus to the Future log collection/root and ensure its month child is visible.
  - :report [window] — Parse a duration window (default 1w), request Service.Report, and render the report overlay
    summarising completed entries by collection.
  - :help — Open/close the help overlay listing keymaps and mode hints; blur the prompt while active.
  - :lock / :unlock — Toggle the immutable flag on the currently highlighted entry, update the store, and refresh
    UI indicators.
  - :debug — Toggle the event viewer (new UI) so developers can inspect Bubble Tea message flow.
  - :migrate [window] — Build the migration dashboard with Service.MigrationCandidates and present the interactive
    task review UI (legacy only).
  - :new-collection — Launch the multi-step collection wizard (choose parent, name, type) to create a new
    collection (legacy only).
  - :type [collection] <type> — Infer the target collection (argument or selection) and persist a type change via
    Service.EnsureCollectionOfType (legacy only).
  - :mkdir <path> — Create a collection hierarchy, inferring parent folders, and refresh navigation/detail views
    (legacy only).
  - :show-hidden [on|off|status] — Toggle whether migrated originals stay visible in the detail pane and reload
    data accordingly (legacy only).
  - :sweep — Hide empty/migrated collections from the nav until rehydrated (legacy only).
  - :delete — Remove the currently selected entry after confirmation, respecting immutability (legacy only).
  - :delete-collection [name] — Prompt for confirmation, delete the named or selected collection, and refresh
    navigation/detail panes (legacy only).

Feature Audit

  - Collection management
      - Legacy UI ships a full collection wizard (pkg/tui/app/app.go:1621), in-place :type assignment, :mkdir path
        creation, and :show-hidden / :sweep housekeeping switches (pkg/tui/app/app.go:1500–1520, 4065).
      - New UI lacks those workflows: no wizard state or commands exist yet, so collection creation/type changes
        require dropping to the CLI or legacy surface.
  - Migration workflow
      - Legacy supports the migration dashboard via :migrate, launching startMigrationMode (pkg/tui/app/
        app.go:2164, 4492).
      - New UI has no migration overlay or command; migration-only key paths are effectively unavailable.
  - Entry operations
      - Legacy relies on inline modes (modeInsert, modePanel, modeParentSelect; pkg/tui/app/app.go:2622, 4081) for
        add/edit/detail flows.
      - New UI replaces these with dedicated overlays: add-task (openAddTaskOverlay, pkg/tui/newapp/app.go:2304),
        bullet detail (closeBulletDetailOverlay setup at pkg/tui/newapp/app.go:2350), and move overlay
        (openMoveOverlay, pkg/tui/newapp/app.go:1034). Functional coverage is similar, but invocation moved to
        overlay-driven interactions.
  - Reporting & help
      - Both UIs can open a report view (pkg/tui/app/app.go:2140, pkg/tui/newapp/app.go:1864) and a help overlay
        (pkg/tui/app/app.go:4081, pkg/tui/newapp/app.go:2132).
      - Only the new UI exposes the event viewer toggle (:debug), reflecting its focus on observability.
  - Command bar behaviour
      - Legacy bottom bar mixes context/help/commands (pkg/tui/app/app.go:2943).
      - The new command component currently forces suggestion overlays to full width as a temporary workaround
        while a collection-detail/command interaction is investigated (pkg/tui/components/command/model.go:361),
        recorded with a TODO.
  - General parity gaps
      - Collection administration (wizard/type/mkdir), housekeeping (:show-hidden, :sweep), destructive commands
        (:delete, :delete-collection), and the migration dashboard remain legacy-only. Users relying on those
        flows must still use the old UI or the CLI.
      - Conversely, the new UI’s event viewer, focus stack, and overlay system have no direct counterparts in the
        legacy surface.
