package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVideoContentAuthAcceptsSignedTaskOwnerLink(t *testing.T) {
	previousSecret := common.SessionSecret
	common.SessionSecret = "video-content-auth-test-secret"
	t.Cleanup(func() { common.SessionSecret = previousSecret })

	gin.SetMode(gin.TestMode)
	link, err := service.BuildSignedVideoContentURL("https://api.example.com/v1/videos/task_one/content", "task_one", 42)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/v1/videos/:task_id/content", VideoContentAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetInt("id")})
	})

	request := httptest.NewRequest(http.MethodGet, link, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"user_id":42}`, recorder.Body.String())

	tamperedRequest := httptest.NewRequest(http.MethodGet, "/v1/videos/task_other/content?"+request.URL.RawQuery, nil)
	tamperedRecorder := httptest.NewRecorder()
	router.ServeHTTP(tamperedRecorder, tamperedRequest)
	require.Equal(t, http.StatusUnauthorized, tamperedRecorder.Code)
}
