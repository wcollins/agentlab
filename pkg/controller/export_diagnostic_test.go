package controller

import (
	"io"
	"os"
	"testing"

	"github.com/gridctl/gridctl/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestCreatePrinter_ExcludesResolvedCredentials(t *testing.T) {
	sc := &StackController{}
	sc.config.Verbose = true
	const canary = "synthetic-diagnostic-canary"
	stack := &config.Stack{
		Name:    canary,
		Gateway: &config.GatewayConfig{Auth: &config.AuthConfig{Token: canary}, TokenizerAPIKey: canary},
		MCPServers: []config.MCPServer{{Name: canary, URL: canary, Command: []string{canary}, Transport: canary,
			Auth: &config.ServerAuth{ClientSecret: canary, Value: canary, Token: canary}}},
	}
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	defer f.Close()
	old := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = old }()
	sc.createPrinter(stack)
	_, err = f.Seek(0, 0)
	require.NoError(t, err)
	data, err := io.ReadAll(f)
	require.NoError(t, err)
	require.NotContains(t, string(data), "synthetic-diagnostic-canary")
	require.Contains(t, string(data), "Stack summary")
	require.Contains(t, string(data), "transport=other")
	require.Equal(t, canary, stack.MCPServers[0].Auth.ClientSecret)
}
