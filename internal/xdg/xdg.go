package xdg

import (
	"path/filepath"
)

const appName = "mufradat"

type Dirs struct {
	Config string
	Data   string
	Cache  string
	State  string
}

func New(getenv func(string) string, home string) Dirs {
	pick := func(envVar, fallback string) string {
		if v := getenv(envVar); v != "" && filepath.IsAbs(v) {
			return filepath.Join(v, appName)
		}
		return filepath.Join(home, fallback, appName)
	}
	return Dirs{
		Config: pick("XDG_CONFIG_HOME", ".config"),
		Data:   pick("XDG_DATA_HOME", filepath.Join(".local", "share")),
		Cache:  pick("XDG_CACHE_HOME", ".cache"),
		State:  pick("XDG_STATE_HOME", filepath.Join(".local", "state")),
	}
}

func (d Dirs) UserConfigFile() string { return filepath.Join(d.Config, "config.toml") }

func (d Dirs) ImportedGoogleKey() string { return filepath.Join(d.Config, "google-credentials.json") }

func (d Dirs) ImportedMistralKey() string { return filepath.Join(d.Config, "mistral-api-key") }

func (d Dirs) DefaultWorkspace() string { return filepath.Join(d.Data, "default") }

func (d Dirs) PythonEnvs() string { return filepath.Join(d.Data, "pyenv") }

func (d Dirs) CamelData() string { return filepath.Join(d.Data, "camel_data") }

func (d Dirs) Sidecar() string { return filepath.Join(d.Data, "sidecar") }

func (d Dirs) ModelCache() string { return filepath.Join(d.Cache, "models") }

func (d Dirs) LogFile() string { return filepath.Join(d.State, "mufradat.log") }
