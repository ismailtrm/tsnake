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
		index := g.buildOccupancyIndexLocked()
		dead := g.detectDeathsLocked(index)
		g.consumeFoodLocked(index, dead)
		for id := range dead {
			snake := g.Snakes[id]
			if snake == nil || !snake.Alive {
				continue
			}
			g.spawnFoodFromBodyLocked(index, snake.Body)
			snake.Die(now.Add(respawnDelay), g.rankLocked(id))
		}
	}

	for len(g.Food) < initialFoodCount {
		if !g.spawnFoodLocked() {
			break
		}
	}
}

type occupancyIndex struct {
	bodyCells map[Point]string
	headCells map[Point][]string
	foodCells map[Point]int
}

func (g *Game) buildOccupancyIndexLocked() occupancyIndex {
	bodyCap := 0
	for _, snake := range g.Snakes {
		if !snake.Alive {
			continue
		}
		bodyCap += len(snake.Body)
	}

	index := occupancyIndex{
		bodyCells: make(map[Point]string, bodyCap),
		headCells: make(map[Point][]string, len(g.Snakes)),
		foodCells: make(map[Point]int, len(g.Food)),
	}

	for i, food := range g.Food {
		index.foodCells[food] = i
	}

	for id, snake := range g.Snakes {
		if !snake.Alive || len(snake.Body) == 0 {
			continue
		}
		index.headCells[snake.Head()] = append(index.headCells[snake.Head()], id)
		for _, segment := range snake.Body[1:] {
			index.bodyCells[segment] = id
		}
	}

	return index
}

func (g *Game) detectDeathsLocked(index occupancyIndex) map[string]struct{} {
	dead := make(map[string]struct{})

	for _, ids := range index.headCells {
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
		if _, hit := index.bodyCells[head]; hit {
			dead[id] = struct{}{}
			continue
		}
	}

	return dead
}

func (g *Game) consumeFoodLocked(index occupancyIndex, dead map[string]struct{}) {
	eatenFood := make(map[int]struct{})
	for id, snake := range g.Snakes {
		if _, isDead := dead[id]; isDead || !snake.Alive || len(snake.Body) == 0 {
			continue
		}
		if foodIdx, ok := index.foodCells[snake.Head()]; ok {
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

func (g *Game) spawnFoodFromBodyLocked(index occupancyIndex, body []Point) {
	for _, segment := range body {
		if _, exists := index.foodCells[segment]; exists {
			continue
		}
		index.foodCells[segment] = len(g.Food)
		g.Food = append(g.Food, segment)
	}
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
