package common

import (
	"fmt"
	"strings"
)

type RunningHubH3WorkflowMode string

const (
	RunningHubH3WorkflowTextToVideo  RunningHubH3WorkflowMode = "text_to_video"
	RunningHubH3WorkflowImageToVideo RunningHubH3WorkflowMode = "image_to_video"
)

// ValidateRunningHubH3APIFormat verifies that a RunningHub workflow remains
// compatible with the fixed MiniMax H3 video channel before a billable task is
// submitted.
func ValidateRunningHubH3APIFormat(body []byte) error {
	return ValidateRunningHubH3APIFormatForMode(body, RunningHubH3WorkflowImageToVideo)
}

func ValidateRunningHubH3APIFormatForMode(body []byte, mode RunningHubH3WorkflowMode) error {
	var envelope map[string]any
	if err := Unmarshal(body, &envelope); err != nil {
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
	return validateRunningHubH3Prompt(prompt, mode)
}

func runningHubPromptObject(promptValue any) (map[string]any, error) {
	switch prompt := promptValue.(type) {
	case map[string]any:
		return prompt, nil
	case string:
		var parsed map[string]any
		if err := Unmarshal([]byte(prompt), &parsed); err != nil {
			return nil, fmt.Errorf("RunningHub data.prompt is not valid Comfy API JSON: %w", err)
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("RunningHub data.prompt must be a Comfy API JSON object")
	}
}

func validateRunningHubH3Prompt(prompt map[string]any, mode RunningHubH3WorkflowMode) error {
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

	if mode == RunningHubH3WorkflowImageToVideo {
		for _, nodeID := range []string{"137", "618", "617", "619", "627", "626", "625", "624", "623"} {
			inputs, ok := runningHubNodeInputs(prompt, nodeID)
			if !ok {
				return fmt.Errorf("RunningHub H3 prompt missing image node %s inputs", nodeID)
			}
			if _, exists := inputs["image"]; !exists {
				return fmt.Errorf("RunningHub H3 prompt missing image node %s input image", nodeID)
			}
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

	videoNode, ok := prompt["92"].(map[string]any)
	if !ok || !strings.EqualFold(strings.TrimSpace(fmt.Sprint(videoNode["class_type"])), "SaveVideo") {
		return fmt.Errorf("RunningHub H3 prompt missing video output node 92 (SaveVideo)")
	}
	videoInputs, ok := videoNode["inputs"].(map[string]any)
	if !ok || videoInputs["video"] == nil {
		return fmt.Errorf("RunningHub H3 prompt missing video output node 92 input video")
	}

	for nodeID, value := range prompt {
		node, ok := value.(map[string]any)
		if !ok || nodeID == "92" {
			continue
		}
		classType := strings.ToLower(strings.TrimSpace(fmt.Sprint(node["class_type"])))
		if strings.Contains(classType, "saveimage") || strings.Contains(classType, "previewimage") {
			return fmt.Errorf("RunningHub H3 prompt has competing non-video output node %s (%s); remove it so SaveVideo is the only final output", nodeID, node["class_type"])
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
