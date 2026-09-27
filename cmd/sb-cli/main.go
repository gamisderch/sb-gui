package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"signbox-cli/pkg/core"
	"signbox-cli/pkg/model"
	"signbox-cli/pkg/process"
	"signbox-cli/pkg/subscription"
	"signbox-cli/pkg/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func printUsage() {
	fmt.Println(`signbox-cli — High-performance terminal client for sing-box

Usage:
  sb-cli                      Launch interactive TUI
  sb-cli --start              Start VPN in background (daemon mode)
  sb-cli --stop               Stop running background VPN
  sb-cli --status             Check current VPN status
  sb-cli --install-service    Install systemd user service for auto-start
  sb-cli --uninstall-service  Uninstall systemd user service
  sb-cli --help               Show this help message`)
}

func main() {
	procMgr := process.NewProcessManager()
	engine := core.NewEngine()

	// Try loading saved configuration
	var state *model.AppState
	cfg, err := core.LoadConfig()
	if err == nil && cfg != nil {
		state = &model.AppState{
			Mode:          cfg.Mode,
			UserAgent:     cfg.UserAgent,
			Subscriptions: cfg.Subscriptions,
			IsConnected:   false,
			ThemeColors:   cfg.ThemeColors,
			GeoRules:      cfg.GeoRules,
		}
		if cfg.ThemeColors != nil {
			tui.ApplyThemeColors(cfg.ThemeColors)
		}
		// Restore bypass names
		for _, name := range cfg.BypassNames {
			procMgr.BypassNames[name] = true
		}

		// Restore active node and ensure node details are fresh from raw_uri
		for _, sub := range state.Subscriptions {
			for _, n := range sub.Nodes {
				if n.RawURI != "" {
					if reParsed, err := subscription.ParseNodeURI(n.RawURI, sub.Name); err == nil {
						n.Details = reParsed.Details
						n.Protocol = reParsed.Protocol
						n.Address = reParsed.Address
						n.Port = reParsed.Port
					}
				}
				if cfg.ActiveNodeID != "" && n.ID == cfg.ActiveNodeID {
					state.ActiveNode = n
				}
			}
		}
	}

	// Clean initial state with NO fake subscriptions
	if state == nil {
		state = &model.AppState{
			Mode:          model.ModeTUN,
			IsConnected:   false,
			UserAgent:     subscription.DefaultUserAgent,
			Subscriptions: nil,
		}
	}

	if state.ActiveNode == nil && len(state.Subscriptions) > 0 && len(state.Subscriptions[0].Nodes) > 0 {
		state.ActiveNode = state.Subscriptions[0].Nodes[0]
	}

	// Check if background VPN is already running
	if engine.IsRunning() {
		state.IsConnected = true
	}

	var bypassList []string
	for name := range procMgr.BypassNames {
		bypassList = append(bypassList, name)
	}

	// CLI argument handling
	if len(os.Args) > 1 {
		arg := os.Args[1]
		switch arg {
		case "--start", "-start", "--daemon", "-d":
			if engine.IsRunning() {
				fmt.Println("signbox-cli: VPN tunnel is already running.")
				return
			}
			if state.ActiveNode == nil {
				fmt.Fprintln(os.Stderr, "Error: No proxy node configured. Please run 'sb-cli' to add a subscription first.")
				os.Exit(1)
			}
			fmt.Printf("Starting VPN tunnel to %s (%s:%d)...\n", state.ActiveNode.Name, state.ActiveNode.Address, state.ActiveNode.Port)
			if err := engine.Start(state.ActiveNode, bypassList, state.GeoRules); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting VPN: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("VPN tunnel started successfully in background.")
			return

		case "--stop", "-stop":
			if !engine.IsRunning() {
				fmt.Println("signbox-cli: VPN is not running.")
				return
			}
			fmt.Println("Stopping VPN tunnel and releasing interfaces...")
			_ = engine.Stop()
			fmt.Println("VPN tunnel stopped.")
			return

		case "--status", "-status":
			if engine.IsRunning() {
				fmt.Println("VPN Status: [ACTIVE]")
				if state.ActiveNode != nil {
					fmt.Printf("Active Node: %s (%s:%d) [%s]\n", state.ActiveNode.Name, state.ActiveNode.Address, state.ActiveNode.Port, state.ActiveNode.Protocol)
				}
			} else {
				fmt.Println("VPN Status: [DISCONNECTED]")
			}
			if core.IsSystemdServiceEnabled() {
				fmt.Println("Systemd Auto-start Service: ENABLED")
			} else {
				fmt.Println("Systemd Auto-start Service: DISABLED")
			}
			return

		case "--install-service":
			path, err := core.InstallSystemdService()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error installing service: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Systemd user service installed and enabled:\n  %s\n", path)
			fmt.Println("signbox-cli VPN will now start automatically on system boot.")
			return

		case "--uninstall-service":
			if err := core.UninstallSystemdService(); err != nil {
				fmt.Fprintf(os.Stderr, "Error uninstalling service: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Systemd user service uninstalled.")
			return

		case "--help", "-h", "help":
			printUsage()
			return

		default:
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n\n", arg)
			printUsage()
			os.Exit(1)
		}
	}

	keepAlive := false
	defer func() {
		if !keepAlive {
			_ = engine.Stop()
		}
	}()

	// Catch external OS termination signals (SIGTERM, SIGHUP)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigChan
		if !keepAlive {
			_ = engine.Stop()
		}
		os.Exit(0)
	}()

	// Launch TUI
	app := tui.NewModel(state, procMgr, engine)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(*tui.Model); ok && m.ShouldKeepVPNAlive() {
		keepAlive = true
		fmt.Println("signbox-cli: VPN tunnel remains active in background.")
		fmt.Println("To stop the VPN, run: sb-cli --stop or launch TUI and press [c]")
		return
	}

	_ = engine.Stop()
}
