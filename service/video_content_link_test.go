package service

import (
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSignedVideoContentURLBindsTaskOwnerAndExpiry(t *testing.T) {
	previousSecret := common.SessionSecret
	common.SessionSecret = "video-content-link-test-secret"
	t.Cleanup(func() { common.SessionSecret = previousSecret })

	link, err := BuildSignedVideoContentURL("https://api.example.com/v1/videos/task_one/content", "task_one", 42)
	require.NoError(t, err)
	parsed, err := url.Parse(link)
	require.NoError(t, err)
	query := parsed.Query()
	require.Equal(t, "42", query.Get("user_id"))
	require.NotEmpty(t, query.Get("signature"))

	userID, err := VerifySignedVideoContentURL("task_one", query.Get("user_id"), query.Get("expires"), query.Get("signature"))
	require.NoError(t, err)
	require.Equal(t, 42, userID)

	_, err = VerifySignedVideoContentURL("task_other", query.Get("user_id"), query.Get("expires"), query.Get("signature"))
	require.ErrorIs(t, err, ErrVideoContentLinkInvalid)
	_, err = VerifySignedVideoContentURL("task_one", "43", query.Get("expires"), query.Get("signature"))
	require.ErrorIs(t, err, ErrVideoContentLinkInvalid)

	expiredAt := time.Now().Add(-time.Second).Unix()
	_, err = VerifySignedVideoContentURL("task_one", "42", strconv.FormatInt(expiredAt, 10), signVideoContentLink("task_one", 42, expiredAt))
	require.ErrorIs(t, err, ErrVideoContentLinkExpired)
}
