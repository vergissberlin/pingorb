// Command pingorb shows live ping status for a set of configured servers on a
// terminal world map.
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/vergissberlin/pingorb/internal/config"
	"github.com/vergissberlin/pingorb/internal/geoip"
	"github.com/vergissberlin/pingorb/internal/pinger"
	"github.com/vergissberlin/pingorb/internal/tui"
)

var (
	configPath string
	privileged bool
	interval   time.Duration
)

// Set via -ldflags at build time (see .goreleaser.yaml); left as "dev" for
// plain `go build`/`go run`.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	root := &cobra.Command{
		Use:     "pingorb",
		Short:   "Live ping status for your servers on a terminal world map",
		Version: fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		RunE:    runDashboard,
	}
	root.PersistentFlags().StringVar(&configPath, "config", "", "path to servers.yaml (default: OS config dir)")
	root.PersistentFlags().BoolVar(&privileged, "privileged", false, "use raw ICMP sockets (requires root/cap_net_raw)")
	root.PersistentFlags().DurationVar(&interval, "interval", pinger.DefaultInterval, "ping interval")

	root.AddCommand(addCmd(), removeCmd(), listCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "pingorb:", err)
		os.Exit(1)
	}
}

func resolveConfigPath() (string, error) {
	if configPath != "" {
		return configPath, nil
	}
	return config.DefaultPath()
}

func loadConfig() (*config.Config, error) {
	path, err := resolveConfigPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	return config.Load(path)
}

func runDashboard(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	monitor := pinger.NewMonitor()
	for _, s := range cfg.Servers {
		iv := interval
		if s.Interval > 0 {
			iv = time.Duration(s.Interval) * time.Millisecond
		}
		monitor.Add(s.Name, s.Host, s.Lat, s.Lon, iv, privileged)
	}
	defer monitor.StopAll()

	m := tui.New(cfg, monitor, privileged, interval)
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}

func addCmd() *cobra.Command {
	var lat, lon float64
	var lookupGeo bool

	cmd := &cobra.Command{
		Use:   "add <name> <host>",
		Short: "Add a server to the config",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			name, host := args[0], args[1]

			if lookupGeo && lat == 0 && lon == 0 {
				l, o, err := geoip.Lookup(context.Background(), host)
				if err != nil {
					fmt.Fprintln(os.Stderr, "pingorb: geoip lookup failed:", err)
				} else {
					lat, lon = l, o
				}
			}

			if err := cfg.Add(config.Server{Name: name, Host: host, Lat: lat, Lon: lon}); err != nil {
				return err
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("added %s (%s) at %.4f,%.4f -> %s\n", name, host, lat, lon, cfg.Path())
			return nil
		},
	}

	cmd.Flags().Float64Var(&lat, "lat", 0, "latitude")
	cmd.Flags().Float64Var(&lon, "lon", 0, "longitude")
	cmd.Flags().BoolVar(&lookupGeo, "geoip", true, "auto-resolve lat/lon from host via public IP geolocation when not given")
	return cmd
}

func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a server from the config",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := cfg.Remove(args[0]); err != nil {
				return err
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("removed %s from %s\n", args[0], cfg.Path())
			return nil
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if len(cfg.Servers) == 0 {
				fmt.Println("no servers configured; add one with: pingorb add <name> <host>")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tHOST\tLAT\tLON")
			for _, s := range cfg.Servers {
				fmt.Fprintf(w, "%s\t%s\t%.4f\t%.4f\n", s.Name, s.Host, s.Lat, s.Lon)
			}
			return w.Flush()
		},
	}
}
