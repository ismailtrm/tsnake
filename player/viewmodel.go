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

type layoutMode int

const (
	layoutCompact layoutMode = iota
	layoutBalanced
	layoutWide
)

type layoutConfig struct {
	mode               layoutMode
	termW              int
	termH              int
	boardW             int
	boardH             int
	boardNextToSidebar bool
	compactTwoUp       bool
	leaderboardWidth   int
	leaderboardCount   int
	statusWidth        int
	minimapPanelWidth  int
	minimapW           int
	minimapH           int
	compactHeader      bool
}

type scoreboardEntry struct {
	ID    string
	Name  string
	Score int
	Len   int
}

type ViewModel struct {
	Layout      layoutConfig
	Header      HeaderViewModel
	Board       BoardViewModel
	Leaderboard LeaderboardViewModel
	Status      StatusViewModel
	Minimap     MinimapViewModel
}

type HeaderViewModel struct {
	RightText string
	BarWidth  int
}

type BoardViewModel struct {
	Width   int
	Height  int
	Grid    [][]Cell
	Dimmed  bool
	Overlay *OverlayViewModel
}

type OverlayViewModel struct {
	Title       string
	Lines       []string
	BorderColor lipgloss.Color
	TitleColor  lipgloss.Color
}

type LeaderboardViewModel struct {
	Width        int
	Title        string
	Entries      []scoreboardEntry
	Placeholders int
	CacheKey     string
}

type StatusViewModel struct {
	Width    int
	Lines    []string
	CacheKey string
}

type MinimapViewModel struct {
	Width    int
	Grid     [][]Cell
	Legend   string
	Tick     int
	CacheKey string
}

func buildViewModel(snap game.GameSnapshot, playerID string, termW, termH int, showHelp bool) ViewModel {
	layout := resolveLayout(termW, termH, snap.W, snap.H)
	vm := ViewModel{
		Layout: layout,
		Header: buildHeaderViewModel(snap, layout),
	}

	if snap.W == 0 || snap.H == 0 {
		return vm
	}

	playerSnake, hasPlayer := snap.Snakes[playerID]
	focus := game.Point{X: snap.W / 2, Y: snap.H / 2}
	if hasPlayer && len(playerSnake.Body) > 0 {
		focus = playerSnake.Body[0]
	}

	vm.Board = buildBoardViewModel(snap, playerID, focus, layout, hasPlayer, playerSnake, showHelp)
	vm.Leaderboard = buildLeaderboardViewModel(snap, layout)
	vm.Status = buildStatusViewModel(snap, playerID, layout)
	vm.Minimap = buildMinimapViewModel(snap, playerID, layout)

	return vm
}

func buildHeaderViewModel(snap game.GameSnapshot, layout layoutConfig) HeaderViewModel {
	alive := 0
	for _, snake := range snap.Snakes {
		if snake.Alive {
			alive++
		}
	}

	rightText := fmt.Sprintf("%d online  %d alive  %dx%d world", len(snap.Snakes), alive, snap.W, snap.H)
	if layout.compactHeader {
		rightText = fmt.Sprintf("%d on  %d up  %dx%d", len(snap.Snakes), alive, snap.W, snap.H)
	}

	return HeaderViewModel{
		RightText: rightText,
		BarWidth:  clamp(layout.termW/8, 4, 14),
	}
}

