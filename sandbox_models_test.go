package gungnir

import "testing"

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
