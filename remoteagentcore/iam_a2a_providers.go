package remoteagentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"github.com/a2aproject/a2a-go/v2/a2acompat/a2av0"
	agentcore "github.com/amammay/adk-go-agentcore"
	"github.com/amammay/adk-go-agentcore/sigv4transport"
	"github.com/aws/aws-sdk-go-v2/aws"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/remoteagent/v2"
)

type a2aOptions struct {
	client            *http.Client
	endpoint          string
	qualifier         string
	service           string
	sessionIDProvider func(context.Context) (string, error)
}

// A2AOption configures AgentCore A2A providers.
type A2AOption func(*a2aOptions)

// WithHTTPClient configures the HTTP client used for AgentCore A2A requests.
//
// The client is copied before its transport is wrapped, so the original client
// is not mutated.
func WithHTTPClient(client *http.Client) A2AOption {
	return func(options *a2aOptions) {
		if client == nil {
			return
		}
		options.client = client
	}
}

// WithEndpoint configures the AgentCore data plane endpoint.
func WithEndpoint(endpoint string) A2AOption {
	return func(options *a2aOptions) {
		options.endpoint = endpoint
	}
}

// WithQualifier configures the AgentCore runtime qualifier used to resolve the agent card.
func WithQualifier(qualifier string) A2AOption {
	return func(options *a2aOptions) {
		options.qualifier = qualifier
	}
}

// WithService configures the SigV4 service name used to sign AgentCore A2A requests.
func WithService(service string) A2AOption {
	return func(options *a2aOptions) {
		options.service = service
	}
}

// WithSessionIDProvider configures how the AgentCore runtime session ID is resolved.
func WithSessionIDProvider(provider func(context.Context) (string, error)) A2AOption {
	return func(options *a2aOptions) {
		if provider == nil {
			return
		}
		options.sessionIDProvider = provider
	}
}

// A2AProviders holds the ADK providers needed to connect to an AgentCore A2A runtime.
type A2AProviders struct {
	agentCardProvider remoteagent.AgentCardProvider
	clientProvider    remoteagent.A2AClientProvider
}

// AgentCardProvider returns the provider used to resolve the AgentCore runtime agent card.
func (p *A2AProviders) AgentCardProvider() remoteagent.AgentCardProvider {
	return p.agentCardProvider
}

// ClientProvider returns the provider used to create A2A clients for the AgentCore runtime.
func (p *A2AProviders) ClientProvider() remoteagent.A2AClientProvider {
	return p.clientProvider
}

// NewIAMA2AProviders returns ADK providers configured for IAM-authenticated AgentCore A2A.
func NewIAMA2AProviders(runtimeARN string, cfg aws.Config, opts ...A2AOption) (*A2AProviders, error) {
	options := defaultOptions(cfg)
	for _, opt := range opts {
		opt(&options)
	}

	if runtimeARN == "" {
		return nil, errors.New("runtime ARN is required")
	}
	if options.endpoint == "" {
		return nil, errors.New("AgentCore endpoint is required")
	}
	runtimeURL, err := runtimeInvocationURL(options.endpoint, runtimeARN, options.qualifier)
	if err != nil {
		return nil, fmt.Errorf("failed to build runtime invocation URL: %w", err)
	}

	clientCopy := *options.client
	clientCopy.Transport = sigv4transport.New(options.client.Transport, cfg, options.service)
	client := &clientCopy

	// todo (amammay) probably can move this part down into a separate file, that way its a bit easier to follow when adding oauth2 bound client
	resolver := &agentcard.Resolver{
		Client:     client,
		CardParser: agentCoreAgentCardParser,
	}

	return &A2AProviders{
		agentCardProvider: newAgentCardProvider(resolver, runtimeURL, options.sessionIDProvider),
		clientProvider:    newCompatA2AClientProvider(client),
	}, nil
}

func defaultOptions(cfg aws.Config) a2aOptions {
	endpoint := ""
	if cfg.Region != "" {
		endpoint = agentcore.Endpoint(cfg.Region)
	}

	return a2aOptions{
		client:            http.DefaultClient,
		endpoint:          endpoint,
		service:           agentcore.ServiceName,
		sessionIDProvider: runtimeSessionID,
	}
}

func runtimeSessionID(ctx context.Context) (string, error) {
	invocationCtx, ok := ctx.(agent.InvocationContext)
	if !ok {
		return "", fmt.Errorf("expected agent invocation context, got %T", ctx)
	}
	return invocationCtx.Session().ID(), nil
}

func runtimeInvocationURL(endpoint, runtimeARN, qualifier string) (string, error) {
	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	runtimeURL := baseURL.JoinPath("runtimes", url.PathEscape(runtimeARN), "invocations")
	if qualifier == "" {
		return runtimeURL.String(), nil
	}

	values := runtimeURL.Query()
	values.Set("qualifier", qualifier)
	runtimeURL.RawQuery = values.Encode()
	return runtimeURL.String(), nil
}

func newAgentCardProvider(resolver *agentcard.Resolver, baseURL string, sessionIDProvider func(context.Context) (string, error)) remoteagent.AgentCardProvider {
	return func(ctx context.Context) (*a2a.AgentCard, error) {
		sessionID, err := sessionIDProvider(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get session ID: %w", err)
		}

		card, err := resolver.Resolve(ctx, baseURL, agentcard.WithRequestHeader(agentcore.RuntimeSessionIDHeader, sessionID))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve agent card: %w", err)
		}
		return card, nil
	}
}

func agentCoreAgentCardParser(body []byte) (*a2a.AgentCard, error) {
	var version struct {
		ProtocolVersion a2a.ProtocolVersion   `json:"protocolVersion"`
		Preferred       a2a.TransportProtocol `json:"preferredTransport"`
		URL             string                `json:"url"`
	}
	if err := json.Unmarshal(body, &version); err != nil {
		return nil, err
	}

	if strings.HasPrefix(string(version.ProtocolVersion), "0.3") || (version.URL != "" && version.Preferred != "") {
		return a2av0.NewAgentCardParser()(body)
	}

	return agentcard.DefaultCardParser(body)
}

func newCompatA2AClientProvider(client *http.Client) remoteagent.A2AClientProvider {
	return remoteagent.NewA2AClientProvider(
		a2aclient.NewFactory(
			a2aclient.WithCompatTransport(
				a2av0.Version,
				a2a.TransportProtocolJSONRPC,
				a2av0.NewJSONRPCTransportFactory(a2av0.JSONRPCTransportConfig{
					Client: client,
				}),
			),
		),
	)
}
