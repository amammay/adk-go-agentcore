package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIAMMcpTransport(t *testing.T) {
	client := &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}
	baseTransport := client.Transport
	clientTimeout := client.Timeout
	endpoint := "https://example.com/mcp"

	transport := NewIAMMCPTransport(endpoint, mockAWSConfig(), WithHTTPClient(client))

	streamable, ok := transport.(*mcp.StreamableClientTransport)
	require.True(t, ok)
	assert.Equal(t, endpoint, streamable.Endpoint)
	require.NotNil(t, streamable.HTTPClient)
	assert.NotSame(t, client, streamable.HTTPClient)
	assert.NotSame(t, baseTransport, streamable.HTTPClient.Transport)
	assert.Same(t, baseTransport, client.Transport)
	assert.Equal(t, clientTimeout, streamable.HTTPClient.Timeout)
}

func TestNewIAMMcpTransportSignsRequests(t *testing.T) {
	received := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}
	transport := NewIAMMCPTransport(server.URL+"/mcp", mockAWSConfig(), WithHTTPClient(client))
	streamable := transport.(*mcp.StreamableClientTransport)

	req, err := http.NewRequest(http.MethodPost, streamable.Endpoint, nil)
	require.NoError(t, err)

	resp, err := streamable.HTTPClient.Do(req)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	signed := <-received
	auth := signed.Header.Get("Authorization")
	require.NotEmpty(t, auth)
	assert.Contains(t, auth, "AWS4-HMAC-SHA256")
	assert.Contains(t, auth, "Credential=MOCK_AWS_ACCESS_KEY/")
	assert.Contains(t, auth, "/us-east-1/bedrock-agentcore/aws4_request")
	assert.Contains(t, auth, "SignedHeaders=")
	assert.Contains(t, auth, "Signature=")
	assert.Equal(t, "MOCK_TOKEN", signed.Header.Get("X-Amz-Security-Token"))
}

func TestNewIAMMcpTransportUsesDefaultBaseTransport(t *testing.T) {
	received := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &http.Client{}
	transport := NewIAMMCPTransport(server.URL+"/mcp", mockAWSConfig(), WithHTTPClient(client))
	streamable := transport.(*mcp.StreamableClientTransport)

	resp, err := streamable.HTTPClient.Get(streamable.Endpoint)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	signed := <-received
	assert.NotEmpty(t, signed.Header.Get("Authorization"))
}

func TestNewIAMMcpTransportUsesDefaultClient(t *testing.T) {
	received := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	transport := NewIAMMCPTransport(server.URL+"/mcp", mockAWSConfig())
	streamable := transport.(*mcp.StreamableClientTransport)
	require.NotNil(t, streamable.HTTPClient)

	resp, err := streamable.HTTPClient.Get(streamable.Endpoint)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	signed := <-received
	assert.NotEmpty(t, signed.Header.Get("Authorization"))
}

func TestNewIAMMcpTransportUsesCustomService(t *testing.T) {
	received := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	transport := NewIAMMCPTransport(server.URL+"/mcp", mockAWSConfig(), WithService("execute-api"))
	streamable := transport.(*mcp.StreamableClientTransport)

	resp, err := streamable.HTTPClient.Get(streamable.Endpoint)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	signed := <-received
	auth := signed.Header.Get("Authorization")
	require.NotEmpty(t, auth)
	assert.Contains(t, auth, "/us-east-1/execute-api/aws4_request")
}

func TestNewIAMMcpTransportIgnoresNilHTTPClientOption(t *testing.T) {
	received := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	transport := NewIAMMCPTransport(server.URL+"/mcp", mockAWSConfig(), WithHTTPClient(nil))
	streamable := transport.(*mcp.StreamableClientTransport)
	require.NotNil(t, streamable.HTTPClient)

	resp, err := streamable.HTTPClient.Get(streamable.Endpoint)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	signed := <-received
	assert.NotEmpty(t, signed.Header.Get("Authorization"))
}

func mockAWSConfig() aws.Config {
	provider := credentials.NewStaticCredentialsProvider("MOCK_AWS_ACCESS_KEY", "MOCK_AWS_SECRET_ACCESS_KEY", "MOCK_TOKEN")
	return aws.Config{
		Region:      "us-east-1",
		Credentials: aws.NewCredentialsCache(provider),
	}
}
