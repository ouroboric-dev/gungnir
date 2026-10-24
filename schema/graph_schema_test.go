package schema

import (
	"strings"
	"testing"
)

const schemaHead = "name: toy\nversion: 1.0.0\n"
const schemaTail = "nodes:\n  - { id: a, kind: agent, role: planner, prompt: p.md, output: ygg.plan/v1 }\n"

func TestAGraphStatesItsSchemaVersion(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(schemaHead+"schema: 2\n"+schemaTail))
	if err != nil || g.Schema != 2 || GraphSchema != 2 {
		t.Fatalf("ParseGraph() = %+v, %v; want Schema 2", g, err)
	}
	for name, y := range map[string]string{
		"no schema":       schemaHead + schemaTail,
		"schema 0":        schemaHead + "schema: 0\n" + schemaTail,
		"a string schema": schemaHead + "schema: \"1\"\n" + schemaTail,
	} {
		_, err := ParseGraph("graph.yaml", []byte(y))
		if err == nil || !strings.Contains(err.Error(), "/schema") {
			t.Errorf("%s: ParseGraph() = %v, want an error naming schema", name, err)
		}
	}
	_, err = ParseGraph("graph.yaml", []byte(schemaHead+"schema: 3\n"+schemaTail))
	if err == nil || !strings.Contains(err.Error(), "needs graph schema 3, and this Gungnir reads schema 2 at most") {
		t.Errorf("schema 3: ParseGraph() = %v, want an error with %q", err, "needs graph schema 3")
	}
	// A real schema 3 graph uses what schema 2 does not know; the refusal still names the schema.
	for name, y := range map[string]string{
		"an unknown top-level key": schemaHead + "schema: 3\nmodels: [x]\n" + schemaTail,
		"an unknown node kind":     schemaHead + "schema: 3\nnodes:\n  - { id: a, kind: fanout, role: planner }\n",
	} {
		_, err := ParseGraph("graph.yaml", []byte(y))
		if err == nil || !strings.Contains(err.Error(), "needs graph schema 3") || strings.Contains(err.Error(), "jsonschema") {
			t.Errorf("%s: ParseGraph() = %v, want only %q", name, err, "needs graph schema 3")
		}
	}
}
