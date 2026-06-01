package sigv4transport

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundTripper(t *testing.T) {
	const requestBody = `{"input":"hello"}`

	received := make(chan *http.Request, 1)
	receivedBody := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		received <- r
		receivedBody <- string(body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	awsCredsProvider := mockCredentials()
	base := http.RoundTripper(http.DefaultTransport.(*http.Transport).Clone())
	tripper := New(base, aws.Config{Credentials: awsCredsProvider, Region: "us-east-1"}, "bedrock-agentcore")

	req, err := http.NewRequest(http.MethodPost, server.URL+"/runtimes", strings.NewReader(requestBody))
	require.NoError(t, err)
	req.Host = "bedrock-agentcore.us-east-1.amazonaws.com"
	req.Header.Set("User-Agent", "custom-client/1.0")
	req.Header.Set("Content-Type", "application/json")

	resp, err := tripper.RoundTrip(req)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	signed := <-received
	assert.JSONEq(t, requestBody, <-receivedBody)

	assert.Equal(t, "bedrock-agentcore.us-east-1.amazonaws.com", signed.Host)

	auth := signed.Header.Get("Authorization")
	require.NotEmpty(t, auth)
	assert.Contains(t, auth, "AWS4-HMAC-SHA256")
	assert.Contains(t, auth, "Credential=MOCK_AWS_ACCESS_KEY/")
	assert.Contains(t, auth, "/us-east-1/bedrock-agentcore/aws4_request")
	assert.Contains(t, auth, "SignedHeaders=")
	assert.Contains(t, auth, "Signature=")

	assert.Equal(t, "MOCK_TOKEN", signed.Header.Get("X-Amz-Security-Token"))
	assert.Contains(t, signed.Header.Get("User-Agent"), "custom-client/1.0")
	assert.Contains(t, signed.Header.Get("User-Agent"), "aws-sdk-go-v2/")
	assert.Equal(t, "application/json", signed.Header.Get("Content-Type"))
	assert.Empty(t, req.Header.Get("Authorization"))
	assert.Equal(t, "custom-client/1.0", req.Header.Get("User-Agent"))
}

func TestRoundTripperInfersServiceAndRegion(t *testing.T) {
	testCases := []struct {
		name          string
		host          string
		path          string
		wantScopePart string
	}{
		{
			name:          "aps workspaces",
			host:          "aps-workspaces.us-west-2.amazonaws.com",
			path:          "/workspaces/ws-123/api/v1/query",
			wantScopePart: "/us-west-2/aps/aws4_request",
		},
		{
			name:          "opensearch",
			host:          "search-example-domain.us-east-2.es.amazonaws.com",
			path:          "/_search",
			wantScopePart: "/us-east-2/es/aws4_request",
		},
		{
			name:          "cloudwatch logs",
			host:          "logs.us-east-1.amazonaws.com",
			path:          "/",
			wantScopePart: "/us-east-1/logs/aws4_request",
		},
		{
			name:          "xray",
			host:          "xray.us-west-1.amazonaws.com",
			path:          "/",
			wantScopePart: "/us-west-1/xray/aws4_request",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			received := make(chan *http.Request, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received <- r
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			base := http.RoundTripper(http.DefaultTransport.(*http.Transport).Clone())
			tripper := New(base, aws.Config{Credentials: mockCredentials()}, "")

			req, err := http.NewRequest(http.MethodGet, server.URL+tc.path, nil)
			require.NoError(t, err)
			req.Host = tc.host

			resp, err := tripper.RoundTrip(req)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, resp.Body.Close())
			}()

			assert.Equal(t, http.StatusAccepted, resp.StatusCode)

			signed := <-received
			assert.Equal(t, tc.host, signed.Host)

			auth := signed.Header.Get("Authorization")
			require.NotEmpty(t, auth)
			assert.Contains(t, auth, tc.wantScopePart)
		})
	}
}

func TestRoundTripperRequiresRequest(t *testing.T) {
	base := http.RoundTripper(http.DefaultTransport.(*http.Transport).Clone())
	tripper := New(base, aws.Config{Credentials: mockCredentials(), Region: "us-east-1"}, "bedrock-agentcore")

	resp, err := tripper.RoundTrip(nil)
	if resp != nil {
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
	}
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "request is nil")
}

func TestRoundTripperRequiresCredentialsProvider(t *testing.T) {
	base := http.RoundTripper(http.DefaultTransport.(*http.Transport).Clone())
	tripper := New(base, aws.Config{Region: "us-east-1"}, "bedrock-agentcore")

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/runtimes", nil)
	require.NoError(t, err)

	resp, err := tripper.RoundTrip(req)
	if resp != nil {
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
	}
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "credentials provider is not set")
}

func TestRoundTripperRequiresReplayableBody(t *testing.T) {
	base := http.RoundTripper(http.DefaultTransport.(*http.Transport).Clone())
	tripper := New(base, aws.Config{Credentials: mockCredentials(), Region: "us-east-1"}, "bedrock-agentcore")

	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1/runtimes", io.NopCloser(strings.NewReader(`{"input":"hello"}`)))
	require.NoError(t, err)
	require.Nil(t, req.GetBody)

	resp, err := tripper.RoundTrip(req)
	if resp != nil {
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
	}
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "GetBody is not set")
}

func mockCredentials() aws.CredentialsProvider {
	provider := credentials.NewStaticCredentialsProvider("MOCK_AWS_ACCESS_KEY", "MOCK_AWS_SECRET_ACCESS_KEY", "MOCK_TOKEN")
	return aws.NewCredentialsCache(provider)
}
