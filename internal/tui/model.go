// Package tui implements the interactive pingorb dashboard: a live server list
// next to a world map with ping-status markers, plus in-place server
// management (add/edit/delete).
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vergissberlin/pingorb/internal/config"
	"github.com/vergissberlin/pingorb/internal/pinger"
	"github.com/vergissberlin/pingorb/internal/worldmap"
)

const tickInterval = 500 * time.Millisecond

// flashWindow is how long a server's dot/marker takes to fade from a
// bright flash back to its normal status color after a fresh ping reply
// comes in, so updates are visible even though the dashboard otherwise
// looks static between pings.
const flashWindow = 1 * time.Second

type mode int

const (
	modeList mode = iota
	modeForm
	modeConfirmDelete
)

const (
	leftWidth  = 34
	headerRows = 2
	footerRows = 2
)

// Model is the root Bubble Tea model for the pingorb dashboard.
type Model struct {
	cfg        *config.Config
	monitor    *pinger.Monitor
	privileged bool
	interval   time.Duration

	servers []config.Server
	stats   map[string]pinger.Stats
	cursor  int

	mode        mode
	frm         form
	confirmName string
	err         string

	width, height int
	mapGrid       *worldmap.Grid
	mapW, mapH    int
}

// New builds a Model. The monitor is expected to already have every server
// in cfg registered (see cmd/pingorb).
func New(cfg *config.Config, monitor *pinger.Monitor, privileged bool, interval time.Duration) Model {
	return Model{
		cfg:        cfg,
		monitor:    monitor,
		privileged: privileged,
		interval:   interval,
		servers:    append([]config.Server(nil), cfg.Servers...),
		stats:      map[string]pinger.Stats{},
		width:      100,
		height:     30,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), tea.RequestWindowSize)
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.recalcMap()
		return m, nil

	case tickMsg:
		m.stats = m.monitor.Snapshot()
		return m, tickCmd()

	case tea.KeyPressMsg:
		switch m.mode {
		case modeForm:
			return m.updateForm(msg)
		case modeConfirmDelete:
			return m.updateConfirm(msg)
		default:
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m *Model) recalcMap() {
	m.mapW = m.width - leftWidth - 3
	m.mapH = m.height - headerRows - footerRows
	if m.mapW < 10 {
		m.mapW = 10
	}
	if m.mapH < 5 {
		m.mapH = 5
	}
	m.mapGrid = worldmap.Generate(m.mapW, m.mapH)
}

func (m Model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.monitor.StopAll()
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.servers)-1 {
			m.cursor++
		}

	case "a":
		m.frm = newForm()
		m.mode = modeForm
		m.err = ""

	case "e":
		if s, ok := m.selected(); ok {
			f := newForm()
			f.loadForEdit(s.Name, s.Host, s.Lat, s.Lon)
			m.frm = f
			m.mode = modeForm
			m.err = ""
		}

	case "d", "x":
		if s, ok := m.selected(); ok {
			m.confirmName = s.Name
			m.mode = modeConfirmDelete
		}
	}
	return m, nil
}

func (m Model) selected() (config.Server, bool) {
	if m.cursor < 0 || m.cursor >= len(m.servers) {
		return config.Server{}, false
	}
	return m.servers[m.cursor], true
}

func (m Model) updateConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		m.monitor.Remove(m.confirmName)
		_ = m.cfg.Remove(m.confirmName)
		if err := m.cfg.Save(); err != nil {
			m.err = err.Error()
		}
		m.servers = append([]config.Server(nil), m.cfg.Servers...)
		if m.cursor >= len(m.servers) {
			m.cursor = len(m.servers) - 1
		}
		m.mode = modeList
	case "n", "esc":
		m.mode = modeList
	}
	return m, nil
}

