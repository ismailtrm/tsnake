package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
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
	localPlayerID   = "local-player"
	localPlayerName = "local"
	defaultAddr     = ":2222"
	defaultHostKey  = "data/host_key"
	passwordEnvName = "TSNAKE_PASSWORD"
)

func main() {
	var (
		mode        = flag.String("mode", "ssh", "run mode: local or ssh")
		addr        = flag.String("addr", defaultAddr, "ssh listen address")
		hostKeyPath = flag.String("host-key-path", defaultHostKey, "ssh host key path")
		tickRate    = flag.Duration("tick", 100*time.Millisecond, "engine tick interval")
		pprofAddr   = flag.String("pprof-addr", "", "optional loopback-only debug address for net/http/pprof")
	)
	flag.Parse()

	if err := startPprofServer(strings.TrimSpace(*pprofAddr)); err != nil {
		log.Fatal(err)
	}

	var err error
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "local":
		err = runLocal(*tickRate)
	case "ssh":
		err = runSSH(*addr, *hostKeyPath, *tickRate)
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}

	if err != nil {
		log.Fatal(err)
	}
}

func runLocal(tickRate time.Duration) error {
	g := game.NewGame(200, 60)
	g.AddSnake(localPlayerID, localPlayerName)

	snapCh := game.StartEngine(g, tickRate)
	model := player.NewModel(g, localPlayerID, snapCh, nil)
	program := tea.NewProgram(model, tea.WithAltScreen())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	go func() {
		<-signals
		g.RemoveSnake(localPlayerID)
		program.Quit()
	}()

	if _, err := program.Run(); err != nil {
		return err
	}

	g.RemoveSnake(localPlayerID)
	return nil
}

func runSSH(addr, hostKeyPath string, tickRate time.Duration) error {
	password := os.Getenv(passwordEnvName)
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("%s must be set in ssh mode", passwordEnvName)
	}

	g := game.NewGame(200, 60)
	hub := NewHub()

	engineCh := game.StartEngine(g, tickRate)
	go func() {
		for snap := range engineCh {
			hub.Broadcast(snap)
		}
	}()

	if err := os.MkdirAll(filepath.Dir(hostKeyPath), 0o755); err != nil {
		return err
	}

	srv, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithIdleTimeout(10*time.Minute),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool {
			return false
		}),
		wish.WithPasswordAuth(func(_ ssh.Context, pass string) bool {
			return pass == password
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

	log.Printf("tsnake listening on %s", externalSSHAddr(addr))
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

		g.AddSnake(playerID, playerName)
		snapCh := hub.Register(playerID, g.Snapshot())

		cleanup := sync.OnceFunc(func() {
			hub.Unregister(playerID)
			g.RemoveSnake(playerID)
			hub.Broadcast(g.Snapshot())
		})

		go func() {
			<-sess.Context().Done()
			cleanup()
		}()

		hub.Broadcast(g.Snapshot())

		return player.NewModel(g, playerID, snapCh, wishbubbletea.MakeRenderer(sess)), []tea.ProgramOption{tea.WithAltScreen()}
	}
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
	name := strings.TrimSpace(sess.User())
	if name == "" || strings.EqualFold(name, "git") {
		return "anon-" + sessionPlayerID(sess)
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
		return "anon-" + sessionPlayerID(sess)
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

func startPprofServer(addr string) error {
	if addr == "" {
		return nil
	}
	if !isLoopbackAddr(addr) {
		return fmt.Errorf("pprof address %q must bind to loopback only", addr)
	}

	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("pprof listening on http://%s/debug/pprof/", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("pprof server error: %v", err)
		}
	}()

	return nil
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	switch host {
	case "127.0.0.1", "localhost", "::1", "[::1]":
		return true
	default:
		return false
	}
}
