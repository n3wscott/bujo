package app

// ensureOverlayStack lazily initializes the overlay stack container.
func (m *Model) ensureOverlayStack() *overlayStack {
	if m.overlayStack == nil {
		m.overlayStack = newOverlayStack(m.width, maxInt(1, m.height-1))
	}
	return m.overlayStack
}
