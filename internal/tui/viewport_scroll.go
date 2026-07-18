package tui

type focusArea int

const (
	focusInput focusArea = iota
	focusCards
)

func (m *Model) scrollCardUp() {
	if m.cardIndex > 0 {
		m.cardIndex--
		m.scrollToCard()
	}
}

func (m *Model) scrollCardDown() {
	if m.cardIndex < len(m.lines)-1 {
		m.cardIndex++
		m.scrollToCard()
	}
}

func (m *Model) scrollToCard() {
	if len(m.lines) == 0 {
		return
	}
	if m.cardIndex < 0 {
		m.cardIndex = 0
	}
	if m.cardIndex >= len(m.lines) {
		m.cardIndex = len(m.lines) - 1
	}
	// приблизительное смещение строк на карточку
	m.viewport.SetYOffset(m.cardIndex * 4)
}
