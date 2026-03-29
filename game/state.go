package game

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/ismail/tsnake/ui"
)

const (
	BotID   = "bot-red"
	BotName = "BOT"
)

type FoodKind string

const (
	FoodNormal   FoodKind = "normal"
	FoodRemnant  FoodKind = "remnant"
	FoodImmortal FoodKind = "immortal_blue"
	FoodMegaRed  FoodKind = "mega_red"
	FoodRainbow  FoodKind = "rainbow_rare"
)

type FoodItem struct {
	Pos       Point
	Kind      FoodKind
	Color     lipgloss.Color
	Char      string
	ExpiresAt time.Time
}

type FoodSnap struct {
	Pos   Point
	Kind  FoodKind
	Color lipgloss.Color
	Char  string
}

type DeathMarker struct {
	Pos       Point
	ExpiresAt time.Time
}

type DeathMarkerSnap struct {
	Pos   Point
	Color lipgloss.Color
	Char  string
}

type Game struct {
	mu           sync.RWMutex
	W            int
	H            int
	Snakes       map[string]*Snake
	Food         []FoodItem
	DeathMarkers []DeathMarker
	Frame        int
	rng          *rand.Rand
	nextColor    int
}

type SnakeSnap struct {
	Body      []Point
	Dir       Direction
	Color     lipgloss.Color
	Name      string
	Initial   string
	IsBot     bool
	Alive     bool
	Boosting  bool
	Immortal  bool
	Score     int
	Kills     int
	PingMS    int
	LastScore int
	LastRank  int
	RespawnIn time.Duration
}

type GameSnapshot struct {
	Snakes map[string]SnakeSnap
	Food   []FoodSnap
	Deaths []DeathMarkerSnap
	W      int
	H      int
	Tick   int
}

func NewGame(w, h int) *Game {
	g := &Game{
		W:      w,
		H:      h,
		Snakes: make(map[string]*Snake),
		Food:   make([]FoodItem, 0, maxFood),
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	for g.normalFoodCountLocked() < initialFoodCount {
		g.spawnNormalFoodLocked()
	}

	return g
}

func (g *Game) AddSnake(id, name string, preferredColor ...lipgloss.Color) *Snake {
	g.mu.Lock()
	defer g.mu.Unlock()

	if existing, ok := g.Snakes[id]; ok {
		if name != "" {
			existing.Name = name
		}
		if len(preferredColor) > 0 && preferredColor[0] != "" {
			existing.Color = preferredColor[0]
		}
		return existing
	}

	if name == "" {
		name = fmt.Sprintf("anon-%04d", g.rng.Intn(10000))
	}

	color := lipgloss.Color("")
	if len(preferredColor) > 0 {
		color = preferredColor[0]
	}
	if color == "" {
		color = ui.PlayerColors[g.nextColor%len(ui.PlayerColors)]
		g.nextColor++
	}

	start := g.randomSpawnPointLocked(initialSnakeLen)
	snake := NewSnake(start, initialSnakeLen, Right, color, name)
	g.Snakes[id] = snake
	return snake
}

func (g *Game) EnsureBot() *Snake {
	g.mu.Lock()
	defer g.mu.Unlock()

	if bot, ok := g.Snakes[BotID]; ok {
		bot.IsBot = true
		bot.Name = BotName
		bot.Color = ui.BotColor
		bot.PingMS = 0
		return bot
	}

	start := g.randomSpawnPointLocked(initialSnakeLen)
	bot := NewSnake(start, initialSnakeLen, Right, ui.BotColor, BotName)
	bot.IsBot = true
	g.Snakes[BotID] = bot
	return bot
}

func (g *Game) RemoveSnake(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.Snakes, id)
}

func (g *Game) SetDirection(id string, dir Direction) {
	g.mu.Lock()
	defer g.mu.Unlock()

	snake, ok := g.Snakes[id]
	if !ok || !snake.Alive {
		return
	}
	if dir == snake.Dir.Opposite() {
		return
	}
	snake.NextDir = dir
}

func (g *Game) SetBoost(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	snake, ok := g.Snakes[id]
	if !ok || !snake.Alive {
		return
	}

	snake.TouchBoost(time.Now())
}

func (g *Game) SetPing(id string, pingMS int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	snake, ok := g.Snakes[id]
	if !ok || snake.IsBot {
		return
	}
	if pingMS < 0 {
		pingMS = 0
	}
	snake.PingMS = pingMS
}

