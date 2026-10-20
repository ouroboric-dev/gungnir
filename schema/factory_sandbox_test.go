package schema

import (
	"strings"
	"testing"
)

const realRoles = `
roles:
  implementer: { agent: claude-code, model: sonnet }
`

const sandboxBlock = `
sandbox:
  engine: unix:///mnt/wsl/podman-sockets/podman-machine-default/podman-user.sock
  mounts:
    - { host: /home/ana/ygg, engine: /mnt/wsl/ygg-bind }
`

func TestFactorySandboxSection(t *testing.T) {
	f, err := ParseFactory("factory.yaml", []byte(realRoles+fullCaps+sandboxBlock+"skills:\n  private_dir: /home/ana/skills\n"))
	if err != nil {
		t.Fatal(err)
	}
	if f.Sandbox == nil || f.Sandbox.Engine != "unix:///mnt/wsl/podman-sockets/podman-machine-default/podman-user.sock" ||
		len(f.Sandbox.Mounts) != 1 || f.Sandbox.Mounts[0].Engine != "/mnt/wsl/ygg-bind" || f.Skills.PrivateDir != "/home/ana/skills" {
		t.Errorf("the sandbox and skills sections were not read: %+v %+v", f.Sandbox, f.Skills)
	}
	// Scripted roles need no sandbox.
	if f, err := ParseFactory("factory.yaml", []byte(scriptedRoles+fullCaps)); err != nil || f.Sandbox != nil {
		t.Errorf("a scripted-only config: %v, sandbox %v", err, f.Sandbox)
	}
}

func TestFactorySandboxRefusals(t *testing.T) {
	cases := map[string]struct{ yaml, want string }{
		"a real agent without a sandbox": {realRoles + fullCaps, "role implementer runs on claude-code, which runs only in a sandbox"},
		"an engine that is not a socket": {realRoles + fullCaps + "sandbox:\n  engine: tcp://10.0.0.1:2375\n", "engine"},
		"a relative mount":               {realRoles + fullCaps + "sandbox:\n  engine: unix:///run/p.sock\n  mounts: [ { host: ygg, engine: /mnt/wsl/b } ]\n", "absolute"},
		"the root as a mount":            {realRoles + fullCaps + "sandbox:\n  engine: unix:///run/p.sock\n  mounts: [ { host: /, engine: /mnt/wsl/b } ]\n", "root"},
		"a host listed twice":            {realRoles + fullCaps + "sandbox:\n  engine: unix:///run/p.sock\n  mounts: [ { host: /a, engine: /x }, { host: /a, engine: /y } ]\n", "twice"},
		"an unknown sandbox key":         {realRoles + fullCaps + "sandbox:\n  engine: unix:///run/p.sock\n  privileged: true\n", "privileged"},
		"a relative skills dir":          {scriptedRoles + fullCaps + "skills:\n  private_dir: skills\n", "private_dir"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseFactory("factory.yaml", []byte(c.yaml))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("ParseFactory() = %v, want an error naming %q", err, c.want)
			}
		})
	}
}

func TestRolePrice(t *testing.T) {
	roles := "roles:\n  reviewer: { agent: opencode, model: openrouter/x/y, price: { input: 0.14, output: 0.28, cache_read: 0.014 } }\n"
	caps := "budget:\n  per_run: { tokens: 1, turns: 1, usd: 1 }\n  per_day: { tokens: 1, turns: 1, usd: 1 }\n"
	f, err := ParseFactory("factory.yaml", []byte(roles+caps+sandboxBlock))
	if err != nil || f.Roles["reviewer"].Price == nil || f.Roles["reviewer"].Price.Output != 0.28 {
		t.Fatalf("ParseFactory() = %+v, %v", f, err)
	}
	for name, p := range map[string]string{
		"a negative price": "{ input: -1, output: 1 }",
		"no output price":  "{ input: 1 }",
		"an unknown price": "{ input: 1, output: 1, discount: 2 }",
	} {
		bad := "roles:\n  reviewer: { agent: opencode, model: openrouter/x/y, price: " + p + " }\n"
		if _, err := ParseFactory("factory.yaml", []byte(bad+caps+sandboxBlock)); err == nil {
			t.Errorf("%s: ParseFactory() = nil, want an error", name)
		}
	}
}

func TestRoleReasoning(t *testing.T) {
	caps := "budget:\n  per_run: { tokens: 1, turns: 1, usd: 1 }\n  per_day: { tokens: 1, turns: 1, usd: 1 }\n"
	load := func(setting string) (*Factory, error) {
		roles := "roles:\n  reviewer: { agent: opencode, model: openrouter/x/y" + setting + " }\n"
		return ParseFactory("factory.yaml", []byte(roles+caps+sandboxBlock))
	}
	for _, v := range []string{"low", "medium", "high", "off"} {
		f, err := load(", reasoning: " + v)
		if err != nil || f.Roles["reviewer"].Reasoning != v {
			t.Errorf("reasoning %s: ParseFactory() = %+v, %v", v, f, err)
		}
	}
	f, err := load("")
	if err != nil || f.Roles["reviewer"].Reasoning != "" {
		t.Errorf("unset: ParseFactory() = %+v, %v, want an empty Reasoning", f, err)
	}
	// Every agent type accepts the setting, as it accepts price.
	for _, role := range []string{"{ agent: claude-code, model: m, reasoning: low }", "{ agent: scripted, model: m, script: s.sh, reasoning: off }"} {
		if _, err := ParseFactory("factory.yaml", []byte("roles:\n  r: "+role+"\n"+caps+sandboxBlock)); err != nil {
			t.Errorf("%s: ParseFactory() = %v", role, err)
		}
	}
	for name, v := range map[string]string{"max": "max", "an empty string": `""`, "a number": "3", "a boolean": "true", "a wrong case": "High"} {
		_, err := load(", reasoning: " + v)
		if err == nil || !strings.Contains(err.Error(), "reasoning") {
			t.Errorf("%s: ParseFactory() = %v, want an error naming reasoning", name, err)
		}
	}
	_, err = load(", reasoning: max")
	for _, want := range []string{"low", "medium", "high", "off"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error for max = %v, want it to list %q", err, want)
		}
	}
}
