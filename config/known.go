package config

import (
	"fmt"
	"strings"
)

func ModelType(model string) (string, error) {
	switch {
	case strings.Contains(model, "command"):
		return ModelTypeChat, nil
	case strings.Contains(model, "gemma"):
		return ModelTypeChat, nil
	case strings.Contains(model, "llama"):
		return ModelTypeChat, nil
	case strings.Contains(model, "llava"):
		return ModelTypeChatImage, nil
	case strings.Contains(model, "whisper"):
		return ModelTypeSpeechToText, nil
	default:
		return "", fmt.Errorf("unknown model: %s", model)
	}
}
