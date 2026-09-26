package news

import (
	"reflect"
	"strings"
	"testing"
	"text/template"

	"github.com/theclifmeister/kingpin/internal/content"
)

// No headline renders with an empty field (#523: "Arrest on : 'the boss
// has a lawyer…'" for a lab raid, which names no corner). Every
// template, the flavour and the boss's swagger, is rendered against a
// fixture with each field it names missing in turn: the line falls back
// to a template of the same key that names only what is there, or is not
// written, and what is written never reads the gap.
func TestNoHeadlineRendersAnEmptyField(t *testing.T) {
	s, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	full := data{}
	v := reflect.ValueOf(&full).Elem()
	for i := 0; i < v.NumField(); i++ {
		switch f := v.Field(i); f.Kind() {
		case reflect.String:
			f.SetString("X" + v.Type().Field(i).Name)
		case reflect.Int:
			f.SetInt(3)
		}
	}
	lists := map[string][]*template.Template{"flavour": s.flav, "swagger": s.swag}
	for key, list := range s.tmpl {
		lists[key] = list
	}
	checked := 0
	for key, list := range lists {
		for i, tm := range list {
			for _, name := range fieldsOf(tm) {
				d := full
				f := reflect.ValueOf(&d).Elem().FieldByName(name)
				if !f.IsValid() || f.Kind() != reflect.String {
					continue
				}
				f.SetString("")
				checked++
				got := fits(list, i, d)
				if got == nil {
					continue // the line is skipped
				}
				for _, n := range fieldsOf(got) {
					if n == name {
						t.Errorf("%s#%d without %s fell back on %s, which names it", key, i, name, got.Name())
					}
				}
				txt := render(got, d)
				for _, gap := range []string{"  ", " :", " ,", " ;", " .", "''", "()"} {
					if strings.Contains(txt, gap) {
						t.Errorf("%s#%d without %s reads %q", key, i, name, txt)
					}
				}
				if strings.TrimSpace(txt) != txt || txt == "" {
					t.Errorf("%s#%d without %s reads %q", key, i, name, txt)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no template names a field; the test holds nothing")
	}
	// The case the testers read: an arrest with no corner.
	d := full
	d.Corner = ""
	for i := range s.tmpl["CrewArrested"] {
		if got := render(fits(s.tmpl["CrewArrested"], i, d), d); strings.Contains(got, "on :") || strings.Contains(got, "Arrest on") {
			t.Errorf("an arrest with no corner reads %q", got)
		}
	}
}
