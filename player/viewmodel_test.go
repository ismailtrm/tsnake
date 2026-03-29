package player

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ismail/tsnake/game"
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

func TestBuildViewModelHelpOverlay(t *testing.T) {
	snap := testSnapshot()

	vm := buildViewModel(snap, "snake-0", 120, 40, true)
	if vm.Board.Overlay == nil || vm.Board.Overlay.Title != "HOW TO PLAY" {
		t.Fatal("expected onboarding overlay for alive player")
	}

	vm = buildViewModel(snap, "snake-0", 120, 40, false)
	if vm.Board.Overlay != nil {
		t.Fatal("expected onboarding overlay to disappear once dismissed")
	}
}

func TestRendererRenderUsesViewModel(t *testing.T) {
	vm := buildViewModel(testSnapshot(), "snake-0", 120, 40, true)

	out := NewRenderer(nil).Render(vm)
	for _, want := range []string{"TSNAKE", "TOP", "MINIMAP", "HOW TO PLAY"} {
		if !strings.Contains(out, want) {
			t.Fatalf("render output missing %q", want)
		}
	}
}

func TestModelDismissesHelpOnGameplayKey(t *testing.T) {
	g := game.NewGame(40, 20)
	id := "snake-0"
	g.AddSnake(id, "snake-0")
	model := NewModel(g, id, nil, nil)

	updated, _ := model.Update(g.Snapshot())
	m := updated.(*Model)
	if !strings.Contains(m.View(), "HOW TO PLAY") {
		t.Fatal("expected help overlay before first gameplay input")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = updated.(*Model)
	if strings.Contains(m.View(), "HOW TO PLAY") {
		t.Fatal("expected help overlay to dismiss after gameplay input")
	}
	if got := g.Snakes[id].NextDir; got != game.Up {
		t.Fatalf("next direction = %v, want %v", got, game.Up)
	}
}

func testSnapshot() game.GameSnapshot {
	g := game.NewGame(200, 60)
	for i := 0; i < 6; i++ {
		id := "snake-" + string(rune('0'+i))
		s := g.AddSnake(id, id)
		s.Score = i * 10
		if i == 0 {
			s.Color = lipgloss.Color("#ffffff")
		}
	}
	return g.Snapshot()
}
