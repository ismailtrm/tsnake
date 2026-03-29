package game

import "time"

const (
	initialSnakeLen      = 4
	initialFoodCount     = 8
	maxFood              = 20
	respawnDelay         = 3 * time.Second
	boostGraceWindow     = 300 * time.Millisecond
	immortalDuration     = 5 * time.Second
	remnantTTL           = 20 * time.Second
	deathMarkerTTL       = 1500 * time.Millisecond
	immortalSpawnChance  = 0.003
	megaFruitSpawnChance = 0.002
	maxMovePhases        = 4
)

type bodyOccupant struct {
	id    string
	index int
}

type occupancyIndex struct {
	bodyCells map[Point]bodyOccupant
	headCells map[Point][]string
	foodCells map[Point]int
}

func (g *Game) Tick() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Frame++
	now := time.Now()

	g.cleanupExpiredLocked(now)
	g.respawnSnakesLocked(now)
	g.replenishAmbientFoodLocked()

	if len(g.Snakes) == 0 {
		return
	}

	for _, snake := range g.Snakes {
		if !snake.Alive || len(snake.Body) == 0 {
			continue
		}
		snake.MoveBudget += snake.Speed(now)
	}

	for phase := 0; phase < maxMovePhases; phase++ {
		moved := false
		for _, snake := range g.Snakes {
			if !snake.Alive || len(snake.Body) == 0 || snake.MoveBudget < 1 {
				continue
			}
			snake.MoveBudget--
			snake.Move()
			snake.Body[0] = wrapPoint(snake.Body[0], g.W, g.H)
			moved = true
		}
		if !moved {
			break
		}
		g.resolvePhaseLocked(now)
	}

	g.replenishAmbientFoodLocked()
}

func (g *Game) respawnSnakesLocked(now time.Time) {
	for _, snake := range g.Snakes {
		if !snake.Alive && !snake.RespawnAt.IsZero() && !snake.RespawnAt.After(now) {
			start := g.randomSpawnPointLocked(initialSnakeLen)
			snake.Respawn(start, initialSnakeLen, Right)
		}
	}
}

func (g *Game) replenishAmbientFoodLocked() {
	for g.normalFoodCountLocked() < initialFoodCount {
		if !g.spawnNormalFoodLocked() {
			break
		}
	}
	if !g.hasFoodKindLocked(FoodImmortal) && g.rng.Float64() < immortalSpawnChance {
		g.spawnSpecialFoodLocked(FoodImmortal)
	}
	if !g.hasFoodKindLocked(FoodMegaRed) && g.rng.Float64() < megaFruitSpawnChance {
		g.spawnSpecialFoodLocked(FoodMegaRed)
	}
}

func (g *Game) cleanupExpiredLocked(now time.Time) {
	if len(g.Food) > 0 {
		food := g.Food[:0]
		for _, item := range g.Food {
			if !item.ExpiresAt.IsZero() && !item.ExpiresAt.After(now) {
				continue
			}
			food = append(food, item)
		}
		g.Food = food
	}

	if len(g.DeathMarkers) > 0 {
		markers := g.DeathMarkers[:0]
		for _, marker := range g.DeathMarkers {
			if !marker.ExpiresAt.After(now) {
				continue
			}
			markers = append(markers, marker)
		}
		g.DeathMarkers = markers
	}
}

