# Command Component & Overlay Plan

## Goals
- Provide a reusable `command` component that reserves the bottom line of the UI for a prompt/text input and remains sticky while the rest of the layout scrolls.
- Support flexible overlay content anchored to the command component so callers can render suggestion popovers, dialogs, or full overlays (e.g., add-task form).
- Align the implementation with Bubble Tea v2 conventions (`Update`/`View` returning `(tea.Model, tea.Cmd)` and `(string, *tea.Cursor)`).

## Proposed Components
- `pkg/tui/components/command`
  - `Model`: owns the command line state, overlay manager, sizing, focus, and event emission.
  - `PromptBar`: tiny sub-model that wraps a `textinput.Model` (or textarea for multi-line) with v2 cursor handling.
  - `OverlayHost`: handles overlay lifecycle, including size/position calculations and render stacking over the content area.
  - `ContentPortal`: lightweight helper to pass the “main content” view/cursor through the command component without the overlay disturbing layout.

## Key Responsibilities
- Command bar: maintain input value, focus, history, and dispatch `events.CommandSubmitMsg`, `events.CommandChangeMsg`, etc.
- Overlay host: accept arbitrary `Overlay` implementations (`Init`, `Update`, `View`, `SetSize`) so existing components (add-task, completion list) can be mounted.
- Layout orchestration: expose APIs to set the main content size (height minus command bar) and render `(content, command)` separately, ensuring overlays remain on top.

## Events & Messages
- New event types in `pkg/tui/events`:
  - `CommandChangeMsg`, `CommandSubmitMsg`, `CommandDeniedMsg`.
  - `OverlayOpenMsg`, `OverlayCloseMsg`, `OverlayResultMsg` (optional future).
- Decide whether overlays emit through the command component (`CommandOverlayMsg{Msg tea.Msg}`) or use existing event bus.

## Integration Plan
1. Prototype the command component in isolation with a simple prompt + overlay stub, ensuring cursor placement and sticky positioning work in the testbed.
2. Extend the testbed (`testbed/main.go`) with a `testbed command` command that embeds the command component under a placeholder content view and overlays sample helpers (e.g., completion list).
3. Adopt the new component in the journal view after validating behaviour, eventually replacing the existing add-task overlay wiring.

## Open Questions / Follow-ups
- Decide whether the command overlay should own focus when visible or allow split focus with content.
  answer: the command should take focus when it sees a `:` but we are not in a text input mode, so this might require us to make a new kind of input focus/blur? like vi has a nav mode and an input mode, you can only enter a command when you are in nav mode.
- Determine how overlay sizing is communicated (explicit width/height vs. letting the overlay compute its own size).
  answer: i think we will have to know, we tell the overlay what size it will be based on the overall size of the window and the goal of the overlay. Some things might end up being full screen. some might just be the bottom left quarter of the screen, some might be centered.
- Consider future needs: e.g., command history persistence, multi-line input, async command execution feedback.
  Answer: no history, but we will want smart input autocomplete and we can look at using bubble tea lists for that. yes for feedback: we can use the bar as a status area when not actively entering a command in it, so the command bar has a mode too 1) insert and 2) passive / active display if needed when feedback is needed or an error is happening. 
