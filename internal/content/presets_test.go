package content

import "testing"

// presets.toml (#357): the four built-ins the issue names, and a row
// that names no dial, a word that is not one, or nothing at all, is
// refused.
func TestPresetsFile(t *testing.T) {
	c := MustLoad()
	var ids []string
	for _, p := range c.Presets.Presets {
		ids = append(ids, p.ID)
	}
	if len(ids) != 4 || ids[0] != "quiet" || ids[3] != "dark" {
		t.Errorf("the built-ins are %v", ids)
	}
	for name, bad := range map[string]PresetConfig{
		"standing": {Standing: "loud"},
		"supply":   {Supply: "pause"},
		"launder":  {Launder: "reckless"},
		"pay":      {Pay: "cheap"},
		"routes":   {Routes: "careful"},
		"nothing":  {},
	} {
		bad.ID, bad.Name, bad.Blurb = "x", "X", "x"
		if err := (PresetsConfig{Presets: []PresetConfig{bad}}).validate(); err == nil {
			t.Errorf("a preset with a bad %s passed", name)
		}
	}
	twice := PresetsConfig{Presets: []PresetConfig{c.Presets.Presets[0], c.Presets.Presets[0]}}
	if twice.validate() == nil {
		t.Error("a preset defined twice passed")
	}
}
