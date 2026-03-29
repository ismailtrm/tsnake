package game

import (
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

type Point struct {
	X int
	Y int
}

func (p Point) Add(other Point) Point {
	return Point{X: p.X + other.X, Y: p.Y + other.Y}
}

type Direction int

const (
	Up Direction = iota
	Down
	Left
	Right
)

func (d Direction) Delta() Point {
	switch d {
	case Up:
		return Point{Y: -1}
	case Down:
		return Point{Y: 1}
	case Left:
		return Point{X: -1}
	case Right:
		return Point{X: 1}
	default:
		return Point{}
	}
}

func (d Direction) Opposite() Direction {
	switch d {
	case Up:
		return Down
	case Down:
		return Up
	case Left:
		return Right
	case Right:
		return Left
	default:
		return d
	}
}

type Snake struct {
	Body             []Point
	Dir              Direction
	NextDir          Direction
	Color            lipgloss.Color
	Name             string
	IsBot            bool
	Alive            bool
	Score            int
	Kills            int
	PingMS           int
	LastScore        int
	LastRank         int
	RespawnAt        time.Time
	ImmortalUntil    time.Time
	LastBoostInputAt time.Time
	MoveBudget       float64
	pendingGrowth    int
}

func NewSnake(start Point, length int, dir Direction, color lipgloss.Color, name string) *Snake {
	body := make([]Point, 0, length)
	for i := 0; i < length; i++ {
		body = append(body, Point{X: start.X - i, Y: start.Y})
	}

	return &Snake{
		Body:    body,
		Dir:     dir,
		NextDir: dir,
		Color:   color,
		Name:    name,
		Alive:   true,
		Score:   canonicalScore(length, 0),
	}
}

func (s *Snake) Head() Point {
	if len(s.Body) == 0 {
		return Point{}
	}
	return s.Body[0]
}

func (s *Snake) Move() {
	if !s.Alive || len(s.Body) == 0 {
		return
	}

	s.Dir = s.NextDir
	nextHead := s.Head().Add(s.Dir.Delta())

	body := make([]Point, 0, len(s.Body)+1)
	body = append(body, nextHead)
	body = append(body, s.Body...)
	if s.pendingGrowth > 0 {
		s.pendingGrowth--
	} else {
		body = body[:len(body)-1]
	}

	s.Body = body
	s.Score = canonicalScore(len(s.Body), s.pendingGrowth)
}

func (s *Snake) Grow() {
	s.pendingGrowth++
	s.Score = canonicalScore(len(s.Body), s.pendingGrowth)
}

func (s *Snake) GrowBy(n int) {
	s.pendingGrowth += max(0, n)
	s.Score = canonicalScore(len(s.Body), s.pendingGrowth)
}

func (s *Snake) TouchBoost(now time.Time) {
	s.LastBoostInputAt = now
}

func (s *Snake) IsBoosting(now time.Time) bool {
	return s.Alive && !s.LastBoostInputAt.IsZero() && now.Sub(s.LastBoostInputAt) <= boostGraceWindow
}

func (s *Snake) Speed(now time.Time) float64 {
	if !s.IsBoosting(now) {
		return 1.0
	}

	switch l := len(s.Body); {
	case l <= 8:
		return 2.5
	case l <= 18:
		return 2.0
	default:
		return 1.6
	}
}

func (s *Snake) IsImmortal(now time.Time) bool {
	return s.Alive && now.Before(s.ImmortalUntil)
}

func (s *Snake) Die(respawnAt time.Time, rank int) {
	s.Alive = false
	s.LastScore = s.Score
	s.LastRank = rank
	s.Score = 0
	s.RespawnAt = respawnAt
	s.ImmortalUntil = time.Time{}
	s.LastBoostInputAt = time.Time{}
	s.MoveBudget = 0
	s.Body = nil
	s.pendingGrowth = 0
}

func (s *Snake) Respawn(start Point, length int, dir Direction) {
	s.Body = make([]Point, 0, length)
	for i := 0; i < length; i++ {
		s.Body = append(s.Body, Point{X: start.X - i, Y: start.Y})
	}
	s.Dir = dir
	s.NextDir = dir
	s.Alive = true
	s.RespawnAt = time.Time{}
	s.ImmortalUntil = time.Time{}
	s.LastBoostInputAt = time.Time{}
	s.MoveBudget = 0
	s.pendingGrowth = 0
	s.Score = canonicalScore(len(s.Body), s.pendingGrowth)
}

func (s *Snake) SyncScore() {
	if !s.Alive {
		return
	}
	s.Score = canonicalScore(len(s.Body), s.pendingGrowth)
}

func Initial(name string) string {
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	return "◆"
}

func canonicalScore(bodyLen, pendingGrowth int) int {
	total := bodyLen + pendingGrowth - initialSnakeLen
	if total < 0 {
		return 0
	}
	return total * 10
}
