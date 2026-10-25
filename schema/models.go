package schema

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ouroboric-dev/gungnir"
)

// ModelList is the operator's list of local models (local-models spec §3.2).
type ModelList struct {
	Server ModelServerConfig           `yaml:"server"`
	Models map[string]LocalModelConfig `yaml:"models"`
}

// ModelServerConfig says where the shared model server runs and where its files are.
type ModelServerConfig struct {
	Name     string `yaml:"name"`
	Network  string `yaml:"network"`
	Port     int    `yaml:"port"`
	HostPort int    `yaml:"host_port"`
	Weights  string `yaml:"weights"`
	State    string `yaml:"state"`
}

// LocalModelConfig is one model of the list.
type LocalModelConfig struct {
	File    string            `yaml:"file"`
	Context int               `yaml:"context"`
	Output  int               `yaml:"output"`
	Args    map[string]string `yaml:"args"`
	Price   Price             `yaml:"price"`
}

// presetKeys are the keys the preset writes itself, so args must not hold them.
var presetKeys = []string{"model", "ctx-size", "c", "host", "port", "alias"}

// ParseModels parses the model list: the schema first, then the checks the schema cannot make.
func ParseModels(file string, data []byte) (*ModelList, error) {
	var l ModelList
	if err := parseYAML(file, "models.schema.json", data, &l); err != nil {
		return nil, err
	}
	if err := l.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &l, nil
}

func (l *ModelList) check() error {
	s := l.Server
	if !gungnir.ValidModelName(s.Name) {
		return fmt.Errorf("server name %q must be 1 to 63 characters from a-z, 0-9, '.', '_' and '-', starting with a letter or digit", s.Name)
	}
	if !gungnir.ValidModelName(s.Network) {
		return fmt.Errorf("server network %q must be 1 to 63 characters from a-z, 0-9, '.', '_' and '-', starting with a letter or digit", s.Network)
	}
	for _, p := range []struct {
		what string
		port int
	}{{"port", s.Port}, {"host_port", s.HostPort}} {
		if p.port < 1 || p.port > 65535 {
			return fmt.Errorf("server %s %d is outside 1 to 65535", p.what, p.port)
		}
	}
	for _, d := range []struct{ what, path string }{{"weights", s.Weights}, {"state", s.State}} {
		if !filepath.IsAbs(d.path) || filepath.Clean(d.path) != d.path {
			return fmt.Errorf("server %s %q must be a clean, absolute path", d.what, d.path)
		}
	}
	for _, name := range sortedKeys(l.Models) {
		if err := l.Models[name].check(name); err != nil {
			return err
		}
	}
	return nil
}

func (m LocalModelConfig) check(name string) error {
	if !localName.MatchString(name) {
		return fmt.Errorf("model %q: the name must be 1 to 64 characters from a-z, 0-9, '.', '_' and '-', starting with a letter or digit", name)
	}
	if m.Price.Input <= 0 || m.Price.Output <= 0 {
		return fmt.Errorf("model %s: price.input and price.output are required and above 0", name)
	}
	if !filepath.IsLocal(m.File) || strings.ContainsAny(m.File, "\r\n") {
		return fmt.Errorf("model %s: file %q must be a relative path under weights", name, m.File)
	}
	for _, key := range sortedKeys(m.Args) {
		switch {
		case slices.Contains(presetKeys, key):
			return fmt.Errorf("model %s: args must not hold %q; the preset writes it", name, key)
		case key == "" || strings.HasPrefix(key, "-"):
			return fmt.Errorf("model %s: args key %q must be a long option name without leading '-'", name, key)
		case strings.ContainsAny(key, "=[]#; \t\r\n"):
			return fmt.Errorf("model %s: args key %q must be a plain option name", name, key)
		case m.Args[key] == "":
			return fmt.Errorf("model %s: args %q has an empty value; give \"true\" or \"false\" for a flag", name, key)
		case strings.ContainsAny(m.Args[key], "\r\n"):
			return fmt.Errorf("model %s: args %q must hold one line", name, key)
		}
	}
	return nil
}

// Preset returns llama-server's preset INI (local-models spec §3.3): version = 1, then a section per model
// in sorted order, with model and ctx-size first and the args keys in sorted order.
func (l *ModelList) Preset() []byte {
	var b strings.Builder
	b.WriteString("version = 1\n")
	for _, name := range sortedKeys(l.Models) {
		m := l.Models[name]
		fmt.Fprintf(&b, "\n[%s]\nmodel = /models/%s\nctx-size = %d\n", name, m.File, m.Context)
		for _, key := range sortedKeys(m.Args) {
			fmt.Fprintf(&b, "%s = %s\n", key, m.Args[key])
		}
	}
	return []byte(b.String())
}

// HostURL is where the host reaches llama-server.
func (l *ModelList) HostURL() string {
	return "http://127.0.0.1:" + strconv.Itoa(l.Server.HostPort)
}
