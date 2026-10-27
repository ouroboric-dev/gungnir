package schema

import (
	"math"
	"os"
	"strings"
	"testing"
)

const modelsHead = `server:
  name: ygg-models
  network: ygg-models
  port: 8089
  host_port: 8088
  weights: /abs/weights
  state: /abs/state
`

const twoModels = modelsHead + `models:
  qwen3.8-27b:
    file: model.gguf
    context: 102400
    output: 32768
    args:
      reasoning-effort: low
      n-gpu-layers: "999"
    price: { input: 0.10, output: 0.40 }
  mtp-27b:
    file: mtp.gguf
    context: 65536
    output: 8192
    args:
      n-gpu-layers: "99"
      flash-attn: "true"
    price: { input: 0.2, output: 0.8, cache_read: 0.05 }
`

func oneModel(extra string) string {
	return modelsHead + "models:\n  m:\n    file: m.gguf\n    context: 1000\n    output: 100\n" + extra
}

func TestModelsParse(t *testing.T) {
	l, err := ParseModels("models.yaml", []byte(twoModels))
	if err != nil {
		t.Fatalf("ParseModels() error = %v", err)
	}
	m := l.Models["qwen3.8-27b"]
	if l.Server.Port != 8089 || l.Server.HostPort != 8088 || m.Context != 102400 || m.Output != 32768 || m.Price.Output != 0.40 || m.Args["reasoning-effort"] != "low" {
		t.Fatalf("ParseModels() = %+v", l)
	}
	if got := l.HostURL(); got != "http://127.0.0.1:8088" {
		t.Errorf("HostURL() = %q", got)
	}
}

func TestModelsNeedAPrice(t *testing.T) {
	for name, price := range map[string]string{
		"none":       "",
		"no input":   "    price: { output: 0.4 }\n",
		"no output":  "    price: { input: 0.1 }\n",
		"zero input": "    price: { input: 0, output: 0.4 }\n",
	} {
		_, err := ParseModels("models.yaml", []byte(oneModel(price)))
		if err == nil || !(strings.Contains(err.Error(), "model m") || strings.Contains(err.Error(), "/models/m")) || !strings.Contains(err.Error(), "price") {
			t.Errorf("%s: error = %v, want a refusal naming model m and price", name, err)
		}
	}
}

func TestModelsRefuseArgsThePresetWrites(t *testing.T) {
	for _, key := range []string{"model", "ctx-size", "c", "host", "port", "alias", "m", "a"} {
		yaml := oneModel("    args:\n      " + key + ": x\n    price: { input: 1, output: 1 }\n")
		_, err := ParseModels("models.yaml", []byte(yaml))
		if err == nil || !strings.Contains(err.Error(), `"`+key+`"`) {
			t.Errorf("key %s: error = %v, want a refusal naming the key", key, err)
		}
	}
}

func TestModelsRefuseBadArgs(t *testing.T) {
	const price = "    price: { input: 1, output: 1 }\n"
	// each case: the args block, and what the error must name
	for name, c := range map[string][2]string{
		"dash key":      {"      --n-gpu-layers: \"9\"\n", "--n-gpu-layers"},
		"short dash":    {"      -ngl: \"9\"\n", "-ngl"},
		"empty value":   {"      flash-attn: \"\"\n", "flash-attn"},
		"key equals":    {"      \"a=b\": x\n", "a=b"},
		"key bracket":   {"      \"a[b\": x\n", "a[b"},
		"key bracket 2": {"      \"a]b\": x\n", "a]b"},
		"key hash":      {"      \"a#b\": x\n", "a#b"},
		"key semicolon": {"      \"a;b\": x\n", "a;b"},
		"key space":     {"      \"a b\": x\n", "a b"},
		"newline":       {"      x: \"a\\nb = c\"\n", `"x"`},
		"return":        {"      x: \"a\\rb\"\n", `"x"`},
		"hash value":    {"      x: \"a # b\"\n", `"x"`},
		"semi value":    {"      x: \"a;b\"\n", `"x"`},
		"quote value":   {"      x: \"'a'\"\n", `"x"`},
		"lead space":    {"      x: \" true\"\n", `"x"`},
		"trail space":   {"      x: \"true \"\n", `"x"`},
		"not a string":  {"      n-gpu-layers: 999\n", "n-gpu-layers"},
	} {
		_, err := ParseModels("models.yaml", []byte(oneModel("    args:\n"+c[0]+price)))
		if err == nil || !strings.Contains(err.Error(), c[1]) {
			t.Errorf("%s: error = %v, want a refusal naming %s", name, err, c[1])
		}
	}
	for name, file := range map[string]string{"newline": `"a\nb"`, "return": `"a\rb"`, "hash": `"a.gguf # x"`, "semicolon": `"a;b"`, "quote": `"a'b"`, "edge space": `" a.gguf"`} {
		yaml := modelsHead + "models:\n  m:\n    file: " + file + "\n    context: 1000\n    output: 100\n" + price
		if _, err := ParseModels("models.yaml", []byte(yaml)); err == nil || !strings.Contains(err.Error(), "model m") || !strings.Contains(err.Error(), "file") {
			t.Errorf("file %s: error = %v, want a refusal naming model m and file", name, err)
		}
	}
}

