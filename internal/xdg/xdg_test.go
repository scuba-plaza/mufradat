package xdg

import (
	"path/filepath"
	"testing"
)

func TestDefaultsUnderHome(t *testing.T) {
	d := New(func(string) string { return "" }, "/home/u")
	if d.Config != "/home/u/.config/mufradat" || d.Data != "/home/u/.local/share/mufradat" ||
		d.Cache != "/home/u/.cache/mufradat" || d.State != "/home/u/.local/state/mufradat" {
		t.Errorf("unexpected dirs: %+v", d)
	}
}

func TestEnvironmentOverridesAreHonouredWhenAbsolute(t *testing.T) {
	env := map[string]string{"XDG_DATA_HOME": "/data", "XDG_CONFIG_HOME": "relative"}
	d := New(func(k string) string { return env[k] }, "/home/u")
	if d.Data != "/data/mufradat" {
		t.Errorf("data = %s", d.Data)
	}
	if d.Config != "/home/u/.config/mufradat" {
		t.Errorf("relative XDG values must be ignored, got %s", d.Config)
	}
	if got := d.DefaultWorkspace(); got != filepath.Join("/data/mufradat", "default") {
		t.Errorf("default workspace = %s", got)
	}
}
