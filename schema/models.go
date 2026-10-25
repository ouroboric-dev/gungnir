package schema

import (
	"fmt"
	"path/filepath"
	"regexp"
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
// m and a are the short forms the parser maps to model and alias.
var presetKeys = []string{"model", "m", "ctx-size", "c", "host", "port", "alias", "a"}

// argKey is a long option name, as llama-server writes it.
var argKey = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// gatePort is where llama-server listens in the same container, so the gate cannot take it.
const gatePort = 8080

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
	if !gungnir.ValidModelHost(s.Name) {
		return fmt.Errorf("server name %q must be one DNS label: 1 to 63 characters from a-z, 0-9 and '-', starting and ending with a letter or digit", s.Name)
	}
	if !gungnir.ValidModelNetwork(s.Network) {
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
	if s.Port == gatePort {
		return fmt.Errorf("server port %d is taken: llama-server listens on it in the same container", gatePort)
	}
	for _, d := range []struct{ what, path string }{{"weights", s.Weights}, {"state", s.State}} {
		if !filepath.IsAbs(d.path) || filepath.Clean(d.path) != d.path || d.path == "/" {
			return fmt.Errorf("server %s %q must be a clean, absolute path, and not the root", d.what, d.path)
		}
	}
	for _, name := range sortedKeys(l.Models) {
		if err := l.Models[name].check(name); err != nil {
			return err
		}
	}
	return nil
}

// A model name may have 64 characters, as local/<name> allows; the server name and network take 63, the engine's limit.
func (m LocalModelConfig) check(name string) error {
	if !localName.MatchString(name) {
		return fmt.Errorf("model %q: the name must be 1 to 64 characters from a-z, 0-9, '.', '_' and '-', starting with a letter or digit", name)
	}
	if m.Price.Input <= 0 || m.Price.Output <= 0 {
		return fmt.Errorf("model %s: price.input and price.output are required and above 0", name)
	}
	if !filepath.IsLocal(m.File) {
		return fmt.Errorf("model %s: file %q must be a relative path under weights", name, m.File)
	}
	if err := plainValue(m.File); err != nil {
		return fmt.Errorf("model %s: file %q %w", name, m.File, err)
	}
	for _, key := range sortedKeys(m.Args) {
		switch {
		case slices.Contains(presetKeys, key):
			return fmt.Errorf("model %s: args must not hold %q; the preset writes it", name, key)
		case !argKey.MatchString(key):
			return fmt.Errorf("model %s: args key %q must be a long option name matching %s", name, key, argKey)
		case m.Args[key] == "":
			return fmt.Errorf("model %s: args %q has an empty value; give \"true\" or \"false\" for a flag", name, key)
		}
		if err := plainValue(m.Args[key]); err != nil {
			return fmt.Errorf("model %s: args %q value %q %w", name, key, m.Args[key], err)
		}
	}
	return nil
}

// plainValue refuses a value the preset's INI parser could change or refuse: a line break, a comment
// character, a quote, or whitespace at either end.
func plainValue(v string) error {
	if strings.ContainsAny(v, "\r\n#;\"'") || strings.TrimSpace(v) != v {
		return fmt.Errorf("must hold one line, with no '#', ';' or quote and no space at either end")
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
