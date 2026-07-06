package tui

import (
	"context"
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vergissberlin/pingorb/internal/geoip"
)

const (
	fieldName = iota
	fieldHost
	fieldLat
	fieldLon
	fieldCount
)

// form is the inline add/edit-server modal.
type form struct {
	inputs   [4]textinput.Model
	focus    int
	editing  bool
	original string // original server name, set when editing
	geoBusy  bool
	err      string
}

func newForm() form {
	f := form{}
	placeholders := [4]string{"e.g. edge-fra-1", "hostname or IP", "auto via ctrl+g", "auto via ctrl+g"}
	for i := range f.inputs {
		ti := textinput.New()
		ti.Placeholder = placeholders[i]
		ti.CharLimit = 64
		ti.Prompt = ""
		f.inputs[i] = ti
	}
	f.inputs[fieldName].Focus()
	return f
}

func (f *form) loadForEdit(name, host string, lat, lon float64) {
	f.editing = true
	f.original = name
	f.inputs[fieldName].SetValue(name)
	f.inputs[fieldHost].SetValue(host)
	f.inputs[fieldLat].SetValue(fmt.Sprintf("%.4f", lat))
	f.inputs[fieldLon].SetValue(fmt.Sprintf("%.4f", lon))
}

func (f *form) focusField(i int) {
	for j := range f.inputs {
		if j == i {
			f.inputs[j].Focus()
		} else {
			f.inputs[j].Blur()
		}
	}
	f.focus = i
}

type geoResultMsg struct {
	lat, lon float64
	err      error
}

func geoLookupCmd(host string) tea.Cmd {
	return func() tea.Msg {
		lat, lon, err := geoip.Lookup(context.Background(), host)
		return geoResultMsg{lat: lat, lon: lon, err: err}
	}
}

// update returns the possibly-updated form plus a tea.Cmd, and a bool
// signalling whether the form should close (submit or cancel handled by
// the caller based on the returned key).
func (f form) update(msg tea.Msg) (form, tea.Cmd) {
	switch msg := msg.(type) {
	case geoResultMsg:
		f.geoBusy = false
		if msg.err != nil {
			f.err = msg.err.Error()
		} else {
			f.err = ""
			f.inputs[fieldLat].SetValue(fmt.Sprintf("%.4f", msg.lat))
			f.inputs[fieldLon].SetValue(fmt.Sprintf("%.4f", msg.lon))
		}
		return f, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			f.focusField((f.focus + 1) % fieldCount)
			return f, nil
		case "shift+tab", "up":
			f.focusField((f.focus - 1 + fieldCount) % fieldCount)
			return f, nil
		case "ctrl+g":
			host := f.inputs[fieldHost].Value()
			if host == "" {
				f.err = "enter a host first"
				return f, nil
			}
			f.geoBusy = true
			f.err = ""
			return f, geoLookupCmd(host)
		}
	}

	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return f, cmd
}

// validate parses the current field values, returning a friendly error if
// something doesn't check out.
func (f form) validate() (name, host string, lat, lon float64, err error) {
	name = f.inputs[fieldName].Value()
	host = f.inputs[fieldHost].Value()
	if name == "" {
		return "", "", 0, 0, fmt.Errorf("name is required")
	}
	if host == "" {
		return "", "", 0, 0, fmt.Errorf("host is required")
	}
	if v := f.inputs[fieldLat].Value(); v != "" {
		lat, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return "", "", 0, 0, fmt.Errorf("invalid latitude")
		}
	}
	if v := f.inputs[fieldLon].Value(); v != "" {
		lon, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return "", "", 0, 0, fmt.Errorf("invalid longitude")
		}
	}
	return name, host, lat, lon, nil
}

func (f form) view() string {
	labels := [4]string{"Name", "Host", "Lat", "Lon"}
	title := "Add server"
	if f.editing {
		title = "Edit server: " + f.original
	}

	body := sectionStyle.Render(title) + "\n\n"
	for i, in := range f.inputs {
		body += labelStyle.Render(fmt.Sprintf("%-6s", labels[i])) + " " + in.View() + "\n"
	}
	body += "\n"
	if f.geoBusy {
		body += dimStyle.Render("resolving location…") + "\n"
	} else if f.err != "" {
		body += errStyle.Render(f.err) + "\n"
	}
	body += helpStyle.Render("tab/↑↓ move · ctrl+g geoip lookup · enter save · esc cancel")

	return modalStyle.Render(body)
}
