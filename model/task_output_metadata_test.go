package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPrivateDataOutputMetadataJSONCompatibility(t *testing.T) {
	t.Run("old data remains readable", func(t *testing.T) {
		var privateData TaskPrivateData
		require.NoError(t, common.Unmarshal([]byte(`{"upstream_task_id":"upstream-1","result_url":"https://example.com/video.mp4"}`), &privateData))

		assert.Equal(t, "upstream-1", privateData.UpstreamTaskID)
		assert.Equal(t, "https://example.com/video.mp4", privateData.ResultURL)
		assert.Zero(t, privateData.OutputSeconds)
		assert.Empty(t, privateData.OutputSize)
	})

	t.Run("new fields round trip", func(t *testing.T) {
		original := TaskPrivateData{
			UpstreamTaskID: "upstream-2",
			OutputSeconds:  5,
			OutputSize:     "1376x768",
		}

		encoded, err := common.Marshal(original)
		require.NoError(t, err)
		assert.JSONEq(t, `{"upstream_task_id":"upstream-2","output_seconds":5,"output_size":"1376x768"}`, string(encoded))

		var decoded TaskPrivateData
		require.NoError(t, common.Unmarshal(encoded, &decoded))
		assert.Equal(t, original, decoded)
	})

	t.Run("empty output fields remain omitted", func(t *testing.T) {
		encoded, err := common.Marshal(TaskPrivateData{UpstreamTaskID: "upstream-3"})
		require.NoError(t, err)
		assert.JSONEq(t, `{"upstream_task_id":"upstream-3"}`, string(encoded))
	})
}

func TestTaskToOpenAIVideoOutputMetadata(t *testing.T) {
	tests := []struct {
		name            string
		status          TaskStatus
		privateData     TaskPrivateData
		expectedSeconds string
		expectedSize    string
	}{
		{
			name:   "successful task exposes both fields",
			status: TaskStatusSuccess,
			privateData: TaskPrivateData{
				OutputSeconds: 5,
				OutputSize:    "1376x768",
			},
			expectedSeconds: "5",
			expectedSize:    "1376x768",
		},
		{
			name:   "successful old task omits unavailable fields",
			status: TaskStatusSuccess,
		},
		{
			name:   "non successful task does not expose frozen fields",
			status: TaskStatusInProgress,
			privateData: TaskPrivateData{
				OutputSeconds: 10,
				OutputSize:    "1920x1088",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{
				TaskID:      "task-test",
				Status:      tt.status,
				PrivateData: tt.privateData,
			}

			video := task.ToOpenAIVideo()

			assert.Equal(t, tt.expectedSeconds, video.Seconds)
			assert.Equal(t, tt.expectedSize, video.Size)

			encoded, err := common.Marshal(video)
			require.NoError(t, err)
			var response map[string]any
			require.NoError(t, common.Unmarshal(encoded, &response))
			if tt.expectedSeconds == "" {
				assert.NotContains(t, response, "seconds")
			} else {
				assert.Equal(t, tt.expectedSeconds, response["seconds"])
			}
			if tt.expectedSize == "" {
				assert.NotContains(t, response, "size")
			} else {
				assert.Equal(t, tt.expectedSize, response["size"])
			}
		})
	}
}

func TestTaskOutputMetadataSurvivesTaskDataReplacement(t *testing.T) {
	task := Task{
		TaskID: "task-polling",
		Status: TaskStatusSuccess,
		PrivateData: TaskPrivateData{
			OutputSeconds: 5,
			OutputSize:    "1376x768",
		},
		Data: []byte(`{"create_response":true}`),
	}

	task.Data = []byte(`{"poll_response":true}`)
	video := task.ToOpenAIVideo()

	assert.Equal(t, "5", video.Seconds)
	assert.Equal(t, "1376x768", video.Size)
}
