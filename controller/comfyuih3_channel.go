package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/comfyuih3"
	"github.com/QuantumNous/new-api/service"
)

func validateComfyUIH3ChannelCapabilities(ctx context.Context, channel *model.Channel, baseURL string) error {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return fmt.Errorf("ComfyUI H3 channel base URL is empty")
	}
	client, err := service.NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return err
	}
	if err := validateComfyUIJSONEndpoint(ctx, client, baseURL+"/system_stats", "system stats"); err != nil {
		return err
	}
	nodeTypes, err := comfyuih3.RequiredNodeTypes()
	if err != nil {
		return err
	}
	choices, err := comfyuih3.RequiredNodeChoices()
	if err != nil {
		return err
	}
	choicesByNodeType := make(map[string][]comfyuih3.RequiredNodeChoice, len(choices))
	for _, choice := range choices {
		choicesByNodeType[choice.NodeType] = append(choicesByNodeType[choice.NodeType], choice)
	}
	for _, nodeType := range nodeTypes {
		endpoint := baseURL + "/object_info/" + url.PathEscape(nodeType)
		objectInfo, err := getComfyUIJSONEndpoint(ctx, client, endpoint, "node "+nodeType)
		if err != nil {
			return err
		}
		nodeInfo, ok := objectInfo[nodeType].(map[string]any)
		if !ok || len(nodeInfo) == 0 {
			return fmt.Errorf("ComfyUI H3 node %s check returned no node definition", nodeType)
		}
		for _, choice := range choicesByNodeType[nodeType] {
			if !comfyUIObjectInfoHasChoice(nodeInfo, choice.InputName, choice.Value) {
				return fmt.Errorf("ComfyUI H3 node %s does not offer required %s %q", nodeType, choice.InputName, choice.Value)
			}
		}
	}
	return nil
}

func validateComfyUIH3ChannelWorkers(ctx context.Context, channel *model.Channel) error {
	if channel == nil {
		return fmt.Errorf("ComfyUI H3 channel is empty")
	}
	settings := channel.GetOtherSettings()
	workerURLs := make([]string, 0, len(settings.ComfyUIH3BackendURLs)+1)
	seen := make(map[string]struct{}, len(settings.ComfyUIH3BackendURLs)+1)
	appendWorker := func(rawURL string) {
		workerURL := strings.TrimRight(strings.TrimSpace(rawURL), "/")
		if workerURL == "" {
			return
		}
		if _, exists := seen[workerURL]; exists {
			return
		}
		seen[workerURL] = struct{}{}
		workerURLs = append(workerURLs, workerURL)
	}
	for _, workerURL := range settings.ComfyUIH3BackendURLs {
		appendWorker(workerURL)
	}
	appendWorker(channel.GetBaseURL())
	if len(workerURLs) == 0 {
		return fmt.Errorf("ComfyUI H3 channel has no worker URL")
	}

	var failures []string
	for _, workerURL := range workerURLs {
		if err := validateComfyUIH3ChannelCapabilities(ctx, channel, workerURL); err != nil {
			failures = append(failures, workerURL+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("ComfyUI H3 worker validation failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

func validateComfyUIJSONEndpoint(ctx context.Context, client *http.Client, endpoint string, label string) error {
	_, err := getComfyUIJSONEndpoint(ctx, client, endpoint, label)
	return err
}

func getComfyUIJSONEndpoint(ctx context.Context, client *http.Client, endpoint string, label string) (map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("ComfyUI H3 %s check failed: %w", label, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		message := strings.TrimSpace(string(body))
		if len(message) > 200 {
			message = message[:200]
		}
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return nil, fmt.Errorf("ComfyUI H3 %s check failed: status %d: %s", label, response.StatusCode, message)
	}
	var object map[string]any
	if err := common.Unmarshal(body, &object); err != nil || len(object) == 0 {
		return nil, fmt.Errorf("ComfyUI H3 %s check returned invalid JSON", label)
	}
	return object, nil
}

func comfyUIObjectInfoHasChoice(nodeInfo map[string]any, inputName string, requiredValue string) bool {
	input, ok := nodeInfo["input"].(map[string]any)
	if !ok {
		return false
	}
	required, ok := input["required"].(map[string]any)
	if !ok {
		return false
	}
	definition, ok := required[inputName].([]any)
	if !ok || len(definition) == 0 {
		return false
	}
	choices, ok := definition[0].([]any)
	if !ok {
		return false
	}
	for _, choice := range choices {
		if value, ok := choice.(string); ok && value == requiredValue {
			return true
		}
	}
	return false
}
