package remoteagentcore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	agentcore "github.com/amammay/adk-go-agentcore"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIAMA2AProviders(t *testing.T) {
	client := &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}
	baseTransport := client.Transport

	providers, err := NewIAMA2AProviders(
		"arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime/example",
		aws.Config{Region: "us-east-1"},
		WithHTTPClient(client),
	)
	require.NoError(t, err)

	assert.NotNil(t, providers.AgentCardProvider())
	assert.NotNil(t, providers.ClientProvider())
	assert.Same(t, baseTransport, client.Transport)
}

func TestNewIAMA2AProvidersRequiresRuntimeARN(t *testing.T) {
	providers, err := NewIAMA2AProviders("", aws.Config{Region: "us-east-1"})

	require.Error(t, err)
	assert.Nil(t, providers)
	assert.Contains(t, err.Error(), "runtime ARN is required")
}

func TestNewIAMA2AProvidersRequiresEndpoint(t *testing.T) {
	providers, err := NewIAMA2AProviders("arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime/example", aws.Config{})

	require.Error(t, err)
	assert.Nil(t, providers)
	assert.Contains(t, err.Error(), "endpoint is required")
}

func TestRuntimeInvocationURL(t *testing.T) {
	got, err := runtimeInvocationURL(
		"https://bedrock-agentcore.us-east-1.amazonaws.com/",
		"arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime/example",
		"prod",
	)
	require.NoError(t, err)

	assert.Equal(t, "https://bedrock-agentcore.us-east-1.amazonaws.com/runtimes/arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime%2Fexample/invocations?qualifier=prod", got)
}

func TestAgentCardProviderResolvesCompatCardWithSessionID(t *testing.T) {
	received := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"name": "remote",
			"description": "Remote agent",
			"version": "dev",
			"url": "https://example.com/a2a",
			"protocolVersion": "0.3.0",
			"preferredTransport": "JSONRPC",
			"capabilities": {},
			"defaultInputModes": ["text"],
			"defaultOutputModes": ["text"],
			"skills": [
				{
					"id": "answer",
					"name": "Answer",
					"description": "Answers questions",
					"tags": ["test"]
				}
			]
		}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	resolver := &agentcard.Resolver{
		Client:     server.Client(),
		CardParser: agentCoreAgentCardParser,
	}
	provider := newAgentCardProvider(
		resolver,
		mustRuntimeInvocationURL(t, server.URL, "arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime/example", "prod"),
		func(context.Context) (string, error) {
			return "session-123", nil
		},
	)

	card, err := provider(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "remote", card.Name)

	req := <-received
	assert.Equal(t, "/runtimes/arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime%2Fexample/invocations/.well-known/agent-card.json", req.URL.EscapedPath())
	assert.Equal(t, "prod", req.URL.Query().Get("qualifier"))
	assert.Equal(t, "session-123", req.Header.Get(agentcore.RuntimeSessionIDHeader))
}

func TestAgentCoreAgentCardParserParsesV1Card(t *testing.T) {
	card, err := agentCoreAgentCardParser([]byte(`{
		"name": "remote-v1",
		"description": "Remote agent",
		"version": "dev",
		"capabilities": {},
		"defaultInputModes": ["text"],
		"defaultOutputModes": ["text"],
		"supportedInterfaces": [
			{
				"url": "https://example.com/a2a",
				"protocolBinding": "JSONRPC",
				"protocolVersion": "1.0"
			}
		],
		"skills": [
			{
				"id": "answer",
				"name": "Answer",
				"description": "Answers questions",
				"tags": ["test"]
			}
		]
	}`))
	require.NoError(t, err)

	require.Len(t, card.SupportedInterfaces, 1)
	assert.Equal(t, "remote-v1", card.Name)
	assert.Equal(t, a2a.Version, card.SupportedInterfaces[0].ProtocolVersion)
}

func TestAgentCoreAgentCardParserParsesV03Card(t *testing.T) {
	card, err := agentCoreAgentCardParser([]byte(`{
		"name": "remote-v03",
		"description": "Remote agent",
		"version": "dev",
		"url": "https://example.com/a2a",
		"protocolVersion": "0.3.0",
		"preferredTransport": "JSONRPC",
		"capabilities": {},
		"defaultInputModes": ["text"],
		"defaultOutputModes": ["text"],
		"skills": [
			{
				"id": "answer",
				"name": "Answer",
				"description": "Answers questions",
				"tags": ["test"]
			}
		]
	}`))
	require.NoError(t, err)

	require.Len(t, card.SupportedInterfaces, 1)
	assert.Equal(t, "remote-v03", card.Name)
	assert.Equal(t, a2a.ProtocolVersion("0.3.0"), card.SupportedInterfaces[0].ProtocolVersion)
}

func TestRuntimeSessionIDRequiresInvocationContext(t *testing.T) {
	sessionID, err := runtimeSessionID(context.Background())

	require.Error(t, err)
	assert.Empty(t, sessionID)
	assert.Contains(t, err.Error(), "expected agent invocation context")
}

func mustRuntimeInvocationURL(t *testing.T, endpoint, runtimeARN, qualifier string) string {
	t.Helper()

	runtimeURL, err := runtimeInvocationURL(endpoint, runtimeARN, qualifier)
	require.NoError(t, err)
	return runtimeURL
}
