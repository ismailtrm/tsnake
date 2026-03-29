package game

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestTickRules(t *testing.T) {
	t.Run("wrap around", func(t *testing.T) {
		g := newRuleTestGame(12, 8)
		s := setTestSnake(g, "a", []Point{{11, 3}, {10, 3}, {9, 3}, {8, 3}}, Right)

		g.Tick()

		if got := s.Head(); got != (Point{X: 0, Y: 3}) {
			t.Fatalf("head = %+v, want %+v", got, Point{X: 0, Y: 3})
		}
		if !s.Alive {
			t.Fatal("snake should stay alive after wrap around")
		}
	})

	t.Run("boost window changes step count", func(t *testing.T) {
		tests := []struct {
			name     string
			boostFor time.Duration
			wantHead Point
		}{
			{name: "active", boostFor: boostWindow, wantHead: Point{X: 6, Y: 2}},
			{name: "expired", boostFor: -time.Millisecond, wantHead: Point{X: 5, Y: 2}},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				g := newRuleTestGame(20, 8)
				s := setTestSnake(g, "a", []Point{{4, 2}, {3, 2}, {2, 2}, {1, 2}}, Right)
				s.BoostUntil = time.Now().Add(tc.boostFor)

				g.Tick()

				if got := s.Head(); got != tc.wantHead {
					t.Fatalf("head = %+v, want %+v", got, tc.wantHead)
				}
			})
		}
	})

	t.Run("head to head kills both snakes", func(t *testing.T) {
		g := newRuleTestGame(20, 8)
		a := setTestSnake(g, "a", []Point{{5, 2}, {4, 2}, {3, 2}, {2, 2}}, Right)
		b := setTestSnake(g, "b", []Point{{7, 2}, {8, 2}, {9, 2}, {10, 2}}, Left)

		g.Tick()

		if a.Alive || b.Alive {
			t.Fatalf("expected both snakes to die, got alive=%v/%v", a.Alive, b.Alive)
		}
	})

	t.Run("body collision kills attacker only", func(t *testing.T) {
		g := newRuleTestGame(20, 8)
		a := setTestSnake(g, "a", []Point{{4, 2}, {3, 2}, {2, 2}, {1, 2}}, Right)
		b := setTestSnake(g, "b", []Point{{7, 2}, {6, 2}, {5, 2}, {4, 2}}, Right)

		g.Tick()

		if a.Alive {
			t.Fatal("attacker should die on body collision")
		}
		if !b.Alive {
			t.Fatal("defender should stay alive on body collision")
		}
	})

	t.Run("dead body becomes unique food trail", func(t *testing.T) {
		g := newRuleTestGame(30, 10)
		deadBody := []Point{
			{4, 4}, {3, 4}, {2, 4}, {1, 4}, {0, 4},
			{0, 5}, {1, 5}, {2, 5}, {3, 5}, {4, 5},
		}
		g.Food = []Point{{5, 4}}
		a := setTestSnake(g, "a", deadBody, Right)
		_ = setTestSnake(g, "b", []Point{{7, 4}, {6, 4}, {5, 4}, {4, 4}}, Right)

		g.Tick()

		if a.Alive {
			t.Fatal("snake should die and drop food trail")
		}
		if len(g.Food) != len(deadBody) {
			t.Fatalf("food count = %d, want %d", len(g.Food), len(deadBody))
		}

		seen := make(map[Point]int, len(g.Food))
		for _, food := range g.Food {
			seen[food]++
		}
		for _, segment := range []Point{{5, 4}, {4, 4}, {3, 4}, {2, 4}, {1, 4}, {0, 4}, {0, 5}, {1, 5}, {2, 5}, {3, 5}} {
			if seen[segment] != 1 {
				t.Fatalf("food at %+v appears %d times, want 1", segment, seen[segment])
			}
		}
	})

	t.Run("respawn after delay restores snake", func(t *testing.T) {
		g := newRuleTestGame(20, 8)
		s := setTestSnake(g, "a", []Point{{6, 3}, {5, 3}, {4, 3}, {3, 3}}, Right)
		s.Die(time.Now().Add(-time.Second), 2)

		g.Tick()

		if !s.Alive {
			t.Fatal("snake should respawn once respawn delay has elapsed")
		}
		if len(s.Body) != initialSnakeLen {
			t.Fatalf("respawned body len = %d, want %d", len(s.Body), initialSnakeLen)
		}
		if !s.RespawnAt.IsZero() {
			t.Fatal("respawn should clear respawn time")
		}
	})
}

func newRuleTestGame(w, h int) *Game {
	g := NewGame(w, h)
	g.Food = nil
	g.Frame = 0
	return g
}

func setTestSnake(g *Game, id string, body []Point, dir Direction) *Snake {
	copied := make([]Point, len(body))
	copy(copied, body)
	s := &Snake{
		Body:    copied,
		Dir:     dir,
		NextDir: dir,
		Color:   lipgloss.Color("#ffffff"),
		Name:    id,
		Alive:   true,
	}
	g.Snakes[id] = s
	return s
}
