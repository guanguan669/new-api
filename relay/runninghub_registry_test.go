package relay

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/task/runninghub"
	"github.com/stretchr/testify/require"
)

func TestRunningHubTaskAdaptorRegistered(t *testing.T) {
	adaptor := GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeRunningHub)))
	require.IsType(t, &runninghub.TaskAdaptor{}, adaptor)
}
