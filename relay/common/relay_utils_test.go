package common

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	hostcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeURLForLogMasksSensitiveQueryValues(t *testing.T) {
	rawURL := "https://example.test/v1beta/models/gemini:streamGenerateContent?alt=sse&key=sk-secret&access_token=ya29-secret&api-version=2024-02-01"

	got := SanitizeURLForLog(rawURL)

	assert.NotContains(t, got, "sk-secret")
	assert.NotContains(t, got, "ya29-secret")
	parsedURL, err := url.Parse(got)
	require.NoError(t, err)
	query := parsedURL.Query()
	assert.Equal(t, "***masked***", query.Get("key"))
	assert.Equal(t, "***masked***", query.Get("access_token"))
	assert.Equal(t, "sse", query.Get("alt"))
	assert.Equal(t, "2024-02-01", query.Get("api-version"))
}

func TestSanitizeURLForLogMasksAWSAndSecretLikeQueryKeys(t *testing.T) {
	rawURL := "https://example.test/path?X-Amz-Credential=credential&X-Amz-Signature=signature&session_token=session&client_secret=secret&model=gpt-test"

	got := SanitizeURLForLog(rawURL)

	assert.NotContains(t, got, "X-Amz-Credential=credential")
	assert.NotContains(t, got, "X-Amz-Signature=signature")
	assert.NotContains(t, got, "session_token=session")
	assert.NotContains(t, got, "client_secret=secret")
	parsedURL, err := url.Parse(got)
	require.NoError(t, err)
	query := parsedURL.Query()
	assert.Equal(t, "***masked***", query.Get("X-Amz-Credential"))
	assert.Equal(t, "***masked***", query.Get("X-Amz-Signature"))
	assert.Equal(t, "***masked***", query.Get("session_token"))
	assert.Equal(t, "***masked***", query.Get("client_secret"))
	assert.Equal(t, "gpt-test", query.Get("model"))
}

func TestSanitizeURLForLogKeepsURLWithoutSensitiveQuery(t *testing.T) {
	rawURL := "https://example.test/v1/chat/completions?api-version=2024-02-01&alt=sse"

	got := SanitizeURLForLog(rawURL)

	assert.Equal(t, rawURL, got)
}

