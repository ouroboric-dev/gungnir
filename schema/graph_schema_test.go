package schema

import (
	"strings"
	"testing"
)

const schemaHead = "name: toy\nversion: 1.0.0\n"
const schemaTail = "nodes:\n  - { id: a, kind: agent, role: planner, prompt: p.md }\n"

func TestAGraphStatesItsSchemaVersion(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(schemaHead+"schema: 1\n"+schemaTail))
	if err != nil || g.Schema != 1 || GraphSchema != 1 {
		t.Fatalf("ParseGraph() = %+v, %v; want Schema 1", g, err)
	}
	for name, y := range map[string]string{
		"no schema":       schemaHead + schemaTail,
		"schema 0":        schemaHead + "schema: 0\n" + schemaTail,
		"a string schema": schemaHead + "schema: \"1\"\n" + schemaTail,
	} {
		_, err := ParseGraph("graph.yaml", []byte(y))
		if err == nil || !strings.Contains(err.Error(), "schema") {
			t.Errorf("%s: ParseGraph() = %v, want an error naming schema", name, err)
		}
	}
	_, err = ParseGraph("graph.yaml", []byte(schemaHead+"schema: 2\n"+schemaTail))
	if err == nil || !strings.Contains(err.Error(), "needs graph schema 2") {
		t.Errorf("schema 2: ParseGraph() = %v, want an error with %q", err, "needs graph schema 2")
	}
}
