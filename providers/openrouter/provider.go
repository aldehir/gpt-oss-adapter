// TODO: OpenRouter needs a bit more work
package openrouter

import "github.com/aldehir/gpt-oss-adapter/providers/types"

func NewProvider() types.Provider {
	return types.Provider{
		Name:            "openrouter",
		Reasoning:       "reasoning",
		ReasoningEffort: "reasoning.effort",
	}
}
