package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const VideoContentLinkTTL = 24 * time.Hour

var (
	ErrVideoContentLinkInvalid = errors.New("video content link is invalid")
	ErrVideoContentLinkExpired = errors.New("video content link has expired")
)

// BuildSignedVideoContentURL adds an expiring, task-owner-bound signature to
// an already-public content endpoint. It deliberately does not include any API
// token, upstream credential, or worker URL.
func BuildSignedVideoContentURL(contentURL, taskID string, userID int) (string, error) {
	if strings.TrimSpace(taskID) == "" || userID <= 0 {
		return "", ErrVideoContentLinkInvalid
	}
	parsedURL, err := url.Parse(contentURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", ErrVideoContentLinkInvalid
	}
	expiresAt := time.Now().Add(VideoContentLinkTTL).Unix()
	query := parsedURL.Query()
	query.Set("expires", strconv.FormatInt(expiresAt, 10))
	query.Set("user_id", strconv.Itoa(userID))
	query.Set("signature", signVideoContentLink(taskID, userID, expiresAt))
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

// VerifySignedVideoContentURL validates a signed link and returns the task
// owner ID that must be used for the subsequent task lookup.
func VerifySignedVideoContentURL(taskID, rawUserID, rawExpiresAt, signature string) (int, error) {
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(signature) == "" {
		return 0, ErrVideoContentLinkInvalid
	}
	userID, err := strconv.Atoi(rawUserID)
	if err != nil || userID <= 0 {
		return 0, ErrVideoContentLinkInvalid
	}
	expiresAt, err := strconv.ParseInt(rawExpiresAt, 10, 64)
	if err != nil || expiresAt <= 0 {
		return 0, ErrVideoContentLinkInvalid
	}
	if time.Now().Unix() >= expiresAt {
		return 0, ErrVideoContentLinkExpired
	}
	received, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return 0, ErrVideoContentLinkInvalid
	}
	expected, err := base64.RawURLEncoding.DecodeString(signVideoContentLink(taskID, userID, expiresAt))
	if err != nil || !hmac.Equal(received, expected) {
		return 0, ErrVideoContentLinkInvalid
	}
	return userID, nil
}

func signVideoContentLink(taskID string, userID int, expiresAt int64) string {
	mac := hmac.New(sha256.New, videoContentLinkSigningKey())
	_, _ = mac.Write([]byte(taskID))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(strconv.Itoa(userID)))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(strconv.FormatInt(expiresAt, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func videoContentLinkSigningKey() []byte {
	mac := hmac.New(sha256.New, []byte(common.SessionSecret))
	_, _ = mac.Write([]byte("new-api/video-content-link/v1"))
	return mac.Sum(nil)
}
