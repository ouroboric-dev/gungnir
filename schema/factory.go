package schema

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/ouroboric-dev/gungnir"
)

// Factory is factory.yaml.
type Factory struct {
	Mode    gungnir.Mode            `yaml:"mode"`
	Gates   map[gungnir.Gate]string `yaml:"gates"`
	Roles   map[string]Role         `yaml:"roles"`
	Budget  Budget                  `yaml:"budget"`
	Privacy string                  `yaml:"privacy"`
}

// Role maps a role to an agent adapter and a model (spec §5).
type Role struct {
	Agent  string `yaml:"agent"`
	Model  string `yaml:"model"`
	Script string `yaml:"script"` // scripted adapter only: the script, relative to factory.yaml
}

// Budget holds the caps. Both are required, and the factory ships no default values.
type Budget struct {
	PerRun Cap `yaml:"per_run"`
	PerDay Cap `yaml:"per_day"`
}

// Cap limits tokens and turns, and dollars when a role bills a paid API.
type Cap struct {
	Tokens int64   `yaml:"tokens"`
	Turns  int64   `yaml:"turns"`
	USD    float64 `yaml:"usd"`
}

// PaidAPI reports whether an adapter bills a paid API. Only adapters known to run on a
// subscription or at no cost return false, so a new adapter needs a spend limit until it is listed.
func PaidAPI(agent string) bool { return agent != "claude-code" && agent != "scripted" }

// CapsError lists the cap fields that factory.yaml lacks.
type CapsError struct {
	File    string
	Missing []string
	Paid    bool // a role bills a paid API, so the caps need usd too
}

func (e *CapsError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s has no complete budget caps, so nothing runs. Missing:\n", e.File)
	for _, m := range e.Missing {
		fmt.Fprintf(&b, "  - %s\n", m)
	}
	usd := ""
	if e.Paid {
		usd = ", usd: <dollars>"
	}
	b.WriteString("Set your own limits in it; the factory ships no defaults:\n  budget:\n")
	fmt.Fprintf(&b, "    per_run: { tokens: <tokens>, turns: <turns>%s }\n", usd)
	fmt.Fprintf(&b, "    per_day: { tokens: <tokens>, turns: <turns>%s }\n", usd)
	return b.String()
}

// LoadFactory reads and parses a factory.yaml file.
func LoadFactory(path string) (*Factory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseFactory(path, data)
}

// ParseFactory parses factory.yaml. Incomplete caps give a *CapsError before any other check, so
// the operator learns what to set. The gate overrides are checked against the file's own mode.
func ParseFactory(file string, data []byte) (*Factory, error) {
	doc, err := decodeYAML(file, data)
	if err != nil {
		return nil, err
	}
	if err := checkCaps(file, doc); err != nil {
		return nil, err
	}
	if err := validate(file, "factory.schema.json", doc); err != nil {
		return nil, err
	}
	var f Factory
	if err := decodeStrict(file, data, &f); err != nil {
		return nil, err
	}
	if f.Mode == "" {
		f.Mode = gungnir.ModeKeyPoints
	}
	if f.Privacy == "" {
		f.Privacy = "standard"
	}
	return &f, nil
}

func checkCaps(file string, doc any) error {
	root, _ := doc.(map[string]any)
	roles, _ := root["roles"].(map[string]any)
	paid := ""
	for _, name := range sortedKeys(roles) {
		role, _ := roles[name].(map[string]any)
		if agent, ok := role["agent"].(string); ok && PaidAPI(agent) {
			paid = fmt.Sprintf("role %s runs on %s, which bills a paid API", name, agent)
			break
		}
	}
	budget, _ := root["budget"].(map[string]any)
	var missing []string
	for _, c := range []string{"per_run", "per_day"} {
		cp, ok := budget[c].(map[string]any)
		if !ok {
			missing = append(missing, "budget."+c)
			continue
		}
		for _, field := range []string{"tokens", "turns"} {
			if cp[field] == nil {
				missing = append(missing, "budget."+c+"."+field)
			}
		}
		if paid != "" && cp["usd"] == nil {
			missing = append(missing, "budget."+c+".usd ("+paid+")")
		}
	}
	if len(missing) > 0 {
		return &CapsError{File: file, Missing: missing, Paid: paid != ""}
	}
	return nil
}

func sortedKeys[K ~string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
