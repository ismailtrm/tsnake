package player

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ismail/tsnake/game"
	"github.com/ismail/tsnake/ui"
)

type menuFocus int

const (
	menuFocusName menuFocus = iota
	menuFocusColor
)

type Model struct {
	game        *game.Game
	playerID    string
	defaultName string
	snapCh      <-chan game.GameSnapshot
	lastSnap    game.GameSnapshot
	renderer    *Renderer
	onJoin      func()

	width    int
	height   int
	started  bool
	helpSeen bool
	joined   bool

	menuName   string
	menuFocus  menuFocus
	colorIndex int
}

func NewModel(
	gameState *game.Game,
	playerID string,
	defaultName string,
	snapCh <-chan game.GameSnapshot,
	lipRenderer *lipgloss.Renderer,
	onJoin func(),
) *Model {
	if onJoin == nil {
		onJoin = func() {}
	}

	return &Model{
		game:        gameState,
		playerID:    playerID,
		defaultName: defaultName,
		snapCh:      snapCh,
		renderer:    NewRenderer(lipRenderer),
		onJoin:      onJoin,
		colorIndex:  defaultColorIndex(playerID),
	}
}

func (m *Model) Init() tea.Cmd {
	return waitForSnapshot(m.snapCh)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

		if !m.joined {
			return m.handleMenuKey(msg)
		}
		return m.handleGameplayKey(msg)
	case game.GameSnapshot:
		m.lastSnap = msg
		m.started = true
		return m, waitForSnapshot(m.snapCh)
	default:
		return m, nil
	}
}

func (m *Model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.menuFocus = (m.menuFocus + 1) % 2
		return m, nil
	case tea.KeyEnter:
		name := strings.TrimSpace(m.menuName)
		if name == "" {
			name = m.defaultName
		}
		m.game.AddSnake(m.playerID, name, ui.PlayerColors[m.colorIndex%len(ui.PlayerColors)])
		m.joined = true
		m.helpSeen = false
		m.onJoin()
		return m, nil
	case tea.KeyBackspace, tea.KeyDelete:
		if m.menuFocus == menuFocusName && len(m.menuName) > 0 {
			runes := []rune(m.menuName)
			m.menuName = string(runes[:len(runes)-1])
		}
		return m, nil
	case tea.KeyLeft:
		m.menuFocus = menuFocusName
		return m, nil
	case tea.KeyRight:
		m.menuFocus = menuFocusColor
		return m, nil
	case tea.KeyUp:
		if m.menuFocus == menuFocusColor {
			m.colorIndex = (m.colorIndex - 1 + len(ui.PlayerColors)) % len(ui.PlayerColors)
		}
		return m, nil
	case tea.KeyDown:
		if m.menuFocus == menuFocusColor {
			m.colorIndex = (m.colorIndex + 1) % len(ui.PlayerColors)
		}
		return m, nil
	case tea.KeyRunes:
		if m.menuFocus == menuFocusColor {
			switch string(msg.Runes) {
			case "h":
				m.menuFocus = menuFocusName
				return m, nil
			case "l":
				m.menuFocus = menuFocusColor
				return m, nil
			case "j":
				m.colorIndex = (m.colorIndex + 1) % len(ui.PlayerColors)
				return m, nil
			case "k":
				m.colorIndex = (m.colorIndex - 1 + len(ui.PlayerColors)) % len(ui.PlayerColors)
				return m, nil
			}
		}

		if m.menuFocus == menuFocusName {
			m.menuName += string(msg.Runes)
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) handleGameplayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case " ":
		m.helpSeen = true
		m.game.SetBoost(m.playerID)
	case "up", "w":
		m.helpSeen = true
		m.game.SetDirection(m.playerID, game.Up)
	case "down", "s":
		m.helpSeen = true
		m.game.SetDirection(m.playerID, game.Down)
	case "left", "a":
		m.helpSeen = true
		m.game.SetDirection(m.playerID, game.Left)
	case "right", "d":
		m.helpSeen = true
		m.game.SetDirection(m.playerID, game.Right)
	}
	return m, nil
}

func (m *Model) View() string {
	if !m.joined {
		return m.renderer.RenderMenu(buildMenuViewModel(m.defaultName, m.menuName, m.colorIndex, m.menuFocus, m.width, m.height))
	}
	if !m.started {
		return "Starting tsnake..."
	}
	return m.renderer.Render(buildViewModel(m.lastSnap, m.playerID, m.width, m.height, !m.helpSeen))
}

func waitForSnapshot(ch <-chan game.GameSnapshot) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		return <-ch
	}
}

func defaultColorIndex(playerID string) int {
	if len(ui.PlayerColors) == 0 {
		return 0
	}
	sum := 0
	for _, r := range playerID {
		sum += int(r)
	}
	return sum % len(ui.PlayerColors)
}