func (g *Game) Snapshot() GameSnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	now := time.Now()

	snakes := make(map[string]SnakeSnap, len(g.Snakes))
	for id, snake := range g.Snakes {
		snakes[id] = SnakeSnap{
			Body:      clonePoints(snake.Body),
			Dir:       snake.Dir,
			Color:     snake.Color,
			Name:      snake.Name,
			Initial:   Initial(snake.Name),
			IsBot:     snake.IsBot,
			Alive:     snake.Alive,
			Boosting:  snake.IsBoosting(now),
			Immortal:  snake.IsImmortal(now),
			Score:     snake.Score,
			Kills:     snake.Kills,
			PingMS:    snake.PingMS,
			LastScore: snake.LastScore,
			LastRank:  snake.LastRank,
			RespawnIn: maxDuration(0, time.Until(snake.RespawnAt)),
		}
	}

	food := make([]FoodSnap, 0, len(g.Food))
	for _, item := range g.Food {
		color := item.Color
		if item.Kind == FoodRainbow {
			color = rainbowColor(g.Frame)
		}
		food = append(food, FoodSnap{
			Pos:   item.Pos,
			Kind:  item.Kind,
			Color: color,
			Char:  item.Char,
		})
	}

	deaths := make([]DeathMarkerSnap, 0, len(g.DeathMarkers))
	for _, marker := range g.DeathMarkers {
		deaths = append(deaths, DeathMarkerSnap{
			Pos:   marker.Pos,
			Color: ui.DangerColor,
			Char:  ui.CharDeathMarker,
		})
	}

	return GameSnapshot{
		Snakes: snakes,
		Food:   food,
		Deaths: deaths,
		W:      g.W,
		H:      g.H,
		Tick:   g.Frame,
	}
}

func clonePoints(src []Point) []Point {
	if len(src) == 0 {
		return nil
	}
	dst := make([]Point, len(src))
	copy(dst, src)
	return dst
}

func (g *Game) SpawnFood() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.spawnNormalFoodLocked()
}

func (g *Game) spawnNormalFoodLocked() bool {
	if g.normalFoodCountLocked() >= maxFood || len(g.Food) >= g.W*g.H {
		return false
	}
	return g.spawnFoodItemLocked(FoodItem{
		Kind:  FoodNormal,
		Color: ui.FoodColor,
		Char:  foodChar(g.Frame),
	})
}

func (g *Game) spawnSpecialFoodLocked(kind FoodKind) bool {
	item := FoodItem{Kind: kind}
	switch kind {
	case FoodImmortal:
		item.Color = ui.ImmortalFruitColor
		item.Char = ui.CharImmortalFruit
	case FoodMegaRed:
		item.Color = ui.MegaFruitColor
		item.Char = ui.CharMegaFruit
	case FoodRainbow:
		item.Color = ui.RainbowFruitColor
		item.Char = ui.CharRainbowFruit
	default:
		return false
	}
	return g.spawnFoodItemLocked(item)
}

func (g *Game) spawnRemnantsLocked(segments []Point, _ lipgloss.Color, expiresAt time.Time) {
	for _, segment := range segments {
		if g.foodIndexAtLocked(segment) >= 0 {
			continue
		}
		g.Food = append(g.Food, FoodItem{
			Pos:       segment,
			Kind:      FoodRemnant,
			Color:     ui.FoodColor,
			Char:      ui.CharRemnantFood,
			ExpiresAt: expiresAt,
		})
	}
}

func (g *Game) spawnFoodItemLocked(item FoodItem) bool {
	for attempt := 0; attempt < 256; attempt++ {
		p := Point{
			X: g.rng.Intn(g.W),
			Y: g.rng.Intn(g.H),
		}
		if g.isOccupiedLocked(p) {
			continue
		}
		item.Pos = p
		g.Food = append(g.Food, item)
		return true
	}

	return false
}

func (g *Game) randomSpawnPointLocked(length int) Point {
	for attempt := 0; attempt < 256; attempt++ {
		xMin := length + 1
		if xMin >= g.W {
			xMin = 0
		}
		x := xMin
		if g.W > xMin {
			x = xMin + g.rng.Intn(g.W-xMin)
		}
		p := Point{
			X: x,
			Y: g.rng.Intn(g.H),
		}
		free := true
		for i := 0; i < length; i++ {
			if g.isOccupiedLocked(Point{X: p.X - i, Y: p.Y}) {
				free = false
				break
			}
		}
		if free {
			return p
		}
	}

	return Point{X: min(length, g.W-1), Y: g.rng.Intn(max(1, g.H))}
}

func (g *Game) foodIndexAtLocked(p Point) int {
	for i, food := range g.Food {
		if food.Pos == p {
			return i
		}
	}
	return -1
}

func (g *Game) isOccupiedLocked(p Point) bool {
	if g.foodIndexAtLocked(p) >= 0 {
		return true
	}
	for _, snake := range g.Snakes {
		for _, segment := range snake.Body {
			if segment == p {
				return true
			}
		}
	}
	return false
}

func (g *Game) normalFoodCountLocked() int {
	count := 0
	for _, item := range g.Food {
		if item.Kind == FoodNormal {
			count++
		}
	}
	return count
}

func (g *Game) hasFoodKindLocked(kind FoodKind) bool {
	for _, item := range g.Food {
		if item.Kind == kind {
			return true
		}
	}
	return false
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

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func rainbowColor(tick int) lipgloss.Color {
	palette := []lipgloss.Color{
		"#FF5E7E",
		"#FF9F1C",
		"#FFD166",
		"#06D6A0",
		"#4CC9F0",
		"#7B61FF",
	}
	return palette[(tick/2)%len(palette)]
}
