# adk-go-agentcore

Go helpers for using Google ADK with Amazon Bedrock AgentCore.

This module focuses on the repetitive wiring needed by library consumers:

- SigV4 signing for AgentCore HTTP calls
- IAM-authenticated MCP gateway transport
- IAM-authenticated A2A remote agent providers
- shared AgentCore constants and endpoint helpers

## Installation

```bash
go get github.com/amammay/adk-go-agentcore
```

## Packages

| Package                                               | Purpose                                                              |
|-------------------------------------------------------|----------------------------------------------------------------------|
| `github.com/amammay/adk-go-agentcore`                 | Shared AgentCore constants, headers, and endpoint helpers.           |
| `github.com/amammay/adk-go-agentcore/gateway`         | MCP transport for IAM-authenticated AgentCore Gateway endpoints.     |
| `github.com/amammay/adk-go-agentcore/remoteagentcore` | A2A provider helpers for IAM-authenticated AgentCore Runtime agents. |
| `github.com/amammay/adk-go-agentcore/sigv4transport`  | Low-level SigV4 `http.RoundTripper`.                                 |

## IAM MCP Gateway Transport

Use `gateway.NewIAMMCPTransport` when connecting an ADK MCP toolset to an AgentCore Gateway endpoint that requires IAM auth.

```go
cfg, _ := config.LoadDefaultConfig(context.Background())

mcpToolSet, _ := mcptoolset.New(mcptoolset.Config{
	Transport: gateway.NewIAMMCPTransport(
		"https://example.gateway.bedrock-agentcore.us-east-1.amazonaws.com/mcp",
		cfg,
	),
})

agent, _ := llmagent.New(llmagent.Config{
	Name:     "github_agent",
	Model:    nil,
	Toolsets: []tool.Toolset{mcpToolSet},
})
```

Pass `gateway.WithHTTPClient` to reuse a custom HTTP client. Pass `gateway.WithService` only if you need to override the default AgentCore SigV4 service name.

## IAM A2A Remote Agent Providers

Use `remoteagentcore.NewIAMA2AProviders` when an ADK remote A2A agent points at an AgentCore Runtime ARN.

```go
cfg, _ := config.LoadDefaultConfig(context.Background())

providers, _ := remoteagentcore.NewIAMA2AProviders(agentRuntimeARN, cfg)

remoteAgent, _ := remoteagent.NewA2A(remoteagent.A2AConfig{
	Name:              "A2A summer agent",
	ClientProvider:    providers.ClientProvider(),
	AgentCardProvider: providers.AgentCardProvider(),
})
```

The provider helper handles:

- SigV4 signing with the AWS SDK credentials provider
- AgentCore runtime invocation URL construction
- `X-Amzn-Bedrock-AgentCore-Runtime-Session-Id` propagation from the ADK invocation session
- Agent card parsing for both current A2A cards and AgentCore's v0.3-compatible cards

Use `remoteagentcore.WithEndpoint`, `WithQualifier`, `WithHTTPClient`, or `WithSessionIDProvider` for advanced configuration.

## Low-Level SigV4 Transport

Use `sigv4transport.New` directly when you need an authenticated `http.RoundTripper`.

```go
cfg, _ := config.LoadDefaultConfig(context.Background())

client := &http.Client{
	Transport: sigv4transport.New(http.DefaultTransport, cfg, agentcore.ServiceName),
}
```

Requests with bodies must be replayable. In Go HTTP terms, that means `req.GetBody` must be set unless the request has no body.

## Testing

Run the full test suite with:

```bash
go test ./...
```

Smoke tests that call live AgentCore resources are opt-in and require environment variables such as `GATEWAY_ENDPOINT`.
