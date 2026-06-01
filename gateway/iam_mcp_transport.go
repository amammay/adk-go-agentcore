package gateway

import (
	"net/http"

	agentcore "github.com/amammay/adk-go-agentcore"
	"github.com/amammay/adk-go-agentcore/sigv4transport"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type iamMcpTransportOptions struct {
	client  *http.Client
	service string
}

// IAMMcpTransportOption configures an IAM MCP transport.
type IAMMcpTransportOption func(*iamMcpTransportOptions)

// WithHTTPClient configures the HTTP client used by the MCP transport.
//
// The client is copied before its transport is wrapped, so the original client
// is not mutated.
func WithHTTPClient(client *http.Client) IAMMcpTransportOption {
	return func(options *iamMcpTransportOptions) {
		if client == nil {
			return
		}
		options.client = client
	}
}

// WithService configures the SigV4 service name used to sign MCP requests.
func WithService(service string) IAMMcpTransportOption {
	return func(options *iamMcpTransportOptions) {
		options.service = service
	}
}

// NewIAMMCPTransport returns a new IAM MCP transport.
func NewIAMMCPTransport(endpoint string, cfg aws.Config, opts ...IAMMcpTransportOption) mcp.Transport {
	options := iamMcpTransportOptions{
		client:  http.DefaultClient,
		service: agentcore.ServiceName,
	}
	for _, opt := range opts {
		opt(&options)
	}

	clientCopy := *options.client
	clientCopy.Transport = sigv4transport.New(options.client.Transport, cfg, options.service)

	return &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: &clientCopy,
	}
}
