package schema

import (
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
	for _, key := range []string{"model", "ctx-size", "c", "host", "port", "alias"} {
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
	for name, bad := range map[string]string{
		"relative weights": strings.Replace(good, "weights: /abs/weights", "weights: weights", 1),
		"relative state":   strings.Replace(good, "state: /abs/state", "state: state", 1),
		"unclean weights":  strings.Replace(good, "/abs/weights", "/abs/../weights", 1),
		"uppercase name":   strings.Replace(good, "name: ygg-models", "name: Ygg", 1),
		"bad network":      strings.Replace(good, "network: ygg-models", "network: -x", 1),
		"long name":        strings.Replace(good, "name: ygg-models", "name: a"+strings.Repeat("b", 63), 1),
		"port 0":           strings.Replace(good, "port: 8089", "port: 0", 1),
		"port 65536":       strings.Replace(good, "port: 8089", "port: 65536", 1),
		"host_port 0":      strings.Replace(good, "host_port: 8088", "host_port: 0", 1),
		"bad model name":   strings.Replace(good, "mtp-27b:", "MTP:", 1),
		"file escapes":     strings.Replace(good, "file: mtp.gguf", "file: ../mtp.gguf", 1),
		"file absolute":    strings.Replace(good, "file: mtp.gguf", "file: /mtp.gguf", 1),
		"empty weights":    strings.Replace(good, "weights: /abs/weights", `weights: ""`, 1),
		"empty state":      strings.Replace(good, "state: /abs/state", `state: ""`, 1),
		"host_port 65536":  strings.Replace(good, "host_port: 8088", "host_port: 65536", 1),
		"no models":        modelsHead + "models: {}\n",
	} {
		if bad == good {
			t.Fatalf("%s: the replacement changed nothing", name)
		}
		if _, err := ParseModels("models.yaml", []byte(bad)); err == nil {
			t.Errorf("%s: want a refusal", name)
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
