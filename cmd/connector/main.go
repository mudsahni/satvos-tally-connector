package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/mudsahni/satvos-tally-connector/internal/cloud"
	"github.com/mudsahni/satvos-tally-connector/internal/config"
	"github.com/mudsahni/satvos-tally-connector/internal/store"
	"github.com/mudsahni/satvos-tally-connector/internal/svc"
	engsync "github.com/mudsahni/satvos-tally-connector/internal/sync"
	"github.com/mudsahni/satvos-tally-connector/internal/tally"
	"github.com/mudsahni/satvos-tally-connector/internal/ui"
)

const version = "0.2.1"

func main() {
	if svc.IsWindowsService() {
		if err := svc.Run(func(ctx context.Context) error {
			return run()
		}); err != nil {
			log.Fatalf("service failed: %v", err)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	log.Printf("SATVOS Tally Connector v%s (build: xml-sanitize+post-outbound) starting...", version)

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	stateDir := stateDirectory()

	// 2. Setup mode — no API key configured yet
	if cfg.NeedsSetup() {
		return runSetupMode(cfg, stateDir)
	}

	// 3. Normal mode — fully configured
	return runNormalMode(cfg, stateDir)
}

// runSetupMode starts only the UI server so the user can enter their API key
// via the setup wizard at http://localhost:<port>/setup.html.
// When the user saves a valid API key, the startEngine callback initializes and
// starts the sync engine inline — no restart required.
func runSetupMode(cfg *config.Config, stateDir string) error {
	log.Println("No API key configured — starting in setup mode")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// The startEngine callback is invoked by the UI handler when the user saves
	// an API key. It wires up the full sync infrastructure and returns a ready
	// engine (not yet started — the handler starts it in a goroutine).
	startEngine := func(apiKey string) (*engsync.Engine, error) {
		// Reload config so it picks up the newly-written connector.yaml.
		newCfg, err := config.Load()
		if err != nil {
			return nil, fmt.Errorf("reloading config: %w", err)
		}

		localStore, err := store.New(stateDir)
		if err != nil {
			return nil, fmt.Errorf("initializing store: %w", err)
		}

		// Discover Tally
		tallyPort := newCfg.Tally.Port
		if tallyPort == 0 {
			state := localStore.Get()
			if state.TallyPort > 0 {
				tallyPort = state.TallyPort
				log.Printf("Using cached Tally port: %d", tallyPort)
			} else {
				discovered, discErr := tally.Discover(ctx, newCfg.Tally.Host)
				if discErr != nil {
					log.Printf("WARNING: Tally not found: %v (will retry in sync cycle)", discErr)
					tallyPort = tally.DefaultPort
				} else {
					tallyPort = discovered
					_ = localStore.Update(func(s *store.State) { s.TallyPort = tallyPort })
				}
			}
		}

		cloudClient := cloud.NewClient(newCfg.SATVOS.BaseURL, newCfg.SATVOS.APIKey)
		tallyClient := tally.NewClient(newCfg.Tally.Host, tallyPort)

		// Register with SATVOS
		resp, regErr := cloudClient.Register(ctx, cloud.RegisterRequest{
			Version: version,
			OSInfo:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		})
		if regErr != nil {
			log.Printf("WARNING: registration failed: %v (will retry via heartbeat)", regErr)
		} else {
			log.Printf("Registered as agent %s", resp.ID)
			_ = localStore.Update(func(s *store.State) { s.AgentID = resp.ID })
		}

		engine := engsync.NewEngine(newCfg, cloudClient, tallyClient, localStore, version)
		log.Printf("Sync engine initialized (interval: %ds)", newCfg.Sync.IntervalSeconds)
		return engine, nil
	}

	uiServer := ui.NewServer(cfg.UI.Port, nil, stateDir, startEngine)

	setupURL := fmt.Sprintf("http://localhost:%d/setup.html", cfg.UI.Port)
	log.Printf("Opening setup wizard: %s", setupURL)
	openBrowser(setupURL)

	log.Printf("Setup UI running at %s — configure your API key there", setupURL)

	if err := uiServer.Start(ctx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

// runNormalMode runs the full connector: discovery, registration, sync engine, and UI.
func runNormalMode(cfg *config.Config, stateDir string) error {
	// Local state store
	localStore, err := store.New(stateDir)
	if err != nil {
		return fmt.Errorf("initializing store: %w", err)
	}

	// Discover Tally
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

	// Create clients
	cloudClient := cloud.NewClient(cfg.SATVOS.BaseURL, cfg.SATVOS.APIKey)
	tallyClient := tally.NewClient(cfg.Tally.Host, tallyPort)

	// Register with SATVOS
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

	// Create sync engine and UI (no startEngine callback needed — engine already running)
	engine := engsync.NewEngine(cfg, cloudClient, tallyClient, localStore, version)
	uiServer := ui.NewServer(cfg.UI.Port, engine, stateDir, nil)

	// Graceful shutdown
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

// openBrowser opens the given URL in the user's default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("WARNING: could not open browser: %v", err)
	}
}
