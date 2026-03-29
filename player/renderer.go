package player

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/ismail/tsnake/game"
	"github.com/ismail/tsnake/ui"
)

type Cell struct {
	Char  string
	Color lipgloss.Color
	Bold  bool
}

type Renderer struct {
	lg        *lipgloss.Renderer
	styles    renderStyles
	cellCache map[cellStyleKey]lipgloss.Style
	prev      [][]Cell
	prevLines []string
	width     int
	height    int

	leaderboardKey  string
	leaderboardView string
	statusKey       string
	statusView      string
	minimapKey      string
	minimapView     string
	minimapTick     int
}

type scoreboardEntry struct {
	ID    string
	Name  string
	Score int
	Len   int
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

func (r *Renderer) Render(snap game.GameSnapshot, playerID string, w, h int) string {
	if snap.W == 0 || snap.H == 0 {
		return r.appStyle().Render("Starting tsnake...")
	}

	boardW, boardH := viewportSize(w, h, snap.W, snap.H)
	focus := game.Point{X: snap.W / 2, Y: snap.H / 2}
	playerSnake, hasPlayer := snap.Snakes[playerID]
	if hasPlayer && len(playerSnake.Body) > 0 {
		focus = playerSnake.Body[0]
	}

	left := clamp(focus.X-boardW/2, 0, max(0, snap.W-boardW))
	top := clamp(focus.Y-boardH/2, 0, max(0, snap.H-boardH))

	grid := make([][]Cell, boardH)
	for y := range grid {
		grid[y] = make([]Cell, boardW)
		for x := range grid[y] {
			grid[y][x] = backgroundCell(left+x, top+y, snap.Tick)
		}
	}

	for _, food := range snap.Food {
		x := food.X - left
		y := food.Y - top
		if x < 0 || x >= boardW || y < 0 || y >= boardH {
			continue
		}
		grid[y][x] = Cell{Char: snap.FoodChar, Color: ui.FoodColor, Bold: true}
	}

	for id, snake := range snap.Snakes {
		for i, part := range snake.Body {
			x := part.X - left
			y := part.Y - top
			if x < 0 || x >= boardW || y < 0 || y >= boardH {
				continue
			}
			cell := Cell{
				Char:  ui.CharSnakeBody,
				Color: bodyColor(snake.Color, i),
			}
			if i == 0 {
				cell = headCell(snake, playerID == id, snap.Tick)
			}
			grid[y][x] = cell
		}
	}

	lines := r.renderLines(grid)
	r.width = boardW
	r.height = boardH

	header := r.renderHeader(snap)
	boardBody := strings.Join(lines, "\n")
	if hasPlayer && !playerSnake.Alive {
		boardBody = lipgloss.Place(
			boardW,
			boardH,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.JoinVertical(
				lipgloss.Center,
				r.style().Faint(true).Render(boardBody),
				r.renderDeathScreen(playerSnake),
			),
		)
	}
	board := r.frameStyle().Render(boardBody)
	sidebar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		r.renderLeaderboard(snap),
		r.renderStatus(snap, playerID),
		r.renderMinimap(snap, playerID),
	)

	return r.appStyle().Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			board,
			sidebar,
		),
	)
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

