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
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ismail/tsnake/game"
	"github.com/ismail/tsnake/player"
	"github.com/ismail/tsnake/server"
)

const (
	localPlayerID   = "local-player"
	localPlayerName = "local"
	defaultAddr     = ":2222"
	defaultHostKey  = "data/host_key"
	passwordEnvName = "TSNAKE_PASSWORD"
	worldWidth      = 240
	worldHeight     = 72
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
		err = server.RunSSH(server.Config{
			Addr:        *addr,
			HostKeyPath: *hostKeyPath,
			TickRate:    *tickRate,
			Password:    os.Getenv(passwordEnvName),
		})
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}

	if err != nil {
		log.Fatal(err)
	}
}

func runLocal(tickRate time.Duration) error {
	g := game.NewGame(worldWidth, worldHeight)

	snapCh := game.StartEngine(g, tickRate)
	model := player.NewModel(g, localPlayerID, localPlayerName, snapCh, nil, nil)
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
