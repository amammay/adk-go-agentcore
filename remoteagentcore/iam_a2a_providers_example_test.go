package remoteagentcore_test

import (
	"context"

	"github.com/amammay/adk-go-agentcore/remoteagentcore"
	"github.com/aws/aws-sdk-go-v2/config"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/remoteagent/v2"
	"google.golang.org/adk/cmd/launcher"
)

const agentRuntimeARN = "arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime/example"

func ExampleNewIAMA2AProviders() {
	ctx := context.Background()

	cfg, _ := config.LoadDefaultConfig(ctx)

	providers, _ := remoteagentcore.NewIAMA2AProviders(agentRuntimeARN, cfg)

	remoteAgent, _ := remoteagent.NewA2A(remoteagent.A2AConfig{
		Name:              "A2A summer agent",
		ClientProvider:    providers.ClientProvider(),
		AgentCardProvider: providers.AgentCardProvider(),
	})

	launchCfg := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(remoteAgent),
	}

	_ = launchCfg
}
