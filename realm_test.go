package gungnir

import "testing"

func TestValidRealmAndVersion(t *testing.T) {
	for _, s := range []string{"midgard-react", "helheim-go", "asgard", "midgard-react2"} {
		if !ValidRealm(s) {
			t.Errorf("ValidRealm(%q) = false", s)
		}
	}
	for _, s := range []string{"", "Midgard", "midgard_react", "-midgard", "midgard-", "midgard--react", "2fast", "midgard react", "../x"} {
		if ValidRealm(s) {
			t.Errorf("ValidRealm(%q) = true", s)
		}
	}
	for _, s := range []string{"0.1.0", "12.0.103"} {
		if !ValidVersion(s) {
			t.Errorf("ValidVersion(%q) = false", s)
		}
	}
	for _, s := range []string{"", "1", "1.2", "v1.2.3", "1.2.3-rc1", "1.2.3.4", "1.x.3"} {
		if ValidVersion(s) {
			t.Errorf("ValidVersion(%q) = true", s)
		}
	}
}

func TestARepoCarriesItsRealmPin(t *testing.T) {
	ok := Repo{Owner: "o", Name: "api", Visibility: VisibilityPrivate, Path: "repos/api", Files: map[string][]byte{"LICENSE": []byte("x")},
		Realm: "helheim-go", RealmVersion: "0.1.0", Consumes: []string{"repos/db"}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	for name, mutate := range map[string]func(*Repo){
		"a realm with no version":     func(r *Repo) { r.RealmVersion = "" },
		"a version with no realm":     func(r *Repo) { r.Realm = "" },
		"a bad realm name":            func(r *Repo) { r.Realm = "Helheim Go" },
		"a bad version":               func(r *Repo) { r.RealmVersion = "latest" },
		"a consumer path that climbs": func(r *Repo) { r.Consumes = []string{"../x"} },
		"a consumer listed twice":     func(r *Repo) { r.Consumes = []string{"repos/db", "repos/db"} },
		"itself as a consumer":        func(r *Repo) { r.Consumes = []string{"repos/api"} },
	} {
		r := ok
		r.Consumes = append([]string(nil), ok.Consumes...)
		mutate(&r)
		if r.Validate() == nil {
			t.Errorf("%s: Validate() = nil", name)
		}
	}
	plain := ok
	plain.Realm, plain.RealmVersion, plain.Consumes = "", "", nil
	if err := plain.Validate(); err != nil {
		t.Errorf("a repo with no pin = %v; it uses the graph pack", err)
	}
}

func TestValidGraph(t *testing.T) {
	for _, s := range []string{"software", "web-app", "a1"} {
		if !ValidGraph(s) {
			t.Errorf("ValidGraph(%q) = false", s)
		}
	}
	for _, s := range []string{"", "Software", "web_app", "-web", "web-", "1web", "a/b"} {
		if ValidGraph(s) {
			t.Errorf("ValidGraph(%q) = true", s)
		}
	}
}