func TestModelsRefuseABadServer(t *testing.T) {
	good := twoModels
	// each case: the bad document, and a word the error must hold
	for name, c := range map[string][2]string{
		"relative weights": {strings.Replace(good, "weights: /abs/weights", "weights: weights", 1), "weights"},
		"relative state":   {strings.Replace(good, "state: /abs/state", "state: state", 1), "state"},
		"unclean weights":  {strings.Replace(good, "/abs/weights", "/abs/../weights", 1), "weights"},
		"root weights":     {strings.Replace(good, "/abs/weights", "/", 1), "weights"},
		"root state":       {strings.Replace(good, "/abs/state", "/", 1), "state"},
		"uppercase name":   {strings.Replace(good, "name: ygg-models", "name: Ygg", 1), "name"},
		"dotted name":      {strings.Replace(good, "name: ygg-models", "name: models.example.com", 1), "name"},
		"trailing dot":     {strings.Replace(good, "name: ygg-models", "name: a.", 1), "name"},
		"underscore name":  {strings.Replace(good, "name: ygg-models", "name: a_b", 1), "name"},
		"dash first name":  {strings.Replace(good, "name: ygg-models", "name: -a", 1), "name"},
		"dash last name":   {strings.Replace(good, "name: ygg-models", "name: a-", 1), "name"},
		"newline name":     {strings.Replace(good, "name: ygg-models", `name: "a\n"`, 1), "name"},
		"bad network":      {strings.Replace(good, "network: ygg-models", "network: -x", 1), "network"},
		"long name":        {strings.Replace(good, "name: ygg-models", "name: a"+strings.Repeat("b", 63), 1), "name"},
		"port 0":           {strings.Replace(good, "port: 8089", "port: 0", 1), "port"},
		"port -1":          {strings.Replace(good, "port: 8089", "port: -1", 1), "port"},
		"port 65536":       {strings.Replace(good, "port: 8089", "port: 65536", 1), "port"},
		"port 8080":        {strings.Replace(good, "port: 8089", "port: 8080", 1), "8080"},
		"host_port 0":      {strings.Replace(good, "host_port: 8088", "host_port: 0", 1), "host_port"},
		"host_port 65536":  {strings.Replace(good, "host_port: 8088", "host_port: 65536", 1), "host_port"},
		"bad model name":   {strings.Replace(good, "mtp-27b:", "MTP:", 1), "MTP"},
		"file escapes":     {strings.Replace(good, "file: mtp.gguf", "file: ../mtp.gguf", 1), "file"},
		"file absolute":    {strings.Replace(good, "file: mtp.gguf", "file: /mtp.gguf", 1), "file"},
		"empty weights":    {strings.Replace(good, "weights: /abs/weights", `weights: ""`, 1), "weights"},
		"empty state":      {strings.Replace(good, "state: /abs/state", `state: ""`, 1), "state"},
		"no models":        {modelsHead + "models: {}\n", "models"},
	} {
		if c[0] == good {
			t.Fatalf("%s: the replacement changed nothing", name)
		}
		if _, err := ParseModels("models.yaml", []byte(c[0])); err == nil || !strings.Contains(err.Error(), c[1]) {
			t.Errorf("%s: error = %v, want a refusal naming %s", name, err, c[1])
		}
	}
	for _, port := range []string{"1", "65535"} {
		doc := strings.Replace(strings.Replace(good, "port: 8089", "port: "+port, 1), "host_port: 8088", "host_port: "+port, 1)
		if _, err := ParseModels("models.yaml", []byte(doc)); err != nil {
			t.Errorf("port %s: %v", port, err)
		}
	}
}

func TestModelsPresetMatchesTheGoldenFile(t *testing.T) {
	l, err := ParseModels("models.yaml", []byte(twoModels))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/models/preset.ini")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(l.Preset()); got != string(want) {
		t.Errorf("Preset() =\n%s\nwant\n%s", got, want)
	}
}

const dlPrice = "    price: { input: 1, output: 1 }\n"

