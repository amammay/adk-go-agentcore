package gateway

import (
	"context"
	"iter"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestNewIAMMCPTransportSmoke(t *testing.T) {
	getenv := os.Getenv("GATEWAY_ENDPOINT")
	if getenv == "" {
		t.Skip("GATEWAY_ENDPOINT not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	require.NoError(t, err)

	transport := NewIAMMCPTransport(getenv, cfg)
	streamable := transport.(*mcp.StreamableClientTransport)

	client := mcp.NewClient(&mcp.Implementation{Name: t.Name(), Version: "dev"}, nil)

	session, err := client.Connect(ctx, streamable, nil)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, session.Close())
	}()

	var tools []*mcp.Tool
	for tool, err := range seq(ctx, session.ListTools) {
		require.NoError(t, err)
		tools = append(tools, tool)
	}
	require.Len(t, tools, 13, "expected to be 13 tools on github/repo/readonly")
}

func seq(ctx context.Context, fn func(ctx context.Context, params *mcp.ListToolsParams) (*mcp.ListToolsResult, error)) iter.Seq2[*mcp.Tool, error] {
	return func(yield func(*mcp.Tool, error) bool) {
		cursor := ""

		for {
			result, err := fn(ctx, &mcp.ListToolsParams{Cursor: cursor})
			if err != nil {
				yield(nil, err)
				return
			}

			for _, tool := range result.Tools {
				if !yield(tool, nil) {
					return
				}
			}
			if result.NextCursor == "" {
				return
			}
			cursor = result.NextCursor
		}
	}
}
