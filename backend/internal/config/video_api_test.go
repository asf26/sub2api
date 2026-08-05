package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVideoAPIConfigActiveRequiresValidPublicHTTPURL(t *testing.T) {
	base := VideoAPIConfig{Enabled: true}
	require.False(t, base.Active())

	base.PublicBaseURL = "https://video.example.test"
	require.True(t, base.Active())

	base.PublicBaseURL = "https://video.example.test?internal=1"
	require.False(t, base.Active())

	base.PublicBaseURL = "javascript:alert(1)"
	require.False(t, base.Active())
}