func (r *Renderer) renderLeaderboard(snap game.GameSnapshot) string {
	key := leaderboardCacheKey(snap)
	if key == r.leaderboardKey {
		return r.leaderboardView
	}

	entries := make([]scoreboardEntry, 0, len(snap.Snakes))
	for id, snake := range snap.Snakes {
		if !snake.Alive {
			continue
		}
		entries = append(entries, scoreboardEntry{
			ID:    id,
			Name:  snake.Name,
			Score: snake.Score,
			Len:   len(snake.Body),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Score == entries[j].Score {
			if entries[i].Len == entries[j].Len {
				return entries[i].Name < entries[j].Name
			}
			return entries[i].Len > entries[j].Len
		}
		return entries[i].Score > entries[j].Score
	})

	if len(entries) > 5 {
		entries = entries[:5]
	}

	lines := []string{"TOP 5"}
	for i, entry := range entries {
		lines = append(lines, fmt.Sprintf("%d. %-10s %3d", i+1, entry.Name, entry.Score))
	}
	for len(lines) < 6 {
		lines = append(lines, r.mutedStyle().Render("..."))
	}

	r.leaderboardKey = key
	r.leaderboardView = r.panelStyle().Width(22).Render(strings.Join(lines, "\n"))
	return r.leaderboardView
}

func (r *Renderer) renderStatus(snap game.GameSnapshot, playerID string) string {
	key := statusCacheKey(snap, playerID)
	if key == r.statusKey {
		return r.statusView
	}

	snake, ok := snap.Snakes[playerID]
	if !ok {
		r.statusKey = key
		r.statusView = r.panelStyle().Width(30).Render("YOU\nDisconnected")
		return r.statusView
	}

	barTotal := 16
	filled := min(barTotal, snake.Score/10)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", max(0, barTotal-filled))
	rank := liveRank(snap, playerID)

	stateLine := r.accentStyle().Render("ALIVE")
	scoreLine := fmt.Sprintf("%s %d pts", bar, snake.Score)
	if !snake.Alive {
		stateLine = r.dangerStyle().Render(fmt.Sprintf("RESPAWN %.1fs", snake.RespawnIn.Seconds()))
		scoreLine = fmt.Sprintf("last:%d  rank:#%d", snake.LastScore, snake.LastRank)
	} else if snake.Boosting {
		stateLine = r.accentStyle().Render("BOOST")
	}

	lines := []string{
		"YOU",
		fmt.Sprintf("%s", snake.Name),
		fmt.Sprintf("len:%d  rank:#%d", len(snake.Body), rank),
		fmt.Sprintf("dir:%s", headChar(snake.Dir)),
		stateLine,
		scoreLine,
	}

	r.statusKey = key
	r.statusView = r.panelStyle().Width(32).Render(strings.Join(lines, "\n"))
	return r.statusView
}

func (r *Renderer) renderHeader(snap game.GameSnapshot) string {
	alive := 0
	for _, snake := range snap.Snakes {
		if snake.Alive {
			alive++
		}
	}

	left := r.titleStyle().Render("TSNAKE")
	right := r.mutedStyle().Render(fmt.Sprintf("%d online  %d alive  %dx%d world", len(snap.Snakes), alive, snap.W, snap.H))
	bar := r.style().
		Foreground(ui.AccentColor).
		Render(strings.Repeat("━", max(0, 12)))

	return lipgloss.JoinHorizontal(lipgloss.Center, left, " ", bar, " ", right)
}

func (r *Renderer) renderDeathScreen(snake game.SnakeSnap) string {
	respawn := fmt.Sprintf("Respawning in %.1fs", snake.RespawnIn.Seconds())
	card := r.style().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.DangerColor).
		Padding(1, 3).
		Background(ui.BGColor).
		Render(strings.Join([]string{
			r.dangerStyle().Render("YOU DIED"),
			fmt.Sprintf("Score: %d", snake.LastScore),
			fmt.Sprintf("Rank: #%d", snake.LastRank),
			r.mutedStyle().Render(respawn),
		}, "\n"))

	return card
}

func (r *Renderer) renderMinimap(snap game.GameSnapshot, playerID string) string {
	key := minimapCacheKey(snap, playerID)
	if key == r.minimapKey || (snap.Tick-r.minimapTick) < 2 && r.minimapView != "" {
		return r.minimapView
	}

	const miniW = 20
	const miniH = 10

	grid := make([][]Cell, miniH)
	for y := range grid {
		grid[y] = make([]Cell, miniW)
		for x := range grid[y] {
			grid[y][x] = Cell{Char: "·", Color: ui.MutedColor}
		}
	}

	for id, snake := range snap.Snakes {
		if len(snake.Body) == 0 {
			continue
		}
		head := snake.Body[0]
		x := min(miniW-1, head.X*miniW/max(1, snap.W))
		y := min(miniH-1, head.Y*miniH/max(1, snap.H))
		char := "•"
		color := snake.Color
		if id == playerID {
			char = "◆"
			color = ui.AccentColor
		}
		grid[y][x] = Cell{Char: char, Color: color, Bold: id == playerID}
	}

	lines := make([]string, miniH)
	for y, row := range grid {
		var line strings.Builder
		for _, cell := range row {
			style := r.style().Foreground(cell.Color)
			if cell.Bold {
				style = style.Bold(true)
			}
			line.WriteString(style.Render(cell.Char))
		}
		lines[y] = line.String()
	}

	r.minimapKey = key
	r.minimapTick = snap.Tick
	r.minimapView = r.panelStyle().Width(24).Render(strings.Join([]string{
		"MINIMAP",
		strings.Join(lines, "\n"),
		r.mutedStyle().Render("you ◆  others •"),
	}, "\n"))
	return r.minimapView
}

