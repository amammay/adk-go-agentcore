package gateway_test

import (
	"context"

	"github.com/amammay/adk-go-agentcore/gateway"
	"github.com/aws/aws-sdk-go-v2/config"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
)

// Showcases how to connect an agent core gateway with iam auth. (over sigv4)
func ExampleNewIAMMCPTransport() {
	cfg, _ := config.LoadDefaultConfig(context.Background())

	mcpToolSet, _ := mcptoolset.New(mcptoolset.Config{
		Transport: gateway.NewIAMMCPTransport("https://example.gateway.bedrock-agentcore.us-east-1.amazonaws.com/mcp", cfg),
	})

	agent, _ := llmagent.New(llmagent.Config{
		Name:        "github_poem_agent",
		Model:       nil, // todo use a model
		Description: "Creates a poem about the last 10 commits from a git repository.",
		Instruction: "Your SOLE purpose is to create a poem about the last 10 commits from a git repository. When given a repository, inspect or retrieve the 10 most recent commits, identify the key changes, themes, and contributors, and turn them into a concise, polished poem. If the request is unrelated to writing a poem about recent git commits, politely refuse.",
		Toolsets:    []tool.Toolset{mcpToolSet},
	})

	_ = agent

}
