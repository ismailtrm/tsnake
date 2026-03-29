package game

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/ismail/tsnake/ui"
)

type Game struct {
	mu        sync.RWMutex
	W         int
	H         int
	Snakes    map[string]*Snake
	Food      []Point
	Frame     int
	rng       *rand.Rand
	nextColor int
}

type SnakeSnap struct {
	Body      []Point
	Dir       Direction
	Color     lipgloss.Color
	Name      string
	Alive     bool
	Boosting  bool
	Score     int
	LastScore int
	LastRank  int
	RespawnIn time.Duration
}

type GameSnapshot struct {
	Snakes   map[string]SnakeSnap
	Food     []Point
	W        int
	H        int
	Tick     int
	FoodChar string
}

func NewGame(w, h int) *Game {
	g := &Game{
		W:      w,
		H:      h,
		Snakes: make(map[string]*Snake),
		Food:   make([]Point, 0, maxFood),
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	for len(g.Food) < initialFoodCount {
		g.spawnFoodLocked()
	}

	return g
}

func (g *Game) AddSnake(id, name string) *Snake {
	g.mu.Lock()
	defer g.mu.Unlock()

	if name == "" {
		name = fmt.Sprintf("anon-%04d", g.rng.Intn(10000))
	}

	color := ui.PlayerColors[g.nextColor%len(ui.PlayerColors)]
	g.nextColor++
	start := g.randomSpawnPointLocked(initialSnakeLen)
	snake := NewSnake(start, initialSnakeLen, Right, color, name)
	g.Snakes[id] = snake
	return snake
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

	snake.BoostUntil = time.Now().Add(boostWindow)
}

func (g *Game) Snapshot() GameSnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	now := time.Now()

	snakes := make(map[string]SnakeSnap, len(g.Snakes))
	for id, snake := range g.Snakes {
		body := clonePoints(snake.Body)
		snakes[id] = SnakeSnap{
			Body:      body,
			Dir:       snake.Dir,
			Color:     snake.Color,
			Name:      snake.Name,
			Alive:     snake.Alive,
			Boosting:  snake.Alive && now.Before(snake.BoostUntil),
			Score:     snake.Score,
			LastScore: snake.LastScore,
			LastRank:  snake.LastRank,
			RespawnIn: maxDuration(0, time.Until(snake.RespawnAt)),
		}
	}

	food := clonePoints(g.Food)

	return GameSnapshot{
		Snakes:   snakes,
		Food:     food,
		W:        g.W,
		H:        g.H,
		Tick:     g.Frame,
		FoodChar: foodChar(g.Frame),
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
	g.spawnFoodLocked()
}

func (g *Game) spawnFoodLocked() bool {
	if len(g.Food) >= maxFood || len(g.Food) >= g.W*g.H {
		return false
	}

	for attempt := 0; attempt < 256; attempt++ {
		p := Point{
			X: g.rng.Intn(g.W),
			Y: g.rng.Intn(g.H),
		}
		if g.isOccupiedLocked(p) {
			continue
		}
		g.Food = append(g.Food, p)
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

func (g *Game) isOccupiedLocked(p Point) bool {
	for _, food := range g.Food {
		if food == p {
			return true
		}
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
