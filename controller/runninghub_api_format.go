package controller

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

func validateRunningHubAPIFormatResponse(body []byte) error {
	var envelope map[string]any
	if err := common.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("invalid RunningHub API format response: %w", err)
	}

	if success, ok := envelope["success"].(bool); ok && !success {
		return fmt.Errorf("RunningHub API format check failed: %s", runningHubEnvelopeMessage(envelope))
	}
	if code, ok := runningHubNumber(envelope["code"]); ok && code != 0 && code != 200 {
		return fmt.Errorf("RunningHub API format check failed: code %d: %s", int(code), runningHubEnvelopeMessage(envelope))
	}

	data, ok := envelope["data"].(map[string]any)
	if !ok {
		return fmt.Errorf("RunningHub API format response missing data")
	}
	promptValue, ok := data["prompt"]
	if !ok {
		return fmt.Errorf("RunningHub API format response missing data.prompt")
	}

	prompt, err := runningHubPromptObject(promptValue)
	if err != nil {
		return err
	}
	return validateRunningHubH3Prompt(prompt)
}

func runningHubPromptObject(promptValue any) (map[string]any, error) {
	switch prompt := promptValue.(type) {
	case map[string]any:
		return prompt, nil
	case string:
		var parsed map[string]any
		if err := common.Unmarshal([]byte(prompt), &parsed); err != nil {
			return nil, fmt.Errorf("RunningHub data.prompt is not valid Comfy API JSON: %w", err)
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("RunningHub data.prompt must be a Comfy API JSON object")
	}
}

func validateRunningHubH3Prompt(prompt map[string]any) error {
	requiredInputs := map[string][]string{
		"138": {"value"},
		"132": {"value"},
		"115": {"aspect_ratio", "megapixels", "multiple"},
	}
	for nodeID, fields := range requiredInputs {
		inputs, ok := runningHubNodeInputs(prompt, nodeID)
		if !ok {
			return fmt.Errorf("RunningHub H3 prompt missing node %s inputs", nodeID)
		}
		for _, field := range fields {
			if _, exists := inputs[field]; !exists {
				return fmt.Errorf("RunningHub H3 prompt missing node %s input %s", nodeID, field)
			}
		}
	}

	for _, nodeID := range []string{"137", "618", "617", "619", "627", "626", "625", "624", "623"} {
		inputs, ok := runningHubNodeInputs(prompt, nodeID)
		if !ok {
			return fmt.Errorf("RunningHub H3 prompt missing image node %s inputs", nodeID)
		}
		if _, exists := inputs["image"]; !exists {
			return fmt.Errorf("RunningHub H3 prompt missing image node %s input image", nodeID)
		}
	}
	for _, nodeID := range []string{"628", "630", "629"} {
		inputs, ok := runningHubNodeInputs(prompt, nodeID)
		if !ok {
			return fmt.Errorf("RunningHub H3 prompt missing audio node %s inputs", nodeID)
		}
		if _, exists := inputs["audio"]; !exists {
			return fmt.Errorf("RunningHub H3 prompt missing audio node %s input audio", nodeID)
		}
	}
	return nil
}

func runningHubNodeInputs(prompt map[string]any, nodeID string) (map[string]any, bool) {
	node, ok := prompt[nodeID].(map[string]any)
	if !ok {
		return nil, false
	}
	inputs, ok := node["inputs"].(map[string]any)
	return inputs, ok
}

func runningHubEnvelopeMessage(envelope map[string]any) string {
	for _, key := range []string{"message", "msg", "error"} {
		if value, ok := envelope[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "unknown error"
}

func runningHubNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case int:
		return float64(number), true
	default:
		return 0, false
	}
}
