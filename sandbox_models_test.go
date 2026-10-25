package gungnir

import (
	"strings"
	"testing"
)

func TestSandboxSpecModels(t *testing.T) {
	tests := []struct {
		name   string
		models *ModelServer
		ok     bool
	}{
		{"none", nil, true},
		{"valid", &ModelServer{Network: "ygg-models", Name: "gate", Port: 8080}, true},
		{"bad name", &ModelServer{Network: "ygg-models", Name: "Gate\nx", Port: 8080}, false},
		{"empty name", &ModelServer{Network: "ygg-models", Name: "", Port: 8080}, false},
		{"bad network", &ModelServer{Network: "-models; rm", Name: "gate", Port: 8080}, false},
		{"dotted name", &ModelServer{Network: "ygg-models", Name: "models.example.com", Port: 8080}, false},
		{"trailing dot", &ModelServer{Network: "ygg-models", Name: "a.", Port: 8080}, false},
		{"underscore name", &ModelServer{Network: "ygg-models", Name: "a_b", Port: 8080}, false},
		{"dash first", &ModelServer{Network: "ygg-models", Name: "-a", Port: 8080}, false},
		{"dash last", &ModelServer{Network: "ygg-models", Name: "a-", Port: 8080}, false},
		{"name of 64", &ModelServer{Network: "ygg-models", Name: strings.Repeat("a", 64), Port: 8080}, false},
		{"name of 63", &ModelServer{Network: "ygg-models", Name: strings.Repeat("a", 63), Port: 8080}, true},
		{"name newline", &ModelServer{Network: "ygg-models", Name: "gate\n", Port: 8080}, false},
		{"dotted network", &ModelServer{Network: "ygg.models_1", Name: "gate", Port: 8080}, true},
		{"network newline", &ModelServer{Network: "ygg\n", Name: "gate", Port: 8080}, false},
		{"port 1", &ModelServer{Network: "ygg-models", Name: "gate", Port: 1}, true},
		{"port 65535", &ModelServer{Network: "ygg-models", Name: "gate", Port: 65535}, true},
		{"port -1", &ModelServer{Network: "ygg-models", Name: "gate", Port: -1}, false},
		{"port 0", &ModelServer{Network: "ygg-models", Name: "gate", Port: 0}, false},
		{"port too high", &ModelServer{Network: "ygg-models", Name: "gate", Port: 65536}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSpec()
			s.Models = tt.models
			if err := s.Validate(); (err == nil) != tt.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}
