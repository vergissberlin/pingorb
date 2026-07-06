// Package pinger runs continuous ICMP pings against a set of targets and
// exposes a thread-safe, point-in-time snapshot of their health.
package pinger

import (
	"fmt"
	"math"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// DefaultInterval is used when a server doesn't override its ping interval.
const DefaultInterval = time.Second

// retryBackoff is how long to wait before retrying a target that failed to
// start (e.g. DNS not resolving yet, permission error).
const retryBackoff = 5 * time.Second

// Stats is a snapshot of one target's current health.
type Stats struct {
	Name        string
	Host        string
	Lat, Lon    float64
	Alive       bool
	LastRTT     time.Duration
	AvgRTT      time.Duration
	MinRTT      time.Duration
	MaxRTT      time.Duration
	PacketLoss  float64 // percent, 0-100
	PacketsSent int
	PacketsRecv int
	LastError   string
	Updated     time.Time
}

type target struct {
	name       string
	host       string
	lat, lon   float64
	interval   time.Duration
	privileged bool

	mu    sync.RWMutex
	stats Stats

	stop chan struct{}
	done chan struct{}
}

// Monitor supervises one ping goroutine per configured target.
type Monitor struct {
	mu      sync.RWMutex
	targets map[string]*target
}

// NewMonitor returns an empty, ready-to-use Monitor.
func NewMonitor() *Monitor {
	return &Monitor{targets: make(map[string]*target)}
}

// Add begins pinging name@host in the background. privileged selects raw
// ICMP sockets (requires root/cap_net_raw) vs. unprivileged UDP-based ICMP.
func (m *Monitor) Add(name, host string, lat, lon float64, interval time.Duration, privileged bool) {
	if interval <= 0 {
		interval = DefaultInterval
	}

	t := &target{
		name:       name,
		host:       host,
		lat:        lat,
		lon:        lon,
		interval:   interval,
		privileged: privileged,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	t.stats = Stats{Name: name, Host: host, Lat: lat, Lon: lon, Updated: time.Now()}

	m.mu.Lock()
	if old, ok := m.targets[name]; ok {
		m.mu.Unlock()
		m.remove(old)
		m.mu.Lock()
	}
	m.targets[name] = t
	m.mu.Unlock()

	go t.run()
}

// Remove stops pinging and forgets the named target.
func (m *Monitor) Remove(name string) {
	m.mu.Lock()
	t, ok := m.targets[name]
	if ok {
		delete(m.targets, name)
	}
	m.mu.Unlock()
	if ok {
		m.remove(t)
	}
}

func (m *Monitor) remove(t *target) {
	close(t.stop)
	<-t.done
}

// StopAll halts every ping goroutine. Call before process exit.
func (m *Monitor) StopAll() {
	m.mu.Lock()
	targets := make([]*target, 0, len(m.targets))
	for _, t := range m.targets {
		targets = append(targets, t)
	}
	m.targets = make(map[string]*target)
	m.mu.Unlock()

	for _, t := range targets {
		m.remove(t)
	}
}

// Snapshot returns a copy of the current stats for every monitored target.
func (m *Monitor) Snapshot() map[string]Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]Stats, len(m.targets))
	for name, t := range m.targets {
		out[name] = t.snapshot()
	}
	return out
}

func (t *target) snapshot() Stats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stats
}

func (t *target) setError(err error) {
	t.mu.Lock()
	t.stats.Alive = false
	t.stats.LastError = err.Error()
	t.stats.Updated = time.Now()
	t.mu.Unlock()
}

func (t *target) run() {
	defer close(t.done)

	for {
		select {
		case <-t.stop:
			return
		default:
		}

		if err := t.pingOnce(); err != nil {
			t.setError(err)
		}

		select {
		case <-t.stop:
			return
		case <-time.After(retryBackoff):
		}
	}
}

func (t *target) pingOnce() error {
	p, err := probing.NewPinger(t.host)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", t.host, err)
	}
	p.Interval = t.interval
	p.Timeout = 365 * 24 * time.Hour // effectively "run until stopped"
	p.SetPrivileged(t.privileged)

	p.OnRecv = func(pkt *probing.Packet) {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.stats.Alive = true
		t.stats.LastRTT = pkt.Rtt
		t.stats.LastError = ""
		t.stats.Updated = time.Now()
		t.recomputeLocked(p)
	}
	p.OnSend = func(pkt *probing.Packet) {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.recomputeLocked(p)
		// A packet that hasn't come back yet after ~2 intervals is a
		// stronger "down" signal than waiting for the whole run to end.
		if t.stats.PacketsSent > 1 && time.Since(t.stats.Updated) > 2*t.interval {
			t.stats.Alive = false
		}
	}

	done := make(chan error, 1)
	go func() { done <- p.Run() }()

	select {
	case <-t.stop:
		p.Stop()
		<-done
		return nil
	case err := <-done:
		return err
	}
}

// recomputeLocked refreshes aggregate counters from the pinger's running
// statistics. Caller must hold t.mu.
func (t *target) recomputeLocked(p *probing.Pinger) {
	s := p.Statistics()
	t.stats.PacketsSent = s.PacketsSent
	t.stats.PacketsRecv = s.PacketsRecv
	t.stats.MinRTT = s.MinRtt
	t.stats.MaxRTT = s.MaxRtt
	t.stats.AvgRTT = s.AvgRtt
	if s.PacketsSent > 0 {
		t.stats.PacketLoss = math.Max(0, s.PacketLoss)
	}
}