func buildBoardViewModel(
	snap game.GameSnapshot,
	playerID string,
	focus game.Point,
	layout layoutConfig,
	hasPlayer bool,
	playerSnake game.SnakeSnap,
	showHelp bool,
) BoardViewModel {
	left := clamp(focus.X-layout.boardW/2, 0, max(0, snap.W-layout.boardW))
	top := clamp(focus.Y-layout.boardH/2, 0, max(0, snap.H-layout.boardH))

	grid := make([][]Cell, layout.boardH)
	for y := range grid {
		grid[y] = make([]Cell, layout.boardW)
		for x := range grid[y] {
			grid[y][x] = backgroundCell(left+x, top+y)
		}
	}

	for _, food := range snap.Food {
		x := food.X - left
		y := food.Y - top
		if x < 0 || x >= layout.boardW || y < 0 || y >= layout.boardH {
			continue
		}
		grid[y][x] = Cell{Char: snap.FoodChar, Color: ui.FoodColor, Bold: true}
	}

	for id, snake := range snap.Snakes {
		for i, part := range snake.Body {
			x := part.X - left
			y := part.Y - top
			if x < 0 || x >= layout.boardW || y < 0 || y >= layout.boardH {
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

	board := BoardViewModel{
		Width:  layout.boardW,
		Height: layout.boardH,
		Grid:   grid,
	}

	switch {
	case hasPlayer && !playerSnake.Alive:
		board.Dimmed = true
		board.Overlay = &OverlayViewModel{
			Title:       "YOU DIED",
			TitleColor:  ui.DangerColor,
			BorderColor: ui.DangerColor,
			Lines: []string{
				fmt.Sprintf("Score: %d", playerSnake.LastScore),
				fmt.Sprintf("Rank: #%d", playerSnake.LastRank),
				fmt.Sprintf("Respawning in %.1fs", playerSnake.RespawnIn.Seconds()),
			},
		}
	case hasPlayer && playerSnake.Alive && showHelp:
		board.Dimmed = true
		board.Overlay = &OverlayViewModel{
			Title:       "HOW TO PLAY",
			TitleColor:  ui.AccentColor,
			BorderColor: ui.AccentColor,
			Lines: []string{
				"Move: arrows or WASD",
				"Sprint: tap or hold space",
				"Edges wrap around the world",
				"Dead snakes drop food trails",
				"Press any move key to start",
			},
		}
	}

	return board
}

func buildLeaderboardViewModel(snap game.GameSnapshot, layout layoutConfig) LeaderboardViewModel {
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

	if len(entries) > layout.leaderboardCount {
		entries = entries[:layout.leaderboardCount]
	}

	var key strings.Builder
	fmt.Fprintf(&key, "%d:%d|", layout.leaderboardWidth, layout.leaderboardCount)
	for _, entry := range entries {
		fmt.Fprintf(&key, "%s:%s:%d:%d|", entry.ID, entry.Name, entry.Score, entry.Len)
	}

	return LeaderboardViewModel{
		Width:        layout.leaderboardWidth,
		Title:        fmt.Sprintf("TOP %d", layout.leaderboardCount),
		Entries:      entries,
		Placeholders: layout.leaderboardCount - len(entries),
		CacheKey:     key.String(),
	}
}

func buildStatusViewModel(snap game.GameSnapshot, playerID string, layout layoutConfig) StatusViewModel {
	snake, ok := snap.Snakes[playerID]
	if !ok {
		return StatusViewModel{
			Width:    layout.statusWidth,
			Lines:    []string{"YOU", "Disconnected"},
			CacheKey: fmt.Sprintf("%d:missing", layout.statusWidth),
		}
	}

	barTotal := clamp(layout.statusWidth-14, 8, 16)
	filled := min(barTotal, snake.Score/10)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", max(0, barTotal-filled))
	rank := liveRank(snap, playerID)

	stateLine := "ALIVE"
	scoreLine := fmt.Sprintf("%s %d pts", bar, snake.Score)
	if !snake.Alive {
		stateLine = fmt.Sprintf("RESPAWN %.1fs", snake.RespawnIn.Seconds())
		scoreLine = fmt.Sprintf("last:%d  rank:#%d", snake.LastScore, snake.LastRank)
	} else if snake.Boosting {
		stateLine = "BOOST"
	}

	var lines []string
	if layout.mode == layoutCompact {
		lines = []string{
			"YOU",
			truncateText(snake.Name, max(8, layout.statusWidth-2)),
			fmt.Sprintf("len:%d rank:#%d %s", len(snake.Body), rank, headChar(snake.Dir)),
			stateLine,
			compactScoreLine(snake),
		}
	} else {
		lines = []string{
			"YOU",
			snake.Name,
			fmt.Sprintf("len:%d  rank:#%d", len(snake.Body), rank),
			fmt.Sprintf("dir:%s", headChar(snake.Dir)),
			stateLine,
			scoreLine,
		}
	}

	return StatusViewModel{
		Width:    layout.statusWidth,
		Lines:    lines,
		CacheKey: fmt.Sprintf("%d:%s", layout.statusWidth, strings.Join(lines, "|")),
	}
}

func buildMinimapViewModel(snap game.GameSnapshot, playerID string, layout layoutConfig) MinimapViewModel {
	grid := make([][]Cell, layout.minimapH)
	for y := range grid {
		grid[y] = make([]Cell, layout.minimapW)
		for x := range grid[y] {
			grid[y][x] = Cell{Char: "·", Color: ui.MutedColor}
		}
	}

	ids := make([]string, 0, len(snap.Snakes))
	for id, snake := range snap.Snakes {
		if len(snake.Body) == 0 {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var key strings.Builder
	fmt.Fprintf(&key, "%d:%d:%d|", layout.minimapPanelWidth, layout.minimapW, layout.minimapH)
	for _, id := range ids {
		snake := snap.Snakes[id]
		head := snake.Body[0]
		x := min(layout.minimapW-1, head.X*layout.minimapW/max(1, snap.W))
		y := min(layout.minimapH-1, head.Y*layout.minimapH/max(1, snap.H))
		char := "•"
		color := snake.Color
		if id == playerID {
			char = "◆"
			color = ui.AccentColor
		}
		grid[y][x] = Cell{Char: char, Color: color, Bold: id == playerID}
		fmt.Fprintf(&key, "%s:%d:%d|", id, head.X, head.Y)
	}

	legend := "you ◆  others •"
	if layout.mode == layoutCompact {
		legend = "◆ you  • all"
	}

	return MinimapViewModel{
		Width:    layout.minimapPanelWidth,
		Grid:     grid,
		Legend:   legend,
		Tick:     snap.Tick,
		CacheKey: key.String(),
	}
}

func backgroundCell(worldX, worldY int) Cell {
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

func resolveLayout(termW, termH, worldW, worldH int) layoutConfig {
	if termW <= 0 {
		termW = 110
	}
	if termH <= 0 {
		termH = 42
	}

	layout := layoutConfig{
		termW:         termW,
		termH:         termH,
		compactHeader: termW < 90,
	}

	switch {
	case termW >= 132 && termH >= 28:
		sidebarWidth := clamp(termW/4, 24, 32)
		layout.mode = layoutWide
		layout.boardNextToSidebar = true
		layout.leaderboardWidth = sidebarWidth
		layout.statusWidth = sidebarWidth
		layout.minimapPanelWidth = sidebarWidth
		layout.minimapW = clamp(sidebarWidth-4, 14, 22)
		layout.minimapH = clamp(termH/4, 7, 11)
		layout.leaderboardCount = 5
		layout.boardW = min(clamp(termW-sidebarWidth-6, 32, 100), worldW)
		layout.boardH = min(clamp(termH-4, 14, 36), worldH)
	case termW >= 96 && termH >= 24:
		panelWidth := clamp((termW-8)/3, 20, 28)
		bottomHeight := max(8, 2+layoutMinimapHeight(termH, layoutBalanced))
		layout.mode = layoutBalanced
		layout.leaderboardWidth = panelWidth
		layout.statusWidth = panelWidth
		layout.minimapPanelWidth = panelWidth
		layout.minimapW = clamp(panelWidth-4, 14, 22)
		layout.minimapH = layoutMinimapHeight(termH, layoutBalanced)
		layout.leaderboardCount = 5
		layout.boardW = min(clamp(termW-4, 28, 100), worldW)
		layout.boardH = min(clamp(termH-bottomHeight-4, 12, 30), worldH)
	default:
		stackedWidth := clamp(termW-4, 20, 34)
		twoUp := termW >= 58
		layout.mode = layoutCompact
		layout.compactTwoUp = twoUp
		layout.leaderboardWidth = stackedWidth
		layout.statusWidth = stackedWidth
		layout.minimapPanelWidth = stackedWidth
		if twoUp {
			layout.leaderboardWidth = clamp(termW/2-3, 18, 26)
			layout.statusWidth = clamp(termW-layout.leaderboardWidth-4, 18, 28)
			layout.minimapPanelWidth = stackedWidth
		}
		layout.minimapW = clamp(layout.minimapPanelWidth-4, 10, 16)
		layout.minimapH = layoutMinimapHeight(termH, layoutCompact)
		layout.leaderboardCount = 3
		bottomHeight := compactBottomHeight(layout)
		layout.boardW = min(clamp(termW-4, 24, 88), worldW)
		layout.boardH = min(clamp(termH-bottomHeight-4, 10, 24), worldH)
	}

	return layout
}

func layoutMinimapHeight(termH int, mode layoutMode) int {
	switch mode {
	case layoutWide:
		return clamp(termH/4, 7, 11)
	case layoutBalanced:
		return clamp(termH/5, 6, 9)
	default:
		return clamp(termH/6, 5, 7)
	}
}

func compactBottomHeight(layout layoutConfig) int {
	leaderboardHeight := layout.leaderboardCount + 3
	statusHeight := 7
	minimapHeight := layout.minimapH + 4
	if layout.compactTwoUp {
		return max(leaderboardHeight, statusHeight) + minimapHeight + 1
	}
	return leaderboardHeight + statusHeight + minimapHeight + 2
}

func compactScoreLine(snake game.SnakeSnap) string {
	if !snake.Alive {
		return fmt.Sprintf("last:%d #%d", snake.LastScore, snake.LastRank)
	}
	if snake.Boosting {
		return fmt.Sprintf("score:%d boost", snake.Score)
	}
	return fmt.Sprintf("score:%d", snake.Score)
}

func truncateText(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-1]) + "…"
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
