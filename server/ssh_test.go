package server

import (
	"testing"
	"time"

	"github.com/ismail/tsnake/game"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{
		Addr:        ":2222",
		HostKeyPath: "data/host_key",
		TickRate:    100 * time.Millisecond,
		Password:    "secret",
	}

	tests := []struct {
		name   string
		cfg    Config
		hasErr bool
	}{
		{name: "valid", cfg: valid, hasErr: false},
		{name: "missing password", cfg: Config{Addr: valid.Addr, HostKeyPath: valid.HostKeyPath, TickRate: valid.TickRate}, hasErr: true},
		{name: "missing addr", cfg: Config{Password: valid.Password, HostKeyPath: valid.HostKeyPath, TickRate: valid.TickRate}, hasErr: true},
		{name: "missing host key", cfg: Config{Password: valid.Password, Addr: valid.Addr, TickRate: valid.TickRate}, hasErr: true},
		{name: "missing tick", cfg: Config{Password: valid.Password, Addr: valid.Addr, HostKeyPath: valid.HostKeyPath}, hasErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.hasErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tc.hasErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestSanitizePlayerName(t *testing.T) {
	tests := []struct {
		raw      string
		fallback string
		want     string
	}{
		{raw: "alice", fallback: "anon-1", want: "alice"},
		{raw: " git ", fallback: "anon-1", want: "anon-1"},
		{raw: "ali ce!*", fallback: "anon-1", want: "alice"},
		{raw: "very-very-long-player-name", fallback: "anon-1", want: "very-very-long-p"},
		{raw: "!!!", fallback: "anon-1", want: "anon-1"},
	}

	for _, tc := range tests {
		if got := sanitizePlayerName(tc.raw, tc.fallback); got != tc.want {
			t.Fatalf("sanitizePlayerName(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestCleanupPlayerRemovesState(t *testing.T) {
	g := game.NewGame(40, 20)
	hub := NewHub()
	id := "player-1"
	g.AddSnake(id, "player-1")
	hub.Register(id, g.Snapshot())

	cleanupPlayer(g, hub, id)

	if _, ok := g.Snapshot().Snakes[id]; ok {
		t.Fatal("expected snake to be removed from game")
	}
	if _, ok := hub.channels[id]; ok {
		t.Fatal("expected player channel to be removed from hub")
	}
}
