package player

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/ismail/tsnake/ui"
)

type Renderer struct {
	lg        *lipgloss.Renderer
	styles    renderStyles
	cellCache map[cellStyleKey]string
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
	faint bool
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
		cellCache: make(map[cellStyleKey]string, 32),
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
	boardBody := strings.Join(r.renderLines(vm.Board.Grid, vm.Board.Dimmed), "\n")
	if vm.Board.Overlay != nil {
		boardBody = lipgloss.Place(
			vm.Board.Width,
			vm.Board.Height,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.JoinVertical(
				lipgloss.Center,
				boardBody,
				r.renderOverlay(*vm.Board.Overlay),
			),
		)
	}

	board := r.renderBoardFrame(boardBody, vm.Board.Width)
	sidebar := r.renderSidebar(vm)

	if vm.Layout.boardNextToSidebar {
		return joinVertical(
			header,
			joinHorizontalTop(board, r.boardFrameWidth(vm.Board.Width), sidebar),
		)
	}

	return joinVertical(
		header,
		board,
		sidebar,
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
		lines[y] = r.renderRow(row, false)
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

func (r *Renderer) renderLines(grid [][]Cell, faint bool) []string {
	lines := make([]string, len(grid))
	if len(grid) == 0 {
		r.prev = nil
		r.prevLines = nil
		return lines
	}

	if faint {
		for y, row := range grid {
			lines[y] = r.renderRow(row, true)
		}
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
		lines[y] = r.renderRow(row, false)
	}

	r.prev = cloneGrid(grid)
	r.prevLines = lines
	return lines
}

func (r *Renderer) renderRow(row []Cell, faint bool) string {
	if len(row) == 0 {
		return ""
	}

	var line strings.Builder
	var run strings.Builder

	current := cellStyleKey{color: string(row[0].Color), bold: row[0].Bold, faint: faint}
	prefix := r.cellPrefix(current)
	run.WriteString(row[0].Char)

	flush := func() {
		if run.Len() == 0 {
			return
		}
		text := run.String()
		run.Reset()
		if prefix == "" {
			line.WriteString(text)
			return
		}
		line.WriteString(prefix)
		line.WriteString(text)
		line.WriteString(ansiReset)
	}

	for _, cell := range row[1:] {
		next := cellStyleKey{color: string(cell.Color), bold: cell.Bold, faint: faint}
		if next != current {
			flush()
			current = next
			prefix = r.cellPrefix(current)
		}
		run.WriteString(cell.Char)
	}
	flush()
	return line.String()
}

func (r *Renderer) renderBoardFrame(body string, width int) string {
	borderPrefix := r.cellPrefix(cellStyleKey{color: string(ui.BorderColor)})
	top := wrapANSI(borderPrefix, "╭"+strings.Repeat("─", width+2)+"╮")
	bottom := wrapANSI(borderPrefix, "╰"+strings.Repeat("─", width+2)+"╯")

	lines := strings.Split(body, "\n")
	framed := make([]string, 0, len(lines)+2)
	framed = append(framed, top)
	for _, line := range lines {
		framed = append(framed, wrapANSI(borderPrefix, "│")+" "+line+" "+wrapANSI(borderPrefix, "│"))
	}
	framed = append(framed, bottom)
	return strings.Join(framed, "\n")
}

func (r *Renderer) boardFrameWidth(boardWidth int) int {
	return boardWidth + 4
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

func (r *Renderer) cellPrefix(key cellStyleKey) string {
	if prefix, ok := r.cellCache[key]; ok {
		return prefix
	}

	seqs := make([]string, 0, 3)
	if key.bold {
		seqs = append(seqs, termenv.BoldSeq)
	}
	if key.faint {
		seqs = append(seqs, termenv.FaintSeq)
	}
	if key.color != "" {
		if seq := r.lg.ColorProfile().Color(key.color).Sequence(false); seq != "" {
			seqs = append(seqs, seq)
		}
	}
	if len(seqs) == 0 {
		r.cellCache[key] = ""
		return ""
	}

	prefix := ansiCSI + strings.Join(seqs, ";") + "m"
	r.cellCache[key] = prefix
	return prefix
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

func joinVertical(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, "\n")
}

func joinHorizontalTop(left string, leftWidth int, right string) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	total := max(len(leftLines), len(rightLines))
	out := make([]string, total)

	blankLeft := strings.Repeat(" ", leftWidth)
	for i := 0; i < total; i++ {
		l := blankLeft
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			out[i] = l + " " + rightLines[i]
			continue
		}
		out[i] = l
	}

	return strings.Join(out, "\n")
}

func wrapANSI(prefix, text string) string {
	if prefix == "" {
		return text
	}
	return prefix + text + ansiReset
}

const (
	ansiCSI   = "\x1b["
	ansiReset = "\x1b[0m"
)
