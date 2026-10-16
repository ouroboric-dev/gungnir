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
