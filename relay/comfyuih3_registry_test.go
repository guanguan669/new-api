package relay

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/task/comfyuih3"
	"github.com/stretchr/testify/require"
)

func TestComfyUIH3TaskAdaptorRegistered(t *testing.T) {
	adaptor := GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeComfyUIH3)))
	require.IsType(t, &comfyuih3.TaskAdaptor{}, adaptor)
}
