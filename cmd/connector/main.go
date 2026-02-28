package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/mudsahni/satvos-tally-connector/internal/cloud"
	"github.com/mudsahni/satvos-tally-connector/internal/config"
	"github.com/mudsahni/satvos-tally-connector/internal/store"
	"github.com/mudsahni/satvos-tally-connector/internal/sync"
	"github.com/mudsahni/satvos-tally-connector/internal/tally"
	"github.com/mudsahni/satvos-tally-connector/internal/ui"
)

const version = "0.1.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	log.Printf("SATVOS Tally Connector v%s starting...", version)

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 2. Local state store
	stateDir := stateDirectory()
	localStore, err := store.New(stateDir)
	if err != nil {
		return fmt.Errorf("initializing store: %w", err)
	}

	// 3. Discover Tally
	tallyPort := cfg.Tally.Port
	if tallyPort == 0 {
		state := localStore.Get()
		if state.TallyPort > 0 {
			tallyPort = state.TallyPort
			log.Printf("Using cached Tally port: %d", tallyPort)
		} else {
			discovered, discErr := tally.Discover(context.Background(), cfg.Tally.Host)
			if discErr != nil {
				log.Printf("WARNING: Tally not found: %v (will retry in sync cycle)", discErr)
				tallyPort = tally.DefaultPort
			} else {
				tallyPort = discovered
				_ = localStore.Update(func(s *store.State) { s.TallyPort = tallyPort })
			}
		}
	}

	// 4. Create clients
	cloudClient := cloud.NewClient(cfg.SATVOS.BaseURL, cfg.SATVOS.APIKey)
	tallyClient := tally.NewClient(cfg.Tally.Host, tallyPort)

	// 5. Register with SATVOS
	resp, err := cloudClient.Register(context.Background(), cloud.RegisterRequest{
		Version: version,
		OSInfo:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	})
	if err != nil {
		log.Printf("WARNING: registration failed: %v (will retry via heartbeat)", err)
	} else {
		log.Printf("Registered as agent %s", resp.ID)
		_ = localStore.Update(func(s *store.State) { s.AgentID = resp.ID })
	}

	// 6. Create sync engine
	engine := sync.NewEngine(cfg, cloudClient, tallyClient, localStore, version)

	// 7. Create UI server
	uiServer := ui.NewServer(cfg.UI.Port, engine)

	// 8. Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return engine.Start(gctx) })
	g.Go(func() error { return uiServer.Start(gctx) })

	log.Printf("Sync engine running (interval: %ds). UI at http://localhost:%d", cfg.Sync.IntervalSeconds, cfg.UI.Port)

	if err := g.Wait(); err != nil && err != context.Canceled {
		return err
	}
	log.Println("Connector stopped")
	return nil
}

func stateDirectory() string {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "satvos-connector")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".satvos-connector")
}
