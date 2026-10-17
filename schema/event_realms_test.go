package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

func TestTheLedgerRecordsRealmsAndTasks(t *testing.T) {
	e := event(gungnir.EventRealmLoaded)
	e.Realm, e.RealmVersion, e.Note = "midgard-react", "0.1.0", "sha256:abc"
	line, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"ygg.realm.name":"midgard-react"`, `"ygg.realm.version":"0.1.0"`} {
		if !strings.Contains(string(line), key) {
			t.Errorf("line lacks %s:\n%s", key, line)
		}
	}
	if err := ValidateEvent(line); err != nil {
		t.Errorf("ValidateEvent() = %v", err)
	}
	for _, name := range []gungnir.EventName{gungnir.EventDocsPublished, gungnir.EventTasksReady, gungnir.EventNodeReturned,
		gungnir.EventTaskStarted, gungnir.EventTaskDone, gungnir.EventTaskBlocked} {
		e := event(name)
		e.Task = "gallery-t001"
		line, _ := json.Marshal(e)
		if !strings.Contains(string(line), `"ygg.task.id":"gallery-t001"`) || ValidateEvent(line) != nil {
			t.Errorf("%s: %s", name, line)
		}
	}
	bad := strings.Replace(string(line), `"midgard-react"`, `"Midgard React"`, 1)
	if ValidateEvent([]byte(bad)) == nil {
		t.Error("a realm name in capitals validated")
	}
}
