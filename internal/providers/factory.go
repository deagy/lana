package providers

import (
	"fmt"

	"github.com/deagy/lana/internal/provider"
)

// Factory creates providers based on configuration.
type Factory struct {
	providerName string
	model        string
	endpoint     string
	apiKey       string
}

// NewFactory creates a new provider factory.
func NewFactory(providerName, model, endpoint, apiKey string) *Factory {
	return &Factory{
		providerName: providerName,
		model:        model,
		endpoint:     endpoint,
		apiKey:       apiKey,
	}
}

// Create instantiates a provider based on the factory configuration.
func (f *Factory) Create() (provider.Client, error) {
	switch f.providerName {
	case "openai-compat":
		return NewOpenAICompatibleClient(f.endpoint, f.apiKey, f.model), nil
	case "ollama":
		return NewOllamaClient(f.endpoint, f.model), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", f.providerName)
	}
}

// AvailableProviders returns a list of available provider names.
func AvailableProviders() []string {
	return []string{
		"openai-compat",
		"ollama",
	}
}

// ProviderDescription returns a human-readable description of a provider.
func ProviderDescription(name string) string {
	descriptions := map[string]string{
		"openai-compat": "OpenAI-compatible API provider (OpenAI, LM Studio, LiteLLM, OpenRouter, vLLM, etc.)",
		"ollama":        "Local Ollama endpoint",
	}

	desc, ok := descriptions[name]
	if !ok {
		return "Unknown provider"
	}
	return desc
}
