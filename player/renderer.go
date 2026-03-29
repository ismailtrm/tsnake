package player

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ismail/tsnake/ui"
)

type Renderer struct {
	lg        *lipgloss.Renderer
	styles    renderStyles
	cellCache map[cellStyleKey]lipgloss.Style
	prev      [][]Cell
	prevLines []string

	leaderboardKey  string
	leaderboardView string
	statusKey       string
	statusView      string
	minimapKey      string
	minimapView     string
	minimapTick     int
}

type cellStyleKey struct {
	color string
	bold  bool
}

type renderStyles struct {
	app    lipgloss.Style
	frame  lipgloss.Style
	panel  lipgloss.Style
	title  lipgloss.Style
	muted  lipgloss.Style
	danger lipgloss.Style
	accent lipgloss.Style
}

func NewRenderer(lg *lipgloss.Renderer) *Renderer {
	if lg == nil {
		lg = lipgloss.DefaultRenderer()
	}
	r := &Renderer{
		lg:        lg,
		cellCache: make(map[cellStyleKey]lipgloss.Style, 32),
	}
	r.styles = renderStyles{
		app: lg.NewStyle().Foreground(ui.TextColor).Background(ui.BGColor),
		frame: lg.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.BorderColor).
			Background(ui.BGColor).
			Padding(0, 1),
		panel: lg.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.BorderColor).
			Background(ui.BGColor).
			Padding(0, 1),
		title:  lg.NewStyle().Foreground(ui.TextColor).Bold(true),
		muted:  lg.NewStyle().Foreground(ui.MutedColor),
		danger: lg.NewStyle().Foreground(ui.DangerColor).Bold(true),
		accent: lg.NewStyle().Foreground(ui.AccentColor).Bold(true),
	}
	return r
}

func (r *Renderer) Render(vm ViewModel) string {
	if vm.Board.Width == 0 || vm.Board.Height == 0 {
		return r.appStyle().Render("Starting tsnake...")
	}

	header := r.renderHeader(vm.Header)
	boardBody := strings.Join(r.renderLines(vm.Board.Grid), "\n")
	if vm.Board.Overlay != nil {
		boardContent := boardBody
		if vm.Board.Dimmed {
			boardContent = r.style().Faint(true).Render(boardContent)
		}
		boardBody = lipgloss.Place(
			vm.Board.Width,
			vm.Board.Height,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.JoinVertical(
				lipgloss.Center,
				boardContent,
				r.renderOverlay(*vm.Board.Overlay),
			),
		)
	}

	board := r.frameStyle().Render(boardBody)
	sidebar := r.renderSidebar(vm)

	if vm.Layout.boardNextToSidebar {
		return r.appStyle().Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				header,
				lipgloss.JoinHorizontal(lipgloss.Top, board, sidebar),
			),
		)
	}

	return r.appStyle().Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			board,
			sidebar,
		),
	)
}

func (r *Renderer) renderHeader(header HeaderViewModel) string {
	left := r.titleStyle().Render("TSNAKE")
	right := r.mutedStyle().Render(header.RightText)
	bar := r.style().
		Foreground(ui.AccentColor).
		Render(strings.Repeat("━", header.BarWidth))

	return lipgloss.JoinHorizontal(lipgloss.Center, left, " ", bar, " ", right)
}

func (r *Renderer) renderSidebar(vm ViewModel) string {
	leaderboard := r.renderLeaderboard(vm.Leaderboard)
	status := r.renderStatus(vm.Status)
	minimap := r.renderMinimap(vm.Minimap)

	switch vm.Layout.mode {
	case layoutWide:
		return lipgloss.JoinVertical(lipgloss.Left, leaderboard, status, minimap)
	case layoutBalanced:
		return lipgloss.JoinHorizontal(lipgloss.Top, leaderboard, status, minimap)
	default:
		if vm.Layout.compactTwoUp {
			return lipgloss.JoinVertical(
				lipgloss.Left,
				lipgloss.JoinHorizontal(lipgloss.Top, leaderboard, status),
				minimap,
			)
		}
		return lipgloss.JoinVertical(lipgloss.Left, status, leaderboard, minimap)
	}
}

func (r *Renderer) renderLeaderboard(leaderboard LeaderboardViewModel) string {
	if leaderboard.CacheKey == r.leaderboardKey {
		return r.leaderboardView
	}

	lines := []string{leaderboard.Title}
	nameWidth := max(4, leaderboard.Width-10)
	for i, entry := range leaderboard.Entries {
		lines = append(lines, renderLeaderboardLine(i+1, entry, nameWidth))
	}
	for i := 0; i < leaderboard.Placeholders; i++ {
		lines = append(lines, r.mutedStyle().Render("..."))
	}

	r.leaderboardKey = leaderboard.CacheKey
	r.leaderboardView = r.panelStyle().Width(leaderboard.Width).Render(strings.Join(lines, "\n"))
	return r.leaderboardView
}