func (g *Game) resolvePhaseLocked(now time.Time) {
	index := g.buildOccupancyIndexLocked()

	dead := make(map[string]struct{})
	killers := make(map[string]string)
	selfBites := make(map[string]int)
	foodClaims := make(map[int]string)

	for _, ids := range index.headCells {
		if len(ids) < 2 {
			continue
		}

		immortalCount := 0
		for _, id := range ids {
			if snake := g.Snakes[id]; snake != nil && snake.IsImmortal(now) {
				immortalCount++
			}
		}

		switch {
		case immortalCount == 0:
			for _, id := range ids {
				dead[id] = struct{}{}
			}
		case immortalCount == len(ids):
			// All participants survive when they are immortal.
		default:
			for _, id := range ids {
				snake := g.Snakes[id]
				if snake == nil || snake.IsImmortal(now) {
					continue
				}
				dead[id] = struct{}{}
			}
		}
	}

	for id, snake := range g.Snakes {
		if snake == nil || !snake.Alive || len(snake.Body) == 0 {
			continue
		}
		if _, alreadyDead := dead[id]; alreadyDead {
			continue
		}

		head := snake.Head()
		if occupant, hit := index.bodyCells[head]; hit {
			if occupant.id == id {
				selfBites[id] = occupant.index
				continue
			}
			if snake.IsImmortal(now) {
				continue
			}
			dead[id] = struct{}{}
			killers[id] = occupant.id
			continue
		}

		if foodIdx, ok := index.foodCells[head]; ok {
			if current, claimed := foodClaims[foodIdx]; !claimed || id < current {
				foodClaims[foodIdx] = id
			}
		}
	}

	for victimID, killerID := range killers {
		if killerID == "" || killerID == victimID {
			continue
		}
		if _, headToHead := dead[killerID]; headToHead {
			continue
		}
		if killer := g.Snakes[killerID]; killer != nil {
			killer.Kills++
		}
		_ = victimID
	}

	g.applySelfBitesLocked(now, selfBites)
	g.consumeClaimedFoodLocked(now, foodClaims)
	g.applyDeathsLocked(now, dead)
}

func (g *Game) applySelfBitesLocked(now time.Time, selfBites map[string]int) {
	for id, biteIndex := range selfBites {
		snake := g.Snakes[id]
		if snake == nil || !snake.Alive || biteIndex <= 0 || biteIndex >= len(snake.Body) {
			continue
		}

		severed := clonePoints(snake.Body[biteIndex:])
		snake.Body = clonePoints(snake.Body[:biteIndex])
		if len(snake.Body) == 0 {
			snake.Body = []Point{snake.Head()}
		}
		g.spawnRemnantsLocked(severed, snake.Color, now.Add(remnantTTL))
	}
}

func (g *Game) consumeClaimedFoodLocked(now time.Time, claims map[int]string) {
	if len(claims) == 0 {
		return
	}

	consumed := make(map[int]struct{}, len(claims))
	for foodIdx, id := range claims {
		if foodIdx < 0 || foodIdx >= len(g.Food) {
			continue
		}
		snake := g.Snakes[id]
		if snake == nil || !snake.Alive {
			continue
		}

		item := g.Food[foodIdx]
		switch item.Kind {
		case FoodNormal, FoodRemnant:
			snake.Grow()
			snake.Score += 10
		case FoodImmortal:
			snake.ImmortalUntil = now.Add(immortalDuration)
		case FoodMegaRed:
			snake.Score += 100
			snake.GrowBy(10)
		}
		consumed[foodIdx] = struct{}{}
	}

	if len(consumed) == 0 {
		return
	}

	nextFood := make([]FoodItem, 0, len(g.Food)-len(consumed))
	for i, item := range g.Food {
		if _, eaten := consumed[i]; eaten {
			continue
		}
		nextFood = append(nextFood, item)
	}
	g.Food = nextFood
}

func (g *Game) applyDeathsLocked(now time.Time, dead map[string]struct{}) {
	for id := range dead {
		snake := g.Snakes[id]
		if snake == nil || !snake.Alive || len(snake.Body) == 0 {
			continue
		}

		head := snake.Head()
		g.DeathMarkers = append(g.DeathMarkers, DeathMarker{
			Pos:       head,
			ExpiresAt: now.Add(deathMarkerTTL),
		})
		g.spawnRemnantsLocked(clonePoints(snake.Body), snake.Color, now.Add(remnantTTL))
		snake.Die(now.Add(respawnDelay), g.rankLocked(id))
	}
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
		bodyCells: make(map[Point]bodyOccupant, bodyCap),
		headCells: make(map[Point][]string, len(g.Snakes)),
		foodCells: make(map[Point]int, len(g.Food)),
	}

	for i, item := range g.Food {
		index.foodCells[item.Pos] = i
	}

	for id, snake := range g.Snakes {
		if !snake.Alive || len(snake.Body) == 0 {
			continue
		}
		index.headCells[snake.Head()] = append(index.headCells[snake.Head()], id)
		for i, segment := range snake.Body[1:] {
			index.bodyCells[segment] = bodyOccupant{id: id, index: i + 1}
		}
	}

	return index
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