func TestModelsDownloadParses(t *testing.T) {
	for name, block := range map[string]string{
		"with revision":    "    download:\n      repo: unsloth/Qwen3.8-27B-GGUF\n      revision: abc123def\n      files: [MTP/mtp-Qwen3.8-27B-Q4_0.gguf, mmproj.gguf]\n",
		"without revision": "    download:\n      repo: unsloth/Qwen3.8-27B-GGUF\n      files: [m.gguf]\n",
	} {
		l, err := ParseModels("models.yaml", []byte(oneModel(block+dlPrice)))
		if err != nil {
			t.Fatalf("%s: ParseModels() error = %v", name, err)
		}
		d := l.Models["m"].Download
		if d == nil || d.Repo != "unsloth/Qwen3.8-27B-GGUF" || len(d.Files) == 0 {
			t.Errorf("%s: Download = %+v", name, d)
		}
	}
	l, err := ParseModels("models.yaml", []byte(oneModel(dlPrice)))
	if err != nil || l.Models["m"].Download != nil {
		t.Errorf("no download: %v, %+v", err, l)
	}
}

func TestModelsDownloadRefuses(t *testing.T) {
	for name, c := range map[string]struct{ block, field string }{
		"bad repo":        {"repo: nofolder\n      files: [m.gguf]\n", "repo"},
		"repo with space": {"repo: a/b c\n      files: [m.gguf]\n", "repo"},
		"bad revision":    {"repo: a/b\n      revision: \"x y\"\n      files: [m.gguf]\n", "revision"},
		"long revision":   {"repo: a/b\n      revision: " + strings.Repeat("a", 65) + "\n      files: [m.gguf]\n", "revision"},
		"empty files":     {"repo: a/b\n      files: []\n", "files"},
		"absolute":        {"repo: a/b\n      files: [/etc/m.gguf]\n", "files"},
		"dotdot":          {"repo: a/b\n      files: [../m.gguf]\n", "files"},
		"unclean":         {"repo: a/b\n      files: [a//m.gguf]\n", "files"},
		"space":           {"repo: a/b\n      files: [\"m 1.gguf\"]\n", "files"},
		"dollar":          {"repo: a/b\n      files: [\"$m.gguf\"]\n", "files"},
		"dash file":       {"repo: a/b\n      files: [-a.gguf]\n", "files"},
		"dot":             {"repo: a/b\n      files: [.]\n", "files"},
		"dots":            {"repo: a/b\n      files: [\"...\"]\n", "files"},
		"dot slash":       {"repo: a/b\n      files: [./a]\n", "files"},
		"mid dot":         {"repo: a/b\n      files: [a/./b]\n", "files"},
		"hidden":          {"repo: a/b\n      files: [a/.hidden]\n", "files"},
		"dash repo":       {"repo: -x/y\n      files: [m.gguf]\n", "repo"},
		"dot repo":        {"repo: ./y\n      files: [m.gguf]\n", "repo"},
		"dotdot repo":     {"repo: ../x\n      files: [m.gguf]\n", "repo"},
		"hidden repo":     {"repo: .x/y\n      files: [m.gguf]\n", "repo"},
		"dash revision":   {"repo: a/b\n      revision: -x\n      files: [m.gguf]\n", "revision"},
		"dot revision":    {"repo: a/b\n      revision: .x\n      files: [m.gguf]\n", "revision"},
		"duplicate":       {"repo: a/b\n      files: [m.gguf, m.gguf]\n", "files"},
	} {
		_, err := ParseModels("models.yaml", []byte(oneModel("    download:\n      "+c.block+dlPrice)))
		if err == nil {
			t.Errorf("%s: no error", name)
			continue
		}
		msg := err.Error()
		if !(strings.Contains(msg, "model m") || strings.Contains(msg, "/models/m")) || !strings.Contains(msg, c.field) {
			t.Errorf("%s: error = %v, want a refusal naming model m and %s", name, err, c.field)
		}
	}
}

func TestModelsDownloadNeverReachesThePreset(t *testing.T) {
	with := oneModel("    download:\n      repo: a/b\n      files: [m.gguf]\n" + dlPrice)
	a, err := ParseModels("models.yaml", []byte(with))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ParseModels("models.yaml", []byte(oneModel(dlPrice)))
	if string(a.Preset()) != string(b.Preset()) {
		t.Errorf("preset changed with download:\n%s", a.Preset())
	}
}

