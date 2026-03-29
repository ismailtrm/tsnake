package player

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ismail/tsnake/game"
)

type Model struct {
	game     *game.Game
	playerID string
	snapCh   <-chan game.GameSnapshot
	lastSnap game.GameSnapshot
	renderer *Renderer
	width    int
	height   int
	started  bool
	helpSeen bool
}

func NewModel(gameState *game.Game, playerID string, snapCh <-chan game.GameSnapshot, lipRenderer *lipgloss.Renderer) *Model {
	return &Model{
		game:     gameState,
		playerID: playerID,
		snapCh:   snapCh,
		renderer: NewRenderer(lipRenderer),
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
	case game.GameSnapshot:
		m.lastSnap = msg
		m.started = true
		return m, waitForSnapshot(m.snapCh)
	default:
		return m, nil
	}
}

func (m *Model) View() string {
	if !m.started {
		return "Starting tsnake..."
	}
	return m.renderer.Render(buildViewModel(m.lastSnap, m.playerID, m.width, m.height, !m.helpSeen))
}

func waitForSnapshot(ch <-chan game.GameSnapshot) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}
