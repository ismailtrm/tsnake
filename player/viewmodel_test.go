package player

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ismail/tsnake/game"
	"github.com/ismail/tsnake/ui"
)

func TestBuildViewModelLayoutModes(t *testing.T) {
	snap := testSnapshot()

	tests := []struct {
		name             string
		w                int
		h                int
		mode             layoutMode
		leaderboardCount int
		boardNextToSide  bool
	}{
		{name: "compact", w: 72, h: 22, mode: layoutCompact, leaderboardCount: 3, boardNextToSide: false},
		{name: "balanced", w: 108, h: 32, mode: layoutBalanced, leaderboardCount: 5, boardNextToSide: false},
		{name: "wide", w: 160, h: 40, mode: layoutWide, leaderboardCount: 5, boardNextToSide: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := buildViewModel(snap, "snake-0", tc.w, tc.h, false)
			if vm.Layout.mode != tc.mode {
				t.Fatalf("mode = %v, want %v", vm.Layout.mode, tc.mode)
			}
			if vm.Leaderboard.Placeholders+len(vm.Leaderboard.Entries) != tc.leaderboardCount {
				t.Fatalf("leaderboard rows = %d, want %d", vm.Leaderboard.Placeholders+len(vm.Leaderboard.Entries), tc.leaderboardCount)
			}
			if vm.Layout.boardNextToSidebar != tc.boardNextToSide {
				t.Fatalf("boardNextToSidebar = %v, want %v", vm.Layout.boardNextToSidebar, tc.boardNextToSide)
			}
		})
	}
}

func TestBuildViewModelHelpOverlayAndHeadInitial(t *testing.T) {
	snap := testSnapshot()
	vm := buildViewModel(snap, "snake-0", 120, 40, true)

	if vm.Board.Overlay == nil || vm.Board.Overlay.Title != "HOW TO PLAY" {
		t.Fatal("expected onboarding overlay for alive player")
	}

	foundInitial := false
	for _, row := range vm.Board.Grid {
		for _, cell := range row {
			if cell.Char == "A" {
				foundInitial = true
				break
			}
		}
	}
	if !foundInitial {
		t.Fatal("expected player head initial on board")
	}
}

func TestBuildViewModelLeaderboardAndSpecialFoods(t *testing.T) {
	snap := testSnapshot()
	vm := buildViewModel(snap, "snake-0", 120, 40, false)

	if len(vm.Leaderboard.Entries) == 0 || !vm.Leaderboard.Entries[0].IsLeader {
		t.Fatal("expected leader to be marked in leaderboard")
	}
	if vm.Leaderboard.Entries[0].Kills == 0 {
		t.Fatal("expected leaderboard kills column data")
	}

	foundSpecial := false
	for _, row := range vm.Board.Grid {
		for _, cell := range row {
			if cell.Char == ui.CharImmortalFruit || cell.Char == ui.CharMegaFruit || cell.Char == ui.CharRemnantFood {
				foundSpecial = true
			}
		}
	}
	if !foundSpecial {
		t.Fatal("expected special food cells on board")
	}
}

func TestRendererRenderUsesGameplayAndMenuViewModels(t *testing.T) {
	menu := buildMenuViewModel("local", "alice", 1, menuFocusName, 120, 40)
	menuOut := NewRenderer(nil).RenderMenu(menu)
	for _, want := range []string{"TSNAKE", "Color 1", "alice"} {
		if !strings.Contains(menuOut, want) {
			t.Fatalf("menu output missing %q", want)
		}
	}

	gameOut := NewRenderer(nil).Render(buildViewModel(testSnapshot(), "snake-0", 120, 40, true))
	for _, want := range []string{"LEADERBOARD", "MINIMAP", "HOW TO PLAY"} {
		if !strings.Contains(gameOut, want) {
			t.Fatalf("game output missing %q", want)
		}
	}
}

func TestModelMenuSpawnAndHelpDismiss(t *testing.T) {
	g := game.NewGame(40, 20)
	model := NewModel(g, "snake-0", "local", nil, nil, nil)

	if !strings.Contains(model.View(), "TSNAKE") {
		t.Fatal("expected menu before joining")
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updated, _ = updated.(*Model).Update(tea.KeyMsg{Type: tea.KeyRight})
	updated, _ = updated.(*Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ = updated.(*Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := updated.(*Model)

	snake := g.Snakes["snake-0"]
	if snake == nil {
		t.Fatal("expected snake to be created on enter")
	}
	if snake.Name != "a" {
		t.Fatalf("snake name = %q, want %q", snake.Name, "a")
	}
	if snake.Color != ui.PlayerColors[(defaultColorIndex("snake-0")+1)%len(ui.PlayerColors)] {
		t.Fatalf("unexpected selected color: %s", snake.Color)
	}

	updated, _ = m.Update(g.Snapshot())
	m = updated.(*Model)
	if !strings.Contains(m.View(), "HOW TO PLAY") {
		t.Fatal("expected help overlay after joining")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = updated.(*Model)
	if strings.Contains(m.View(), "HOW TO PLAY") {
		t.Fatal("expected help overlay to dismiss after gameplay input")
	}
}

func testSnapshot() game.GameSnapshot {
	g := game.NewGame(60, 30)
	g.Food = []game.FoodItem{
		{Pos: game.Point{14, 12}, Kind: game.FoodImmortal, Color: ui.ImmortalFruitColor, Char: ui.CharImmortalFruit},
		{Pos: game.Point{15, 12}, Kind: game.FoodMegaRed, Color: ui.MegaFruitColor, Char: ui.CharMegaFruit},
		{Pos: game.Point{16, 12}, Kind: game.FoodRemnant, Color: lipgloss.Color("#999999"), Char: ui.CharRemnantFood},
	}
	g.DeathMarkers = []game.DeathMarker{{Pos: game.Point{18, 12}, ExpiresAt: time.Now().Add(time.Second)}}
	names := []string{"alice", "bravo", "charlie", "delta", "echo", "foxtrot"}
	for i, name := range names {
		id := "snake-" + string(rune('0'+i))
		s := g.AddSnake(id, name)
		y := 12 + i
		s.Body = []game.Point{
			{X: 12 + i*4, Y: y},
			{X: 11 + i*4, Y: y},
			{X: 10 + i*4, Y: y},
			{X: 9 + i*4, Y: y},
		}
		s.Dir = game.Right
		s.NextDir = game.Right
		s.Score = i * 10
		s.Kills = i
		if i == len(names)-1 {
			s.Score = 999
		}
	}
	return g.Snapshot()
}
