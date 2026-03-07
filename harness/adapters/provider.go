package adapters

import (
	"fmt"
	"sort"
	"strings"
)

// ProviderAdapter describes a provider integration and its default runtime command.
type ProviderAdapter interface {
	Name() string
	DefaultCommand() string
	SupportsHarnessSupervisor() bool
	IsStub() bool
}

type providerAdapter struct {
	name                      string
	defaultCommand            string
	supportsHarnessSupervisor bool
	stub                      bool
}

func (p providerAdapter) Name() string                    { return p.name }
func (p providerAdapter) DefaultCommand() string          { return p.defaultCommand }
func (p providerAdapter) SupportsHarnessSupervisor() bool { return p.supportsHarnessSupervisor }
func (p providerAdapter) IsStub() bool                    { return p.stub }

var providerRegistry = map[string]ProviderAdapter{
	"codex": providerAdapter{
		name:                      "codex",
		defaultCommand:            "codex --sandbox danger-full-access --ask-for-approval never",
		supportsHarnessSupervisor: true,
		stub:                      false,
	},
	"gemini": providerAdapter{
		name:                      "gemini",
		defaultCommand:            "gemini --yolo",
		supportsHarnessSupervisor: false,
		stub:                      true,
	},
	"cloud": providerAdapter{
		name:                      "cloud",
		defaultCommand:            "claude --dangerously-skip-permissions",
		supportsHarnessSupervisor: false,
		stub:                      true,
	},
}

// Lookup resolves a provider adapter by name.
// Aliases:
// - claude -> cloud
func Lookup(name string) (ProviderAdapter, error) {
	v := strings.ToLower(strings.TrimSpace(name))
	if v == "claude" {
		v = "cloud"
	}
	adapter, ok := providerRegistry[v]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q", name)
	}
	return adapter, nil
}

func SupportedProviders() []string {
	out := make([]string, 0, len(providerRegistry))
	for name := range providerRegistry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
