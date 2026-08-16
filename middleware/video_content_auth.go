package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// VideoContentAuth accepts the normal dashboard/API credentials or a short-
// lived signature issued in a completed task's metadata.url. API credentials
// retain their existing precedence when both forms are present.
func VideoContentAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, hasAuthorization := authorizationToken(c.GetHeader("Authorization")); hasAuthorization {
			TokenOrUserAuth()(c)
			return
		}

		rawUserID := strings.TrimSpace(c.Query("user_id"))
		rawExpiresAt := strings.TrimSpace(c.Query("expires"))
		signature := strings.TrimSpace(c.Query("signature"))
		if rawUserID == "" && rawExpiresAt == "" && signature == "" {
			TokenOrUserAuth()(c)
			return
		}

		userID, err := service.VerifySignedVideoContentURL(c.Param("task_id"), rawUserID, rawExpiresAt, signature)
		if err != nil {
			message := "Invalid video download link"
			if errors.Is(err, service.ErrVideoContentLinkExpired) {
				message = "Video download link has expired"
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": message,
					"type":    "authentication_error",
				},
			})
			return
		}

		c.Set("id", userID)
		c.Set("video_content_signed", true)
		c.Next()
	}
}
