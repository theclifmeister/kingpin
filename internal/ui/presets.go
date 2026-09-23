package ui

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The presets dialog (#357, docs/presets.md): P on the market opens the
// operation presets, named bundles of the routine's dials, in one modal
// with two pages. The first is the list: the built-ins of presets.toml,
// then the player's own, saved in the profile so they carry across
// runs (s saves today's routine as one, named for the day; x deletes
// the one under the cursor). enter turns to the review: one line a
// setting the preset would move, from what it is to what it would be,
// with the estimate where there is one (a standing order's take and
// heat by orderEstimate, a contract's morning buy by the market's plan,
// each `~`), the settings that stay folded into a count and the
// commands the rules would refuse listed; enter there applies it, one
// session command at a time (Session.ApplyPreset), and nothing moves
// before it.

// presetsDialog is the dialog's state: the page, the preset under the
// cursor, the list as it was opened and the review on the second page.
type presetsDialog struct {
	stepper
	cursor int
	list   []engine.Preset
	review engine.Review
}

func (d *presetsDialog) field() *numberField { return nil }

// presetCols are the list's table; changeCols the review's.
var (
	presetCols = []col{{"preset", kText, 0}, {"what it sets", kText, 0}}
	changeCols = []col{{"setting", kText, 0}, {"now", kText, 0}, {"after", kText, 0}, {"estimate", kText, 0}}
)

// askPresets opens the dialog on its list.
func (m *Model) askPresets() {
	if m.w.Over != nil {
		return
	}
	m.pre = presetsDialog{}
	m.loadPresets()
	m.modalScroll = 0
	m.mode = modePresets
}

// loadPresets hands the session the profile's presets and reads the
// list back.
func (m *Model) loadPresets() {
	var saved []game.Preset
	if m.profile != nil {
		saved = m.profile.Presets
	}
	m.sess.UsePresets(saved)
	m.pre.list = m.sess.Presets()
	clamp(&m.pre.cursor, len(m.pre.list))
}

// presetSelected is the preset under the list's cursor, or nil.
func (m *Model) presetSelected() *engine.Preset {
	if m.pre.cursor < 0 || m.pre.cursor >= len(m.pre.list) {
		return nil
	}
	return &m.pre.list[m.pre.cursor]
}

// presetOnSaved is the list's cursor on a preset of the player's: where
// x deletes.
func presetOnSaved(m *Model) bool {
	p := m.presetSelected()
	return m.modalStep() == 0 && p != nil && p.Saved
}

// keyPresets is the dialog's keys: the list's cursor, digits and enter
// to the review, s to save the routine, x to delete a saved preset;
// on the review enter applies, the arrows scroll, shift+tab goes back.
func (m *Model) keyPresets(key string) {
	d := &m.pre
	d.err = ""
	if closes(key) {
		m.mode = modePlay
		return
	}
	if d.step == 1 {
		switch key {
		case "shift+tab":
			d.back(noField)
			m.modalScroll = 0
		case "enter":
			m.applyPreset()
		default:
			m.scrollModal(key)
		}
		return
	}
	switch key {
	case "up", "k":
		stepCursor(&d.cursor, -1, len(d.list))
	case "down", "j":
		stepCursor(&d.cursor, 1, len(d.list))
	case "enter", "tab":
		m.reviewPreset()
	case "s":
		m.savePreset()
	case "x":
		m.deletePreset()
	default:
		if i, ok := digit(key); ok && i < len(d.list) {
			d.cursor = i
			m.reviewPreset()
		}
	}
}

// reviewPreset turns to the review of the preset under the cursor.
func (m *Model) reviewPreset() {
	p := m.presetSelected()
	if p == nil {
		return
	}
	r, err := m.sess.PresetDiff(p.ID)
	if err != nil {
		m.pre.err = dialogError(err)
		return
	}
	m.pre.review = r
	m.pre.step = 1
	m.modalScroll = 0
}

// applyPreset applies the preset reviewed, or says there is nothing to.
func (m *Model) applyPreset() {
	r := m.pre.review
	if len(r.Changes) == 0 {
		m.pre.err = "Nothing to change: the routine is set that way already."
		return
	}
	got, err := m.sess.ApplyPreset(r.Preset.ID)
	if err != nil {
		m.pre.err = dialogError(err)
		return
	}
	m.mode = modePlay
	say := fmt.Sprintf("%s: %s changed.", got.Preset.Name, plural(len(got.Changes), "setting"))
	if n := len(got.Refused); n > 0 {
		m.alarm(say + fmt.Sprintf(" %s refused: %s.", plural(n, "command"), got.Refused[0].Why))
		return
	}
	m.say(say)
}

// savePreset keeps the routine as it stands in the profile, named for
// the day (a second save that day replaces the first).
func (m *Model) savePreset() {
	if m.profile == nil {
		m.profile = game.NewProfile()
	}
	name := fmt.Sprintf("Day %d", m.w.Day)
	m.profile.SavePreset(m.sess.Snapshot(name))
	m.saveProfile()
	m.loadPresets()
	for i, p := range m.pre.list {
		if p.Saved && p.Name == name {
			m.pre.cursor = i
		}
	}
	m.say(fmt.Sprintf("Saved today's routine as %s, kept across runs.", name))
}

// deletePreset drops the saved preset under the cursor.
func (m *Model) deletePreset() {
	p := m.presetSelected()
	if p == nil || !p.Saved {
		m.pre.err = "Only a preset of your own can be deleted."
		return
	}
	if m.profile.DeletePreset(p.Name) {
		m.saveProfile()
		m.say("Deleted the preset " + p.Name + ".")
	}
	m.loadPresets()
}

// viewPresets is the dialog: the list, then the review.
func (m *Model) viewPresets() string {
	d := &m.pre
	if d.step == 0 {
		var cells [][]any
		for _, p := range d.list {
			cells = append(cells, []any{p.Name, styled{theme.Subtle, p.Blurb}})
		}
		notes := m.subtle("Each is reviewed before it applies: what it would change, from what to what. Your own are kept with your profile, across runs.")
		if d.err != "" {
			notes = append(notes, "", theme.Bad.Render(d.err))
		}
		return m.pickerModal("PRESETS", nil, presetCols, cells, clamp(&d.cursor, len(d.list)), notes...)
	}
	r := d.review
	body := m.subtle(r.Preset.Blurb)
	body = append(body, "")
	if len(r.Changes) == 0 {
		body = append(body, theme.Subtle.Render("Nothing would change: the routine is set that way already."))
	} else {
		var cells [][]any
		for _, c := range r.Changes {
			cells = append(cells, []any{m.changeName(c), c.From, styled{theme.Gold, c.To}, styled{theme.Subtle, m.changeEstimate(c)}})
		}
		body = append(body, table(changeCols, cells, -1, m.modalInner())...)
	}
	if r.Same > 0 {
		body = append(body, "", theme.Subtle.Render(fmt.Sprintf("%s of the routine stay as they are.", plural(r.Same, "other setting"))))
	}
	for _, f := range r.Refused {
		body = append(body, theme.Bad.Render(fmt.Sprintf("Refused, left as it is: %s: %s.", m.commandName(f.Command), f.Why)))
	}
	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	return m.modal("PRESET · "+r.Preset.Name, body, m.modalFooter())
}

// changeName is the setting a change moves, as the review names it.
func (m *Model) changeName(c engine.Change) string {
	w := m.w
	switch c.Setting {
	case "standing":
		return "standing " + w.ProductName(c.Product) + ", " + w.CityName(c.City)
	case "supply":
		return "contract " + w.ProductName(c.Product) + ", " + w.CityName(c.City)
	case "launder":
		return "launder dial"
	case "pay":
		return "pay dial"
	case "route":
		return "route " + m.routeName(c.Route)
	case "target":
		return "target " + w.ProductName(c.Product) + ", " + m.routeName(c.Route)
	case "lie_low":
		return "lie low today"
	}
	return c.Setting
}

// commandName is what a refused command was for, as the review names
// it.
func (m *Model) commandName(c engine.Command) string {
	w := m.w
	switch {
	case c.City != "" && c.Product != "":
		return w.ProductName(c.Product) + ", " + w.CityName(c.City)
	case c.Route != "":
		return m.routeName(c.Route)
	}
	return c.Op
}

// changeEstimate is a change's estimate, `~` before it: a standing
// order's take and heat a night by orderEstimate (as a share where the
// order stands before and after, in money and heat where it comes or
// goes), a
// contract's buy tomorrow morning by the market's plan, and the orders
// lying low drops tonight.
func (m *Model) changeEstimate(c engine.Change) string {
	switch c.Setting {
	case "standing":
		var takeA, takeB int
		var heatA, heatB float64
		if c.Was != nil {
			_, takeA, heatA = m.orderEstimate(*c.Was, true)
		}
		if c.Now != nil {
			_, takeB, heatB = m.orderEstimate(*c.Now, true)
		}
		if c.Was != nil && c.Now != nil && takeA > 0 && heatA > 0 {
			return "~take " + signedPct(float64(takeB-takeA)/float64(takeA)*100) + ", heat " + signedPct((heatB-heatA)/heatA*100)
		}
		return fmt.Sprintf("~take %s, heat %+.1f", signedCash(takeB-takeA), heatB-heatA)
	case "supply":
		if c.Cost == c.CostTo {
			return ""
		}
		return "~" + signedCash(c.CostTo-c.Cost) + " a morning"
	case "lie_low":
		if c.Dropped > 0 {
			return "drops " + plural(c.Dropped, "order") + " tonight"
		}
	}
	return ""
}

// signedPct is a change in percent with its sign: +12%, -40%.
func signedPct(f float64) string {
	if f >= 0 {
		return "+" + pctText(f)
	}
	return pctText(f)
}

// signedCash is a change in cash with its sign: +$1.2K, -$300.
func signedCash(n int) string {
	if n < 0 {
		return "-" + cash(-n)
	}
	return "+" + cash(n)
}
