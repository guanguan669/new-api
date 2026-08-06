package controller

import "github.com/QuantumNous/new-api/common"

func validateRunningHubAPIFormatResponse(body []byte) error {
	return validateRunningHubAPIFormatResponseForMode(body, common.RunningHubH3WorkflowImageToVideo)
}

func validateRunningHubAPIFormatResponseForMode(body []byte, mode common.RunningHubH3WorkflowMode) error {
	return common.ValidateRunningHubH3APIFormatForMode(body, mode)
}
