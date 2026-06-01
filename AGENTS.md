# Repository Guidelines

## Project Structure & Module Organization

This repository is a Go module for Amazon Bedrock AgentCore helpers.

- `agentcore.go` contains shared AgentCore constants and endpoint helpers.
- `sigv4transport/` provides an `http.RoundTripper` that signs requests with SigV4.
- `gateway/` contains IAM-authenticated MCP transport helpers.
- `remoteagentcore/` contains IAM-authenticated A2A provider helpers for ADK remote agents.
- Tests live next to source files as `*_test.go`; examples use Go example tests such as `ExampleNewIAMMCPTransport`.

Keep new packages narrow and named around their public API. Prefer colocating tests with the package they cover.

## Build, Test, and Development Commands

- `go test ./...` runs the full test suite.
- `go test ./sigv4transport` runs only SigV4 transport tests.
- `go test ./gateway` runs gateway transport tests.
- `go test ./remoteagentcore` runs remote AgentCore A2A provider tests.
- `go test -run Example ./...` compiles and runs Go examples with expected output, if any.
- `go mod tidy` updates `go.mod` and `go.sum` after import changes.
- `gofmt -w <files>` formats edited Go files.

Run `go test ./...` before handing off changes.

## Coding Style & Naming Conventions

Use standard Go style: tabs from `gofmt`, concise exported comments, and small package APIs. Exported identifiers should describe the integration clearly, for example `NewIAMMCPTransport` and `NewIAMA2AProviders`.

Prefer option functions for configurable constructors, following existing patterns like `WithHTTPClient`, `WithService`, `WithEndpoint`, and `WithSessionIDProvider`. Avoid duplicating AgentCore constants; use the root `agentcore` package for shared names such as `ServiceName`.

## Testing Guidelines

Tests use Go’s `testing` package with `testify/assert` and `testify/require`. Name tests after the behavior under test, for example `TestRoundTripperRequiresReplayableBody`.

Use `httptest` for HTTP behavior and avoid real AWS calls in unit tests. Smoke tests that require live resources must be skipped unless required environment variables are set, as in `gateway/iam_mcp_transport_smoke_test.go`.

## Commit & Pull Request Guidelines

The repository has minimal history, so use short imperative commit messages, for example `Add IAM A2A provider helper`. Keep commits focused and avoid mixing formatting, dependency churn, and behavior changes unless they are directly related.

Pull requests should include a brief summary, tests run, and any configuration needed for smoke tests. Link issues when available and call out API renames or behavior changes explicitly.

## Security & Configuration Tips

Do not commit credentials, real session IDs, or private runtime ARNs. Use AWS SDK configuration loading in examples and tests, and keep live AWS tests opt-in through environment variables.
