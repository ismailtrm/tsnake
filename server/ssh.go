package server

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	wishbubbletea "github.com/charmbracelet/wish/bubbletea"
	"github.com/muesli/termenv"

	"github.com/ismail/tsnake/game"
	"github.com/ismail/tsnake/player"
)

const (
	worldWidth  = 240
	worldHeight = 72
)

type Config struct {
	Addr        string
	HostKeyPath string
	TickRate    time.Duration
	Password    string
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Password) == "" {
		return errors.New("password must be set")
	}
	if strings.TrimSpace(c.Addr) == "" {
		return errors.New("addr must be set")
	}
	if strings.TrimSpace(c.HostKeyPath) == "" {
		return errors.New("host key path must be set")
	}
	if c.TickRate <= 0 {
		return errors.New("tick rate must be positive")
	}
	return nil
}

func RunSSH(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	g := game.NewGame(worldWidth, worldHeight)
	g.EnsureBot()
	hub := NewHub()

	engineCh := game.StartEngine(g, cfg.TickRate)
	go func() {
		for snap := range engineCh {
			hub.Broadcast(snap)
		}
	}()

	if err := os.MkdirAll(filepath.Dir(cfg.HostKeyPath), 0o755); err != nil {
		return err
	}

	srv, err := wish.NewServer(
		wish.WithAddress(cfg.Addr),
		wish.WithIdleTimeout(10*time.Minute),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool {
			return false
		}),
		wish.WithPasswordAuth(func(_ ssh.Context, pass string) bool {
			return pass == cfg.Password
		}),
		wish.WithMiddleware(
			wishbubbletea.MiddlewareWithColorProfile(playerHandler(g, hub), termenv.ANSI256),
		),
	)
	if err != nil {
		return err
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	go func() {
		<-signals
		_ = srv.Close()
	}()

	log.Printf("tsnake listening on %s", externalSSHAddr(cfg.Addr))
	err = srv.ListenAndServe()
	if errors.Is(err, ssh.ErrServerClosed) {
		return nil
	}
	return err
}

func playerHandler(g *game.Game, hub *Hub) wishbubbletea.Handler {
	return func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
		playerID := sessionPlayerID(sess)
		playerName := sessionPlayerName(sess)

		snapCh := hub.Register(playerID, g.Snapshot())

		cleanup := sync.OnceFunc(func() {
			cleanupPlayer(g, hub, playerID)
		})

		go func() {
			<-sess.Context().Done()
			cleanup()
		}()

		return player.NewModel(
			g,
			playerID,
			playerName,
			snapCh,
			wishbubbletea.MakeRenderer(sess),
			func() { hub.Broadcast(g.Snapshot()) },
		), []tea.ProgramOption{tea.WithAltScreen()}
	}
}

func cleanupPlayer(g *game.Game, hub *Hub, playerID string) {
	hub.Unregister(playerID)
	g.RemoveSnake(playerID)
	hub.Broadcast(g.Snapshot())
}

func sessionPlayerID(sess ssh.Session) string {
	sessionID := sess.Context().SessionID()
	if len(sessionID) > 12 {
		return sessionID[:12]
	}
	if sessionID != "" {
		return sessionID
	}
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}

func sessionPlayerName(sess ssh.Session) string {
	return sanitizePlayerName(sess.User(), "anon-"+sessionPlayerID(sess))
}

func sanitizePlayerName(raw, fallback string) string {
	name := strings.TrimSpace(raw)
	if name == "" || strings.EqualFold(name, "git") {
		return fallback
	}

	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		}
		if b.Len() >= 16 {
			break
		}
	}

	if b.Len() == 0 {
		return fallback
	}

	return b.String()
}

func externalSSHAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return fmt.Sprintf("<server-ip>:%s", port)
	}
	return addr
}
