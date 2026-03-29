package game

import "time"

const (
	initialSnakeLen  = 4
	initialFoodCount = 8
	maxFood          = 20
	respawnDelay     = 3 * time.Second
	boostWindow      = 250 * time.Millisecond
)

func (g *Game) Tick() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Frame++
	now := time.Now()

	if len(g.Snakes) == 0 {
		for len(g.Food) < initialFoodCount {
			g.spawnFoodLocked()
		}
		return
	}

	for _, snake := range g.Snakes {
		if !snake.Alive && !snake.RespawnAt.IsZero() && !snake.RespawnAt.After(now) {
			start := g.randomSpawnPointLocked(initialSnakeLen)
			snake.Respawn(start, initialSnakeLen, Right)
		}
	}

	steps := 1
	for _, snake := range g.Snakes {
		if snake.Alive && snake.Speed(now) > steps {
			steps = snake.Speed(now)
		}
	}

	for step := 0; step < steps; step++ {
		moved := false
		for _, snake := range g.Snakes {
			if !snake.Alive || len(snake.Body) == 0 || snake.Speed(now) <= step {
				continue
			}
			snake.Move()
			snake.Body[0] = wrapPoint(snake.Body[0], g.W, g.H)
			moved = true
		}
		if !moved {
			continue
		}
		dead := g.detectDeathsLocked()
		g.consumeFoodLocked(dead)
		for id := range dead {
			snake := g.Snakes[id]
			if snake == nil || !snake.Alive {
				continue
			}
			g.spawnFoodFromBodyLocked(snake.Body)
			snake.Die(now.Add(respawnDelay), g.rankLocked(id))
		}
	}

	for len(g.Food) < initialFoodCount {
		if !g.spawnFoodLocked() {
			break
		}
	}
}

func (g *Game) detectDeathsLocked() map[string]struct{} {
	dead := make(map[string]struct{})
	headPositions := make(map[Point][]string)
	for id, snake := range g.Snakes {
		if !snake.Alive || len(snake.Body) == 0 {
			continue
		}
		headPositions[snake.Head()] = append(headPositions[snake.Head()], id)
	}

	for _, ids := range headPositions {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			dead[id] = struct{}{}
		}
	}

	for id, snake := range g.Snakes {
		if !snake.Alive || len(snake.Body) == 0 {
			continue
		}
		if _, alreadyDead := dead[id]; alreadyDead {
			continue
		}

		head := snake.Head()
		for _, segment := range snake.Body[1:] {
			if segment == head {
				dead[id] = struct{}{}
				break
			}
		}
		if _, alreadyDead := dead[id]; alreadyDead {
			continue
		}

		for otherID, other := range g.Snakes {
			if otherID == id || !other.Alive {
				continue
			}
			for _, segment := range other.Body {
				if segment == head {
					dead[id] = struct{}{}
					break
				}
			}
			if _, alreadyDead := dead[id]; alreadyDead {
				break
			}
		}
	}

	return dead
}

func (g *Game) consumeFoodLocked(dead map[string]struct{}) {
	eatenFood := make(map[int]struct{})
	for foodIdx, food := range g.Food {
		for id, snake := range g.Snakes {
			if _, isDead := dead[id]; isDead || !snake.Alive {
				continue
			}
			if snake.Head() != food {
				continue
			}
			snake.Grow()
			snake.Score += 10
			eatenFood[foodIdx] = struct{}{}
		}
	}

	if len(eatenFood) == 0 {
		return
	}

	nextFood := make([]Point, 0, len(g.Food)-len(eatenFood))
	for i, food := range g.Food {
		if _, eaten := eatenFood[i]; eaten {
			continue
		}
		nextFood = append(nextFood, food)
	}
	g.Food = nextFood
}

func (g *Game) spawnFoodFromBodyLocked(body []Point) {
	for _, segment := range body {
		if g.hasFoodLocked(segment) {
			continue
		}
		g.Food = append(g.Food, segment)
	}
}

func (g *Game) hasFoodLocked(p Point) bool {
	for _, food := range g.Food {
		if food == p {
			return true
		}
	}
	return false
}

func wrapPoint(p Point, w, h int) Point {
	if w > 0 {
		p.X = ((p.X % w) + w) % w
	}
	if h > 0 {
		p.Y = ((p.Y % h) + h) % h
	}
	return p
}

func (g *Game) rankLocked(id string) int {
	target := g.Snakes[id]
	if target == nil {
		return 0
	}

	rank := 1
	for otherID, other := range g.Snakes {
		if otherID == id {
			continue
		}
		if other.Score > target.Score || (other.Score == target.Score && len(other.Body) > len(target.Body)) {
			rank++
		}
	}
	return rank
}

func StartEngine(g *Game, interval time.Duration) <-chan GameSnapshot {
	ch := make(chan GameSnapshot, 1)

	pushSnapshot := func() {
		snap := g.Snapshot()
		select {
		case ch <- snap:
		default:
			<-ch
			ch <- snap
		}
	}

	pushSnapshot()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			g.Tick()
			pushSnapshot()
		}
	}()

	return ch
}

func foodChar(tick int) string {
	if tick%2 == 0 {
		return "✦"
	}
	return "✧"
}
