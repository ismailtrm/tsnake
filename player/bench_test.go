package player

import (
	"fmt"
	"testing"

	"github.com/ismail/tsnake/game"
)

func benchmarkSnapshot() (game.GameSnapshot, string) {
	g := game.NewGame(200, 60)
	playerID := "snake-0"
	for i := 0; i < 24; i++ {
		id := fmt.Sprintf("snake-%d", i)
		s := g.AddSnake(id, id)
		s.Score = i * 10
		if id == playerID {
			g.SetBoost(id)
		}
	}
	return g.Snapshot(), playerID
}

func BenchmarkRendererRender(b *testing.B) {
	snap, playerID := benchmarkSnapshot()
	r := NewRenderer(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Render(snap, playerID, 120, 42)
	}
}
