package game

import (
	"fmt"
	"testing"
)

func benchmarkGameWithSnakes(b *testing.B, snakes, food int) *Game {
	g := NewGame(200, 60)
	g.Food = g.Food[:0]
	for i := 0; i < snakes; i++ {
		s := g.AddSnake(fmt.Sprintf("snake-%d", i), fmt.Sprintf("snake-%d", i))
		s.Score = i * 10
	}
	for len(g.Food) < food {
		g.spawnNormalFoodLocked()
	}
	return g
}

func BenchmarkGameTick(b *testing.B) {
	g := benchmarkGameWithSnakes(b, 24, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Tick()
	}
}

func BenchmarkSnapshot(b *testing.B) {
	g := benchmarkGameWithSnakes(b, 24, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Snapshot()
	}
}
