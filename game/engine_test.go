package game

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/ismail/tsnake/ui"
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

	t.Run("boost move budget scales by size", func(t *testing.T) {
		tests := []struct {
			name     string
			body     []Point
			wantHead Point
		}{
			{name: "small snake", body: lineBody(Point{4, 2}, 6), wantHead: Point{9, 2}},
			{name: "medium snake", body: lineBody(Point{4, 3}, 12), wantHead: Point{8, 3}},
			{name: "large snake", body: lineBody(Point{4, 4}, 20), wantHead: Point{7, 4}},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				g := newRuleTestGame(40, 20)
				s := setTestSnake(g, "a", tc.body, Right)
				s.LastBoostInputAt = time.Now()

				g.Tick()
				g.Tick()

				if got := s.Head(); got != tc.wantHead {
					t.Fatalf("head = %+v, want %+v", got, tc.wantHead)
				}
			})
		}
	})

	t.Run("self bite severs tail into remnants", func(t *testing.T) {
		g := newRuleTestGame(20, 12)
		s := setTestSnake(g, "a", []Point{{4, 4}, {5, 4}, {5, 5}, {4, 5}, {3, 5}, {3, 4}}, Down)
		s.Score = 40

		g.Tick()

		if !s.Alive {
			t.Fatal("snake should stay alive after self bite")
		}
		if got, want := len(s.Body), 4; got != want {
			t.Fatalf("body len = %d, want %d", got, want)
		}
		if got := s.Score; got != 40 {
			t.Fatalf("score = %d, want 40", got)
		}

		remnants := 0
		for _, item := range g.Food {
			if item.Kind == FoodRemnant {
				remnants++
			}
		}
		if remnants != 2 {
			t.Fatalf("remnants = %d, want 2", remnants)
		}
	})

	t.Run("body owner gets kill credit", func(t *testing.T) {
		g := newRuleTestGame(20, 8)
		a := setTestSnake(g, "a", []Point{{5, 2}, {4, 2}, {3, 2}, {2, 2}}, Right)
		b := setTestSnake(g, "b", []Point{{4, 1}, {3, 1}, {2, 1}, {1, 1}}, Down)

		g.Tick()

		if !a.Alive {
			t.Fatal("defender should stay alive")
		}
		if b.Alive {
			t.Fatal("attacker should die on body collision")
		}
		if got := a.Kills; got != 1 {
			t.Fatalf("kills = %d, want 1", got)
		}
	})

	t.Run("head to head gives no kill credit", func(t *testing.T) {
		g := newRuleTestGame(20, 8)
		a := setTestSnake(g, "a", []Point{{5, 2}, {4, 2}, {3, 2}, {2, 2}}, Right)
		b := setTestSnake(g, "b", []Point{{7, 2}, {8, 2}, {9, 2}, {10, 2}}, Left)

		g.Tick()

		if a.Alive || b.Alive {
			t.Fatal("expected both snakes to die")
		}
		if a.Kills != 0 || b.Kills != 0 {
			t.Fatalf("kills = %d/%d, want 0/0", a.Kills, b.Kills)
		}
	})

	t.Run("blue fruit grants immortality and protects collisions", func(t *testing.T) {
		g := newRuleTestGame(20, 8)
		a := setTestSnake(g, "a", []Point{{4, 2}, {3, 2}, {2, 2}, {1, 2}}, Right)
		b := setTestSnake(g, "b", []Point{{5, 0}, {4, 0}, {3, 0}, {2, 0}}, Down)
		g.Food = []FoodItem{{
			Pos:   Point{5, 2},
			Kind:  FoodImmortal,
			Color: ui.ImmortalFruitColor,
			Char:  ui.CharImmortalFruit,
		}}

		g.Tick()
		if !a.IsImmortal(time.Now()) {
			t.Fatal("snake should become immortal after blue fruit")
		}

		g.Tick()
		if !a.Alive {
			t.Fatal("immortal snake should survive lethal collision")
		}
		if b.Alive {
			t.Fatal("attacker should die when hitting immortal body")
		}
		if got := a.Kills; got != 1 {
			t.Fatalf("kills = %d, want 1", got)
		}
	})

	t.Run("red fruit grants score and growth", func(t *testing.T) {
		g := newRuleTestGame(20, 8)
		s := setTestSnake(g, "a", []Point{{4, 2}, {3, 2}, {2, 2}, {1, 2}}, Right)
		g.Food = []FoodItem{{
			Pos:   Point{5, 2},
			Kind:  FoodMegaRed,
			Color: ui.MegaFruitColor,
			Char:  ui.CharMegaFruit,
		}}

		g.Tick()

		if got := s.Score; got != 100 {
			t.Fatalf("score = %d, want 100", got)
		}
		if got := s.pendingGrowth; got != 10 {
			t.Fatalf("pending growth = %d, want 10", got)
		}
	})

	t.Run("expired remnants are cleaned up", func(t *testing.T) {
		g := newRuleTestGame(20, 8)
		g.Food = []FoodItem{
			{Pos: Point{1, 1}, Kind: FoodRemnant, Color: ui.RemnantTintColor, Char: ui.CharRemnantFood, ExpiresAt: time.Now().Add(-time.Second)},
			{Pos: Point{2, 1}, Kind: FoodRemnant, Color: ui.RemnantTintColor, Char: ui.CharRemnantFood, ExpiresAt: time.Now().Add(time.Second)},
		}

		g.Tick()

		activeRemnants := 0
		for _, item := range g.Food {
			if item.Kind == FoodRemnant {
				activeRemnants++
			}
		}
		if activeRemnants != 1 {
			t.Fatalf("active remnants = %d, want 1", activeRemnants)
		}
		found := false
		for _, item := range g.Food {
			if item.Kind == FoodRemnant && item.Pos == (Point{2, 1}) {
				found = true
			}
		}
		if !found {
			t.Fatal("expected fresh remnant to remain after cleanup")
		}
	})

	t.Run("expired death markers are cleaned up", func(t *testing.T) {
		g := newRuleTestGame(20, 8)
		g.DeathMarkers = []DeathMarker{
			{Pos: Point{1, 1}, ExpiresAt: time.Now().Add(-time.Second)},
			{Pos: Point{2, 1}, ExpiresAt: time.Now().Add(time.Second)},
		}

		g.Tick()

		if len(g.DeathMarkers) != 1 {
			t.Fatalf("death markers = %d, want 1", len(g.DeathMarkers))
		}
		if got := g.DeathMarkers[0].Pos; got != (Point{2, 1}) {
			t.Fatalf("remaining marker = %+v, want %+v", got, Point{2, 1})
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
	})
}

func newRuleTestGame(w, h int) *Game {
	g := NewGame(w, h)
	g.Food = nil
	g.DeathMarkers = nil
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

func lineBody(head Point, length int) []Point {
	body := make([]Point, 0, length)
	for i := 0; i < length; i++ {
		body = append(body, Point{X: head.X - i, Y: head.Y})
	}
	return body
}
