package lmstudio

import "github.com/aldehir/gpt-oss-adapter/providers/types"

func NewProvider() types.Provider {
	return types.Provider{
		Name:            "lmstudio",
		Reasoning:       "reasoning",
		ReasoningEffort: "reasoning.effort",
	}
}