func renderLeaderboardLine(rank int, entry scoreboardEntry, nameWidth int) string {
	return fmt.Sprintf("%d. %-*s %3d", rank, nameWidth, truncateText(entry.Name, nameWidth), entry.Score)
}

func (r *Renderer) renderStatus(status StatusViewModel) string {
	if status.CacheKey == r.statusKey {
		return r.statusView
	}

	lines := make([]string, len(status.Lines))
	copy(lines, status.Lines)
	for i := range lines {
		switch lines[i] {
		case "ALIVE", "BOOST":
			lines[i] = r.accentStyle().Render(lines[i])
		default:
			if strings.HasPrefix(lines[i], "RESPAWN") {
				lines[i] = r.dangerStyle().Render(lines[i])
			}
		}
	}

	r.statusKey = status.CacheKey
	r.statusView = r.panelStyle().Width(status.Width).Render(strings.Join(lines, "\n"))
	return r.statusView
}

func (r *Renderer) renderMinimap(minimap MinimapViewModel) string {
	if minimap.CacheKey == r.minimapKey || (minimap.Tick-r.minimapTick) < 2 && r.minimapView != "" {
		return r.minimapView
	}

	lines := make([]string, len(minimap.Grid))
	for y, row := range minimap.Grid {
		lines[y] = r.renderRow(row)
	}

	r.minimapKey = minimap.CacheKey
	r.minimapTick = minimap.Tick
	r.minimapView = r.panelStyle().Width(minimap.Width).Render(strings.Join([]string{
		"MINIMAP",
		strings.Join(lines, "\n"),
		r.mutedStyle().Render(minimap.Legend),
	}, "\n"))
	return r.minimapView
}

func (r *Renderer) renderOverlay(overlay OverlayViewModel) string {
	titleStyle := r.style().Foreground(overlay.TitleColor).Bold(true)
	lines := make([]string, 0, len(overlay.Lines)+1)
	lines = append(lines, titleStyle.Render(overlay.Title))
	lines = append(lines, overlay.Lines...)

	return r.style().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(overlay.BorderColor).
		Padding(1, 3).
		Background(ui.BGColor).
		Render(strings.Join(lines, "\n"))
}

func (r *Renderer) renderLines(grid [][]Cell) []string {
	lines := make([]string, len(grid))
	if len(grid) == 0 {
		r.prev = nil
		r.prevLines = nil
		return lines
	}

	prevCompatible := len(r.prev) == len(grid) && len(r.prevLines) == len(grid)
	if prevCompatible {
		for y := range grid {
			if len(r.prev[y]) != len(grid[y]) {
				prevCompatible = false
				break
			}
		}
	}

	for y, row := range grid {
		if prevCompatible && cellsEqual(r.prev[y], row) {
			lines[y] = r.prevLines[y]
			continue
		}
		lines[y] = r.renderRow(row)
	}

	r.prev = cloneGrid(grid)
	r.prevLines = lines
	return lines
}

func (r *Renderer) renderRow(row []Cell) string {
	var line strings.Builder
	for _, cell := range row {
		line.WriteString(r.cellStyle(cell.Color, cell.Bold).Render(cell.Char))
	}
	return line.String()
}

func cloneGrid(src [][]Cell) [][]Cell {
	dst := make([][]Cell, len(src))
	for y := range src {
		dst[y] = make([]Cell, len(src[y]))
		copy(dst[y], src[y])
	}
	return dst
}

func cellsEqual(a, b []Cell) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *Renderer) style() lipgloss.Style {
	return r.lg.NewStyle()
}

func (r *Renderer) cellStyle(color lipgloss.Color, bold bool) lipgloss.Style {
	key := cellStyleKey{color: string(color), bold: bold}
	if style, ok := r.cellCache[key]; ok {
		return style
	}

	style := r.lg.NewStyle().Foreground(color)
	if bold {
		style = style.Bold(true)
	}
	r.cellCache[key] = style
	return style
}

func (r *Renderer) appStyle() lipgloss.Style {
	return r.styles.app
}

func (r *Renderer) frameStyle() lipgloss.Style {
	return r.styles.frame
}

func (r *Renderer) panelStyle() lipgloss.Style {
	return r.styles.panel
}

func (r *Renderer) titleStyle() lipgloss.Style {
	return r.styles.title
}

func (r *Renderer) mutedStyle() lipgloss.Style {
	return r.styles.muted
}

func (r *Renderer) dangerStyle() lipgloss.Style {
	return r.styles.danger
}

func (r *Renderer) accentStyle() lipgloss.Style {
	return r.styles.accent
}
