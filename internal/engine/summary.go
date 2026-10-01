package engine

import (
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The run summary's two texts (#49, docs/endings.md): the epilogue and
// the story. The TUI's summary drew them off the world itself; since
// #554 they are the session's, so the summary and Street Edition's
// ending screen (view 18's over.epilogue and over.story) tell one run
// the one way.

// Epilogue is the ending's epilogue from endings.toml, filled from the
// world as it stood; "" with no run or no ending, and the cause in caps
// for one whose template will not render (the file is validated at
// load, so that is a field a template names that the data lacks).
func (s *Session) Epilogue() string {
	w := s.w
	if w == nil || w.Over == nil {
		return ""
	}
	e := w.Over
	home := w.Home()
	fronts := plural(len(w.Fronts), "business")
	if len(w.Fronts) != 1 {
		fronts = fmt.Sprintf("%d businesses", len(w.Fronts))
	}
	d := content.Epilogue{
		Days:     e.Day,
		City:     home.Name,
		Here:     w.Here().Name,
		Offshore: format.Money(w.Offshore),
		Left:     format.Money(w.Cash()),
		Bodies:   w.Stats.Bodies,
		Fronts:   fronts,
		Corners:  w.Held(),
		Name:     e.Who,
		Leader:   e.Who,
		DA:       w.Law.DA.Name,
		Chief:    w.Law.Chief.Name,
		Pages:    max(1, w.Heat.Evidence),
		Years:    plural(s.cfg.Endings.Kingpin.Reign(home.Heat, home.Pressure), "year"),
		Reign:    plural(max(1, w.ReignDay()), "day"),
		Hot:      home.Heat > home.Pressure,
	}
	if e.Who == "" {
		d.Name, d.Leader = "Somebody", w.Rival().Leader
		if d.Leader == "" {
			d.Leader = "The rival"
		}
	}
	text, err := s.cfg.Endings.Render(e.Cause, d)
	if err != nil {
		return strings.ToUpper(e.Cause)
	}
	return text
}

// Story is the summary's timeline: [summary] lines headlines of the
// run by the weight of their source, in the order they happened. A
// text is told once, at its latest (#465: five identical patrol lines
// were the story), and the run is cut into as many spans as there are
// lines, each span giving its heaviest headline, the latest among
// equals, so a 563-day run is not told from its last month alone; a
// span with nothing to tell leaves its line to the heaviest of the
// rest. Nil with no run.
func (s *Session) Story() []game.Headline {
	w := s.w
	if w == nil {
		return nil
	}
	cfg := s.cfg.Endings.Summary
	type entry struct {
		h      game.Headline
		weight float64
		i      int
	}
	quiet := s.quietLines()
	last := map[string]int{} // a text's latest entry in the journal
	for i, h := range w.Journal {
		last[h.Text] = i
	}
	var cands []entry
	for i, h := range w.Journal {
		if quiet[h.Text] || last[h.Text] != i {
			continue
		}
		if wt := cfg.Weight[h.Source]; wt > 0 {
			cands = append(cands, entry{h, wt, i})
		}
	}
	heavier := func(a, b entry) bool {
		if a.weight != b.weight {
			return a.weight > b.weight
		}
		return a.i > b.i
	}
	sort.SliceStable(cands, func(a, b int) bool { return heavier(cands[a], cands[b]) })
	days := 1
	if w.Over != nil {
		days = max(days, w.Over.Day)
	}
	for _, c := range cands {
		days = max(days, c.h.Day)
	}
	var picked []entry
	taken := map[int]bool{}
	if n := cfg.Lines; n > 0 {
		for span := 0; span < n; span++ {
			for _, c := range cands {
				if sp := min(n-1, (c.h.Day-1)*n/days); sp == span && !taken[c.i] {
					picked, taken[c.i] = append(picked, c), true
					break
				}
			}
		}
		for _, c := range cands {
			if len(picked) >= n {
				break
			}
			if !taken[c.i] {
				picked, taken[c.i] = append(picked, c), true
			}
		}
	}
	sort.Slice(picked, func(a, b int) bool { return picked[a].i < picked[b].i })
	out := make([]game.Headline, 0, len(picked))
	for _, p := range picked {
		out = append(out, p.h)
	}
	return out
}

// quietLines are the texts the summary's quiet templates render to in
// every city of the run (#424): "Nothing to report from Eastside
// corners" is a heat line, and the story never tells it.
func (s *Session) quietLines() map[string]bool {
	out := map[string]bool{}
	for _, key := range s.cfg.Endings.Summary.Quiet {
		for _, src := range s.cfg.Headlines.Templates[key] {
			tpl, err := template.New(key).Parse(src)
			if err != nil {
				continue
			}
			for _, c := range s.w.Cities {
				var b strings.Builder
				if tpl.Execute(&b, map[string]string{"City": c.Name}) == nil {
					out[b.String()] = true
				}
			}
		}
	}
	return out
}
