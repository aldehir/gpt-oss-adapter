package llamacpp

import "github.com/aldehir/gpt-oss-adapter/providers/types"

func NewProvider() types.Provider {
	return types.Provider{
		Name:            "llama-cpp",
		Reasoning:       "reasoning_content",
		ReasoningEffort: "chat_template_kwargs.reasoning_effort",
	}
}
