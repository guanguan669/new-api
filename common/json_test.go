package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJsonRawMessageToString(t *testing.T) {
	tests := []struct {
		name string
		data json.RawMessage
		want string
	}{
		{
			name: "object",
			data: json.RawMessage(`{"city":"Paris","days":0,"strict":false}`),
			want: `{"city":"Paris","days":0,"strict":false}`,
		},
		{
			name: "string",
			data: json.RawMessage(`"{\"city\":\"Paris\",\"days\":0,\"strict\":false}"`),
			want: `{"city":"Paris","days":0,"strict":false}`,
		},
		{
			name: "null",
			data: json.RawMessage(`null`),
			want: "",
		},
		{
			name: "empty",
			data: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, JsonRawMessageToString(tt.data))
		})
	}
}

func TestMarshalNoHTMLEscapePreservesURLQuerySeparators(t *testing.T) {
	data, err := MarshalNoHTMLEscape(map[string]string{
		"url": "https://api.example.com/v1/videos/task/content?expires=1&signature=abc&user_id=42",
	})
	require.NoError(t, err)
	require.Contains(t, string(data), "&signature=abc&user_id=42")
	require.NotContains(t, string(data), `\u0026`)

	var decoded map[string]string
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, "https://api.example.com/v1/videos/task/content?expires=1&signature=abc&user_id=42", decoded["url"])
}
