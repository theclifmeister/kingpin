package news

import (
	"regexp"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
)

// The article agrees with the value (#148): every template rendered
// with a vowel in every slot writes `an` before it, and a consonant
// keeps `a`. headlines.toml:35 wrote `a {{.City}}` and Eastside read
// `a Eastside`.
func TestArticlesAgreeWithTheValue(t *testing.T) {
	cfg := content.MustLoad()
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	vowel := data{City: "Eastside", Product: "Acid", Name: "Auntie Mai", Role: "accountant", Corner: "Old Mill", Rival: "Ash", Front: "Arcade", Mode: "airship", From: "Eastside", To: "Ashport", Deal: "alliance", Stance: "anti-crime", Level: content.Arrest}
	consonant := data{City: "Bayport", Product: "Weed", Name: "Cass", Role: "runner", Corner: "Rail Yard", Rival: "Mother", Front: "Laundromat", Mode: "car", From: "Bayport", To: "Bayport", Deal: "truce", Stance: "reform", Level: content.Sting}
	wrongA := regexp.MustCompile(`\b[Aa] [AEIOU]`)
	wrongAn := regexp.MustCompile(`\b[Aa]n [BCDFGHJKLMNPQRSTVWXYZ]`)
	rendered := 0
	for key, list := range s.tmpl {
		for i, tm := range list {
			if got := render(tm, vowel); wrongA.MatchString(got) {
				t.Errorf("%s[%d]: %q writes `a` before a vowel", key, i, got)
			}
			if got := render(tm, consonant); wrongAn.MatchString(got) {
				t.Errorf("%s[%d]: %q writes `an` before a consonant", key, i, got)
			}
			rendered++
		}
	}
	if rendered == 0 {
		t.Fatal("no templates rendered")
	}
	for _, src := range cfg.Headlines.Templates["UnlockedProduct"] {
		if strings.Contains(src, "a {{.City}}") {
			got := render(s.tmpl["UnlockedProduct"][2], vowel)
			if !strings.Contains(got, "an Eastside outfit") {
				t.Fatalf("headlines.toml's `a {{.City}}` renders %q", got)
			}
		}
	}
}

// article rewrites the article before a field and nothing else.
func TestArticleRewrite(t *testing.T) {
	for _, c := range [][2]string{
		{"Rumour: a {{.City}} outfit has a line on {{.Product}}", "Rumour: {{a .City}} outfit has a line on {{.Product}}"},
		{"An {{.Product}} order went unfilled; a unit short", "{{A .Product}} order went unfilled; a unit short"},
		{"Pizza {{.City}} and Ana {{.Name}}", "Pizza {{.City}} and Ana {{.Name}}"},
	} {
		if got := article(c[0]); got != c[1] {
			t.Errorf("article(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}