func TestValidateMultipartDirectNormalizesImageField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.NewReader(`{"model":"wan2.7-i2v","prompt":"animate","image":" https://example.com/first.png "}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	info := &RelayInfo{
		TaskRelayInfo: &TaskRelayInfo{},
	}

	taskErr := ValidateMultipartDirect(context, info)

	require.Nil(t, taskErr)
	storedReq, err := GetTaskRequest(context)
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/first.png"}, storedReq.Images)
	require.Equal(t, constant.TaskActionGenerate, info.Action)
}

func TestTaskSubmitReqAcceptsKuocaiDocumentVideoFields(t *testing.T) {
	var req TaskSubmitReq
	err := hostcommon.Unmarshal([]byte(`{
		"model":"seedance",
		"prompt":"A sunset over the sea",
		"seconds":8,
		"count":2,
		"resolution":"720P",
		"reference_images":["https://cdn.example/one.jpg","https://cdn.example/two.jpg"]
	}`), &req)
	require.NoError(t, err)
	require.Equal(t, "8", req.Seconds)
	require.Equal(t, 2, req.Count)
	require.Equal(t, "720P", req.Metadata["resolution"])
	require.Equal(t, []interface{}{"https://cdn.example/one.jpg", "https://cdn.example/two.jpg"}, req.Metadata["reference_images"])
	require.True(t, req.ShouldEnhancePrompt())
}

func TestTaskSubmitReqPromptEnhanceTriState(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		enabled bool
	}{
		{name: "omitted defaults enabled", body: `{"prompt":"dance"}`, enabled: true},
		{name: "explicit true", body: `{"prompt":"dance","prompt_enhance":true}`, enabled: true},
		{name: "explicit false", body: `{"prompt":"dance","prompt_enhance":false}`, enabled: false},
		{name: "string false", body: `{"prompt":"dance","prompt_enhance":"false"}`, enabled: false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var req TaskSubmitReq
			require.NoError(t, hostcommon.Unmarshal([]byte(testCase.body), &req))
			require.Equal(t, testCase.enabled, req.ShouldEnhancePrompt())
			_, leaked := req.Metadata["prompt_enhance"]
			require.False(t, leaked)
		})
	}
}

func TestValidateBasicTaskRequestAcceptsOpenAIVideoMultipartForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "seedance"))
	require.NoError(t, writer.WriteField("prompt", "A sunset over the sea"))
	require.NoError(t, writer.WriteField("seconds", "8"))
	require.NoError(t, writer.WriteField("count", "2"))
	require.NoError(t, writer.WriteField("resolution", "720P"))
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	info := &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}

	taskErr := ValidateBasicTaskRequest(context, info, constant.TaskActionTextGenerate)
	require.Nil(t, taskErr)
	storedReq, err := GetTaskRequest(context)
	require.NoError(t, err)
	require.Equal(t, "8", storedReq.Seconds)
	require.Equal(t, 2, storedReq.Count)
	require.Equal(t, "720P", storedReq.Metadata["resolution"])
}

func TestValidateBasicTaskRequestParsesMultipartPromptEnhance(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		value   string
		present bool
		enabled bool
	}{
		{name: "omitted defaults enabled", enabled: true},
		{name: "explicit true", value: "true", present: true, enabled: true},
		{name: "explicit false", value: "false", present: true, enabled: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			require.NoError(t, writer.WriteField("model", "minimax_h3"))
			require.NoError(t, writer.WriteField("prompt", "A dancer"))
			if testCase.present {
				require.NoError(t, writer.WriteField("prompt_enhance", testCase.value))
			}
			require.NoError(t, writer.Close())

			request := httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = request
			info := &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}

			require.Nil(t, ValidateBasicTaskRequest(context, info, constant.TaskActionTextGenerate))
			storedReq, err := GetTaskRequest(context)
			require.NoError(t, err)
			require.Equal(t, testCase.enabled, storedReq.ShouldEnhancePrompt())
			_, leaked := storedReq.Metadata["prompt_enhance"]
			require.False(t, leaked)
		})
	}
}

func TestValidateBasicTaskRequestRejectsInvalidMultipartPromptEnhance(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "minimax_h3"))
	require.NoError(t, writer.WriteField("prompt", "A dancer"))
	require.NoError(t, writer.WriteField("prompt_enhance", "sometimes"))
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	taskErr := ValidateBasicTaskRequest(context, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}, constant.TaskActionTextGenerate)
	require.NotNil(t, taskErr)
	require.Equal(t, "invalid_multipart_form", taskErr.Code)
}

// TestTaskDurationBounds guards the billing invariant that user-supplied
// video duration (a quota multiplier via OtherRatio "seconds") is bounded, so
// it can never overflow quota calculation into a negative charge.
func TestTaskDurationBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newContext := func(t *testing.T, body string) (*gin.Context, *RelayInfo) {
		request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = request
		return context, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}
	}

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "huge duration is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","duration":9999999999}`,
			wantErr: true,
		},
		{
			name:    "huge seconds string is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","seconds":"9999999999"}`,
			wantErr: true,
		},
		{
			name:    "negative duration is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","duration":-8}`,
			wantErr: true,
		},
		{
			name: "normal duration is accepted",
			body: `{"model":"sora-2","prompt":"a cat","seconds":"8"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (multipart direct)", func(t *testing.T) {
			context, info := newContext(t, tt.body)
			taskErr := ValidateMultipartDirect(context, info)
			if tt.wantErr {
				require.NotNil(t, taskErr)
				require.Equal(t, "invalid_seconds", taskErr.Code)
			} else {
				require.Nil(t, taskErr)
			}
		})
		t.Run(tt.name+" (basic task request)", func(t *testing.T) {
			context, info := newContext(t, tt.body)
			taskErr := ValidateBasicTaskRequest(context, info, constant.TaskActionGenerate)
			if tt.wantErr {
				require.NotNil(t, taskErr)
				require.Equal(t, "invalid_seconds", taskErr.Code)
			} else {
				require.Nil(t, taskErr)
			}
		})
	}
}