// The schema refuses these first through ParseModels, so the Go check is tested on its own.
func TestModelsDownloadGoCheckRefusesOnItsOwn(t *testing.T) {
	for name, d := range map[string]Download{
		"dash file":    {Repo: "a/b", Files: []string{"-a.gguf"}},
		"dot":          {Repo: "a/b", Files: []string{"."}},
		"dots":         {Repo: "a/b", Files: []string{"..."}},
		"dot slash":    {Repo: "a/b", Files: []string{"./a"}},
		"mid dot":      {Repo: "a/b", Files: []string{"a/./b"}},
		"hidden":       {Repo: "a/b", Files: []string{"a/.hidden"}},
		"double slash": {Repo: "a/b", Files: []string{"a//b"}},
		"dotdot":       {Repo: "a/b", Files: []string{".."}},
		"dash repo":    {Repo: "-x/y", Files: []string{"m"}},
		"dotdot repo":  {Repo: "../x", Files: []string{"m"}},
		"hidden repo":  {Repo: ".x/y", Files: []string{"m"}},
		"dash rev":     {Repo: "a/b", Revision: "-x", Files: []string{"m"}},
		"duplicate":    {Repo: "a/b", Files: []string{"m", "m"}},
		"no files":     {Repo: "a/b"},
	} {
		if err := d.check("m"); err == nil || !strings.Contains(err.Error(), "model m") {
			t.Errorf("%s: error = %v", name, err)
		}
	}
	if err := (Download{Repo: "unsloth/Qwen3.8-27B-GGUF", Files: []string{"MTP/mtp-Qwen3.8-27B-Q4_0.gguf"}}).check("m"); err != nil {
		t.Errorf("valid: %v", err)
	}
}

// YAML reads .nan, .inf and -.inf as floats. The YAML-to-JSON step refuses them before the schema runs, so the error
// must name the field, not blame a key.
func TestModelsRefuseAPriceThatIsNotFinite(t *testing.T) {
	for _, bad := range []string{".nan", ".inf", "-.inf"} {
		for name, price := range map[string]string{
			"price.input":       "{ input: " + bad + ", output: 0.4 }",
			"price.output":      "{ input: 0.1, output: " + bad + " }",
			"price.cache_read":  "{ input: 0.1, output: 0.4, cache_read: " + bad + " }",
			"price.cache_write": "{ input: 0.1, output: 0.4, cache_write: " + bad + " }",
		} {
			_, err := ParseModels("models.yaml", []byte(oneModel("    price: "+price+"\n")))
			if err == nil || !strings.Contains(err.Error(), name) || strings.Contains(err.Error(), "key") {
				t.Errorf("%s = %s: error = %v, want a refusal naming %s", name, bad, err, name)
			}
		}
	}
}

// The schema refuses a negative cache price first; the Go check holds the same line on its own.
func TestModelsRefuseANegativeCachePrice(t *testing.T) {
	for _, field := range []string{"cache_read", "cache_write"} {
		_, err := ParseModels("models.yaml", []byte(oneModel("    price: { input: 0.1, output: 0.4, "+field+": -1 }\n")))
		if err == nil || !strings.Contains(err.Error(), "/price/"+field) {
			t.Errorf("%s = -1: error = %v, want a refusal naming %s", field, err, field)
		}
	}
	if _, err := ParseModels("models.yaml", []byte(oneModel("    price: { input: 0.1, output: 0.4, cache_read: 0, cache_write: 0 }\n"))); err != nil {
		t.Errorf("a cache price of 0: error = %v, want none", err)
	}
}

// The Go check is the second line, so it is tested on its own with the values the schema step cannot pass on.
func TestModelsPriceGoCheckRefusesOnItsOwn(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	for name, p := range map[string]Price{
		"nan input":   {Input: nan, Output: 1},
		"inf input":   {Input: inf, Output: 1},
		"-inf input":  {Input: -inf, Output: 1},
		"nan output":  {Input: 1, Output: nan},
		"inf output":  {Input: 1, Output: inf},
		"zero output": {Input: 1},
		"nan read":    {Input: 1, Output: 1, CacheRead: nan},
		"inf read":    {Input: 1, Output: 1, CacheRead: inf},
		"neg read":    {Input: 1, Output: 1, CacheRead: -1},
		"nan write":   {Input: 1, Output: 1, CacheWrite: nan},
		"-inf write":  {Input: 1, Output: 1, CacheWrite: -inf},
		"neg write":   {Input: 1, Output: 1, CacheWrite: -0.5},
	} {
		err := LocalModelConfig{File: "m.gguf", Price: p}.check("m")
		if err == nil || !strings.Contains(err.Error(), "model m") || !strings.Contains(err.Error(), "price") {
			t.Errorf("%s: error = %v, want a refusal naming model m and price", name, err)
		}
	}
	if err := (LocalModelConfig{File: "m.gguf", Price: Price{Input: 1, Output: 1, CacheRead: 0, CacheWrite: 0.5}}).check("m"); err != nil {
		t.Errorf("valid: %v", err)
	}
}
