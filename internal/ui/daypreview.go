package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The day's preview (#353) is the END THE DAY? modal's body: what is
// left idle and what else needs you first, then the cart in a sentence,
// then tonight's money as #351's cash flow, estimated (engine.Preview),
// and what the estimate cannot know. It reads the session and writes
// nothing.

// previewCols are the preview's money columns: the flow's, the first
// saying every figure under it is an estimate.
var previewCols = []col{{"tonight ~", kText, 0}, {"dirty", kMoney, 0}, {"clean", kMoney, 0}, {"total", kMoney, 0}}

// unknownWords are engine.PreviewUnknown as the preview words them.
var unknownWords = map[string]string{
	"robbery": "robberies",
	"police":  "the police",
	"prices":  "tomorrow's prices",
	"audit":   "audits",
	"skim":    "skims",
	"rivals":  "the rivals",
	"crew":    "the crew's nights",
}

// previewLines is the END THE DAY? modal's body.
func (m *Model) previewLines() []string {
	p := m.sess.Preview()
	if p == nil {
		return []string{m.endDayLine(), "The sims step and the run autosaves."}
	}
	var body []string
	// What needs you: the morning's alerts (the idle crew and the
	// corners nobody works among them), the selected one marked, the
	// one o opens (#352: the dashboard's ALERTS cursor, over the same
	// list).
	if len(p.Alerts) > 0 {
		clamp(&m.alertCursor, len(p.Alerts))
	}
	for i, a := range p.Alerts {
		cur := "  "
		if i == m.alertCursor {
			cur = theme.Gold.Render("▸ ")
		}
		for _, l := range m.wrapWhole(m.alertOf(a).text, m.modalInner()-2) {
			body = append(body, cur+l)
			cur = "  "
		}
	}
	if len(body) > 0 {
		body = append(body, "")
	}
	// The money: the cart, then the night by category.
	body = append(body, m.wrapLines(m.endDayLine())...)
	for _, s := range p.Sales {
		line := fmt.Sprintf("%s: ~%s sold", m.w.CityName(s.City), plural(s.Units, "unit"))
		if s.Delivered > 0 {
			line += fmt.Sprintf(", %d handed over", s.Delivered)
		}
		body = append(body, theme.Subtle.Render(line+fmt.Sprintf(", ~%s, %+.1f heat.", cash(s.Take), s.Heat)))
	}
	body = append(body, "")
	body = append(body, m.previewTable(p.Flow)...)
	var unknown []string
	for _, id := range p.Unknown {
		if w, ok := unknownWords[id]; ok {
			unknown = append(unknown, w)
		}
	}
	body = append(body, "")
	body = append(body, m.wrapLines(theme.Subtle.Render(fmt.Sprintf("Estimates before the dice: %s are not in them. The sims step and the run autosaves.", andList(unknown))))...)
	return body
}

// screenPointerWords is a pointer to a screen, `on the crew screen (4)`
// (screenPointer), which a wrap never breaks.
var screenPointerWords = regexp.MustCompile(`on the \w+ screen \(\d\)`)

// glue stands in for a pointer's spaces while its line wraps: one cell
// wide and no space to the wrap, so the pointer moves down whole.
const glue = "\ue000"

// wrapWhole wraps prose to width cells keeping each pointer to a
// screen on one line.
func (m *Model) wrapWhole(s string, width int) []string {
	s = screenPointerWords.ReplaceAllStringFunc(s, func(p string) string { return strings.ReplaceAll(p, " ", glue) })
	out := strings.Split(theme.Plain.Width(width).Render(s), "\n")
	for i := range out {
		out[i] = strings.ReplaceAll(out[i], glue, " ")
	}
	return out
}

// previewHasAlerts is the day's preview with an alert to pick and
// open: its [ ] and o are listed and live.
func previewHasAlerts(m *Model) bool { return len(m.sess.Alerts()) > 0 }

// andList is names joined `a, b and c`.
func andList(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// previewTable is tonight's money as the report's flow table draws a
// night (moneyLines): the piles now, a row a category the night is
// expected to move, and the projected closing, every figure an estimate.
func (m *Model) previewTable(f engine.FlowView) []string {
	ends := func(label string, p engine.PoolsView) []any {
		return []any{styled{theme.Gold, label}, styled{theme.Gold, p.Dirty}, styled{theme.Gold, p.Clean}, styled{theme.Gold, p.Dirty + p.Clean}}
	}
	rows := [][]any{ends("Now", f.Opening)}
	for _, l := range f.Lines {
		if l.Dirty == 0 && l.Clean == 0 {
			continue
		}
		row := []any{game.FlowLabel(l.Cat), flowCell(l.Dirty), flowCell(l.Clean), signed{l.Dirty + l.Clean}}
		if l.Big {
			st := theme.Good
			if l.Dirty+l.Clean < 0 {
				st = theme.Bad
			}
			for i := range row {
				row[i] = styled{st, row[i]}
			}
		}
		rows = append(rows, row)
	}
	rows = append(rows, ends("Closing", f.Closing))
	return table(previewCols, rows, -1, 0)
}
