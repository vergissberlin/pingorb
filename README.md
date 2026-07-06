# pingorb

Live ping status for your servers, plotted on a terminal world map.

`pingorb` continuously pings a set of configured hosts and renders their
health as colored markers over an ASCII world map, right in your terminal.
Servers are kept in a simple YAML config file, which you can manage either
from the CLI (`pingorb add`) or interactively from within the dashboard.

## Features

- **Live world map** — an ASCII map with servers plotted at their
  lat/lon position, colored by status (green = healthy, yellow = high
  latency, red = down).
- **Continuous ICMP pinging** — one background pinger per server, with
  automatic retry on transient failures (DNS, permissions, etc).
- **In-app server management** — add, edit, and delete servers directly
  from the dashboard, no need to hand-edit YAML.
- **Optional GeoIP lookup** — resolve a server's approximate position
  automatically from its host/IP if you don't know the coordinates.
- **Plain YAML config** — easy to version-control, script, or edit by hand.

## Install

Requires Go 1.26+.

```bash
go install github.com/vergissberlin/pingorb/cmd/pingorb@latest
```

Or build from source:

```bash
git clone https://github.com/vergissberlin/pingorb.git
cd pingorb
go build -o pingorb ./cmd/pingorb
```

## Usage

### Add a server

```bash
pingorb add fra-edge 1.1.1.1 --lat 50.1109 --lon 8.6821
```

If you omit `--lat`/`--lon`, pingorb resolves an approximate position via a
public IP-geolocation API (disable with `--geoip=false`):

```bash
pingorb add gh-edge github.com
```

### List / remove servers

```bash
pingorb list
pingorb remove fra-edge
```

### Launch the dashboard

```bash
pingorb
```

**Keybindings:**

| Key             | Action                    |
| --------------- | ------------------------- |
| `↑`/`k`, `↓`/`j` | Move selection             |
| `a`              | Add a server               |
| `e`              | Edit the selected server   |
| `d`/`x`          | Delete the selected server |
| `q`, `ctrl+c`    | Quit                        |

Inside the add/edit form:

| Key                | Action                                  |
| ------------------ | ---------------------------------------- |
| `tab`/`shift+tab`   | Move between fields                      |
| `ctrl+g`            | Auto-resolve lat/lon from the host field |
| `enter`             | Save                                      |
| `esc`               | Cancel                                    |

### Flags

| Flag           | Default            | Description                                             |
| -------------- | ------------------- | -------------------------------------------------------- |
| `--config`     | OS config dir        | Path to `servers.yaml`                                    |
| `--interval`   | `1s`                  | Ping interval                                             |
| `--privileged` | `false`               | Use raw ICMP sockets instead of unprivileged UDP-based ICMP (needs root/`cap_net_raw`) |

## Configuration

By default, the config lives at `servers.yaml` in your OS config directory
(e.g. `~/Library/Application Support/pingorb` on macOS, `~/.config/pingorb`
on Linux). Override the path with `--config` or the `PINGORB_CONFIG`
environment variable.

```yaml
servers:
  - name: fra-edge
    host: 1.1.1.1
    lat: 50.1109
    lon: 8.6821
  - name: gh-edge
    host: github.com
    lat: 51.5113
    lon: -0.0792
    interval_ms: 2000 # optional per-server override
```

## How it works

- **Pinging** — [`prometheus-community/pro-bing`](https://github.com/prometheus-community/pro-bing)
  runs one continuous pinger per server in the background; results are
  aggregated into thread-safe stats snapshots the dashboard polls twice a
  second.
- **The map** — a deliberately coarse polygon approximation of the
  continents, rasterized into a stippled character grid at whatever size
  fits your terminal. It's built for recognizability at low resolution, not
  survey-accurate coastlines.
- **The dashboard** — built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
  and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## License

No license specified yet.
