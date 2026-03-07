package adapters

import "testing"

func TestLookupProvider(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantName   string
		wantCmd    string
		wantStub   bool
		wantSup    bool
		expectFail bool
	}{
		{
			name:     "codex",
			input:    "codex",
			wantName: "codex",
			wantCmd:  "codex --sandbox danger-full-access --ask-for-approval never",
			wantSup:  true,
			wantStub: false,
		},
		{
			name:     "gemini stub",
			input:    "gemini",
			wantName: "gemini",
			wantCmd:  "gemini --yolo",
			wantSup:  false,
			wantStub: true,
		},
		{
			name:     "claude alias to cloud",
			input:    "claude",
			wantName: "cloud",
			wantCmd:  "claude --dangerously-skip-permissions",
			wantSup:  false,
			wantStub: true,
		},
		{
			name:       "unknown",
			input:      "unknown",
			expectFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Lookup(tt.input)
			if tt.expectFail {
				if err == nil {
					t.Fatal("expected lookup to fail")
				}
				return
			}
			if err != nil {
				t.Fatalf("lookup failed: %v", err)
			}
			if got.Name() != tt.wantName {
				t.Fatalf("adapter name mismatch: want %q got %q", tt.wantName, got.Name())
			}
			if got.DefaultCommand() != tt.wantCmd {
				t.Fatalf("adapter command mismatch: want %q got %q", tt.wantCmd, got.DefaultCommand())
			}
			if got.SupportsHarnessSupervisor() != tt.wantSup {
				t.Fatalf("supportsHarness mismatch: want %v got %v", tt.wantSup, got.SupportsHarnessSupervisor())
			}
			if got.IsStub() != tt.wantStub {
				t.Fatalf("stub mismatch: want %v got %v", tt.wantStub, got.IsStub())
			}
		})
	}
}
