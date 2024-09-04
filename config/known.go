package config

import "strings"

func ModelType(model string) string {
	switch {
	case strings.Contains(model, "command"):
		return ModelTypeChat
	case strings.Contains(model, "gemma"):
		return ModelTypeChat
	case strings.Contains(model, "llama"):
		return ModelTypeChat
	case strings.Contains(model, "whisper"):
		return ModelTypeSpeechToText
	default:
		panic("unknown model: " + model)
	}
}
