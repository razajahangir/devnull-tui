// Command devnull is a terminal client for https://dev-null.sh.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/razajahangir/devnull-tui/internal/api"
	"github.com/razajahangir/devnull-tui/internal/auth"
	"github.com/razajahangir/devnull-tui/internal/config"
	"github.com/razajahangir/devnull-tui/internal/ui"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "callback" {
		raw := ""
		if len(os.Args) >= 3 {
			raw = strings.Join(os.Args[2:], " ")
		}
		if err := auth.WriteCallback(raw); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	fs := flag.NewFlagSet("devnull", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	urlFlag := fs.String("url", "", "instance URL (default: "+config.DefaultURL+")")
	configFlag := fs.String("config", "", "path to config.json")
	pollFlag := fs.String("poll-interval", "", "background poll interval (10s–10m)")
	autoLoadFlag := fs.String("auto-load-new", "", "load new commits at the top of a feed (true/false)")
	refreshSwitchFlag := fs.String("refresh-on-switch", "", "reload a tab when you switch to it if it is stale (true/false)")
	mouseFlag := fs.String("mouse", "", "enable mouse wheel (true/false)")
	insecureFlag := fs.String("allow-insecure-http", "", "allow http:// to non-loopback hosts (true/false)")
	deviceFlag := fs.String("device-name", "", "name shown on the instance's device list")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	if *showVersion {
		fmt.Println("devnull", version)
		return
	}

	res, err := config.Load(config.Overrides{
		ConfigPath:        *configFlag,
		URL:               *urlFlag,
		PollInterval:      *pollFlag,
		AutoLoadNew:       *autoLoadFlag,
		RefreshOnSwitch:   *refreshSwitchFlag,
		Mouse:             *mouseFlag,
		AllowInsecureHTTP: *insecureFlag,
		DeviceName:        *deviceFlag,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg := res.Config
	key, keyNotice, err := config.LoadKey(cfg.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if keyNotice != "" {
		if res.Notice != "" {
			res.Notice += "; " + keyNotice
		} else {
			res.Notice = keyNotice
		}
	}
	args := fs.Args()
	if len(args) > 0 && args[0] == "config" {
		fmt.Print(res.Format())
		return
	}
	if res.Notice != "" {
		fmt.Fprintln(os.Stderr, "note:", res.Notice)
	}
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "unknown command %q (try devnull, devnull config, or devnull --version)\n", args[0])
		os.Exit(2)
	}

	api.UserAgent = "devnull-tui/" + version
	client := api.NewWithOptions(cfg.URL, key, cfg.AllowInsecureHTTP)
	loggedIn := key != nil

	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if cfg.Mouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(ui.New(cfg, client, loggedIn), opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
