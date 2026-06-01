package agentcore

import "fmt"

// ServiceName is the SigV4 service name for Amazon Bedrock AgentCore.
const ServiceName = "bedrock-agentcore"

// RuntimeSessionIDHeader is the AgentCore runtime session ID header.
const RuntimeSessionIDHeader = "X-Amzn-Bedrock-AgentCore-Runtime-Session-Id"

// Endpoint returns the regional AgentCore data plane endpoint.
func Endpoint(region string) string {
	return fmt.Sprintf("https://%s.%s.amazonaws.com", ServiceName, region)
}