func (r *Renderer) renderRow(row []Cell) string {
	var line strings.Builder
	for _, cell := range row {
		line.WriteString(r.cellStyle(cell.Color, cell.Bold).Render(cell.Char))
	}
	return line.String()
}

func backgroundCell(worldX, worldY, _ int) Cell {
	switch {
	case worldX%8 == 0 && worldY%4 == 0:
		return Cell{Char: "·", Color: ui.GridGlow}
	case worldX%4 == 0 && worldY%2 == 0:
		return Cell{Char: "·", Color: ui.GridColor}
	case worldX%8 == 4 && worldY%4 == 2:
		return Cell{Char: "·", Color: ui.GridColor}
	default:
		return Cell{Char: ui.CharEmpty, Color: ui.TextColor}
	}
}

func headCell(snake game.SnakeSnap, isPlayer bool, tick int) Cell {
	color := snake.Color
	bold := true
	if snake.Boosting {
		color = lipgloss.Color("#FFF3B0")
	}
	if isPlayer && tick%2 == 0 {
		color = lipgloss.Color("#FFFFFF")
	}

	return Cell{
		Char:  headChar(snake.Dir),
		Color: color,
		Bold:  bold,
	}
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

func leaderboardCacheKey(snap game.GameSnapshot) string {
	entries := make([]scoreboardEntry, 0, len(snap.Snakes))
	for id, snake := range snap.Snakes {
		if !snake.Alive {
			continue
		}
		entries = append(entries, scoreboardEntry{
			ID:    id,
			Name:  snake.Name,
			Score: snake.Score,
			Len:   len(snake.Body),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Score == entries[j].Score {
			if entries[i].Len == entries[j].Len {
				return entries[i].Name < entries[j].Name
			}
			return entries[i].Len > entries[j].Len
		}
		return entries[i].Score > entries[j].Score
	})
	if len(entries) > 5 {
		entries = entries[:5]
	}
	var b strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&b, "%s:%d:%d|", entry.ID, entry.Score, entry.Len)
	}
	return b.String()
}

func statusCacheKey(snap game.GameSnapshot, playerID string) string {
	snake, ok := snap.Snakes[playerID]
	if !ok {
		return "missing"
	}
	return fmt.Sprintf("%s:%t:%t:%d:%d:%d:%d:%d:%d",
		playerID,
		snake.Alive,
		snake.Boosting,
		snake.Score,
		snake.LastScore,
		snake.LastRank,
		len(snake.Body),
		liveRank(snap, playerID),
		snake.Dir,
	)
}

func minimapCacheKey(snap game.GameSnapshot, playerID string) string {
	var b strings.Builder
	for id, snake := range snap.Snakes {
		if len(snake.Body) == 0 {
			continue
		}
		head := snake.Body[0]
		fmt.Fprintf(&b, "%s:%d:%d|", id, head.X, head.Y)
	}
	b.WriteString(playerID)
	return b.String()
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

func viewportSize(termW, termH, worldW, worldH int) (int, int) {
	if termW <= 0 {
		termW = 110
	}
	if termH <= 0 {
		termH = 42
	}

	boardW := clamp(termW-4, 24, 100)
	boardH := clamp(termH-10, 12, 32)

	return min(boardW, worldW), min(boardH, worldH)
}

func headChar(dir game.Direction) string {
	switch dir {
	case game.Up:
		return ui.CharHeadUp
	case game.Down:
		return ui.CharHeadDown
	case game.Left:
		return ui.CharHeadLeft
	case game.Right:
		return ui.CharHeadRight
	default:
		return ui.CharSnakeHead
	}
}

func bodyColor(base lipgloss.Color, segment int) lipgloss.Color {
	if segment == 0 {
		return base
	}

	blend := 0.55
	switch {
	case segment <= 2:
		blend = 0.18
	case segment <= 4:
		blend = 0.32
	}

	baseColor, err := colorful.Hex(string(base))
	if err != nil {
		return base
	}
	bgColor, err := colorful.Hex(string(ui.BGColor))
	if err != nil {
		return base
	}

	return lipgloss.Color(baseColor.BlendRgb(bgColor, blend).Hex())
}

func liveRank(snap game.GameSnapshot, playerID string) int {
	snake, ok := snap.Snakes[playerID]
	if !ok || !snake.Alive {
		return snake.LastRank
	}

	rank := 1
	for otherID, other := range snap.Snakes {
		if otherID == playerID || !other.Alive {
			continue
		}
		if other.Score > snake.Score || (other.Score == snake.Score && len(other.Body) > len(snake.Body)) {
			rank++
		}
	}
	return rank
}

func clamp(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