func (m Model) updateForm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil

	case "enter":
		name, host, lat, lon, err := m.frm.validate()
		if err != nil {
			m.frm.err = err.Error()
			return m, nil
		}

		s := config.Server{Name: name, Host: host, Lat: lat, Lon: lon}

		if m.frm.editing {
			m.monitor.Remove(m.frm.original)
			if name != m.frm.original {
				_ = m.cfg.Remove(m.frm.original)
				if err := m.cfg.Add(s); err != nil {
					m.frm.err = err.Error()
					m.monitor.Add(m.frm.original, s.Host, s.Lat, s.Lon, m.interval, m.privileged)
					return m, nil
				}
			} else if err := m.cfg.Update(s); err != nil {
				m.frm.err = err.Error()
				return m, nil
			}
		} else if err := m.cfg.Add(s); err != nil {
			m.frm.err = err.Error()
			return m, nil
		}

		if err := m.cfg.Save(); err != nil {
			m.frm.err = err.Error()
			return m, nil
		}

		m.monitor.Add(s.Name, s.Host, s.Lat, s.Lon, m.interval, m.privileged)
		m.servers = append([]config.Server(nil), m.cfg.Servers...)
		m.mode = modeList
		return m, nil
	}

	var cmd tea.Cmd
	m.frm, cmd = m.frm.update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true

	header := m.renderHeader()
	footer := helpStyle.Render(" a add · e edit · d delete · ↑/k ↓/j select · q quit")

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderList(),
		dimStyle.Render(" │ "),
		m.renderMap(),
	)

	content := header + "\n" + body + "\n" + footer

	switch m.mode {
	case modeForm:
		content = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.frm.view())
	case modeConfirmDelete:
		content = fmt.Sprintf("Delete server %q?\n\n", m.confirmName) + helpStyle.Render("y confirm · n/esc cancel")
		content = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalStyle.Render(content))
	}
	v.SetContent(content)
	return v
}

func (m Model) renderHeader() string {
	badge := badgeStyle.Render(fmt.Sprintf(" pingorb · %d servers ", len(m.servers)))
	clock := clockStyle.Render(time.Now().UTC().Format("2006-01-02 15:04:05") + " UTC")

	gap := m.width - lipgloss.Width(badge) - lipgloss.Width(clock)
	if gap < 1 {
		gap = 1
	}
	return badge + strings.Repeat(" ", gap) + clock
}

func (m Model) renderList() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("Servers") + "\n")

	if len(m.servers) == 0 {
		b.WriteString(dimStyle.Render("No servers yet.\nPress 'a' to add one.") + "\n")
	}

	for i, s := range m.servers {
		st, ok := m.stats[s.Name]
		dot := dimStyle.Render("●")
		latency := dimStyle.Render("  --  ")
		if ok {
			rttMS := float64(st.LastRTT) / float64(time.Millisecond)
			style := statusStyle(st.Alive, rttMS)
			dotStyle := style
			if elapsed := time.Since(st.Updated); elapsed < flashWindow {
				dotStyle = fadeStyle(st.Alive, rttMS, float64(elapsed)/float64(flashWindow))
			}
			dot = dotStyle.Render("●")
			if st.Alive {
				latency = style.Render(fmt.Sprintf("%5.1fms", rttMS))
			} else {
				latency = style.Render(" down ")
			}
		}

		name := s.Name
		if len(name) > leftWidth-14 {
			name = name[:leftWidth-14]
		}
		row := fmt.Sprintf("%s %-*s %s", dot, leftWidth-13, name, latency)

		if i == m.cursor {
			b.WriteString(selectedRowStyle.Render(row) + "\n")
		} else {
			b.WriteString(normalRowStyle.Render(row) + "\n")
		}
	}

	return lipgloss.NewStyle().Width(leftWidth).Height(m.height - headerRows - footerRows).Render(b.String())
}

func (m Model) renderMap() string {
	if m.mapGrid == nil {
		m.recalcMap()
	}
	grid := m.mapGrid

	// marker overlay: name -> style, keyed by (row,col)
	type marker struct {
		style lipgloss.Style
	}
	markers := make(map[[2]int]marker)
	for _, s := range m.servers {
		if s.Lat == 0 && s.Lon == 0 {
			continue // unset position, nothing sensible to plot
		}
		col, row := worldmap.Project(s.Lat, s.Lon, grid.Width, grid.Height)
		style := dimStyle
		if st, ok := m.stats[s.Name]; ok {
			rttMS := float64(st.LastRTT) / float64(time.Millisecond)
			style = statusStyle(st.Alive, rttMS)
			if elapsed := time.Since(st.Updated); elapsed < flashWindow {
				style = fadeStyle(st.Alive, rttMS, float64(elapsed)/float64(flashWindow))
			}
		}
		markers[[2]int{row, col}] = marker{style: style}
	}

	var b strings.Builder
	for row := 0; row < grid.Height; row++ {
		for col := 0; col < grid.Width; col++ {
			if mk, ok := markers[[2]int{row, col}]; ok {
				b.WriteString(mk.style.Render("●"))
				continue
			}
			b.WriteString(dimStyle.Render(string(grid.Cells[row][col])))
		}
		if row < grid.Height-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
