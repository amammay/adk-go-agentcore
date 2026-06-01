// Package sigv4transport is an RoundTripper that signs requests with SigV4.
// inspired from https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/6698bc24dc8ee69f839f16eb9950b99b074f8191/extension/sigv4authextension/extension.go#L45
package sigv4transport

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/aws-http-auth/credentials"
	"github.com/aws/smithy-go/aws-http-auth/sigv4"
	v4 "github.com/aws/smithy-go/aws-http-auth/v4"
)

// New returns a new SigV4RoundTripper that signs requests with SigV4.
func New(base http.RoundTripper, cfg aws.Config, serviceName string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	return &signingRoundTripper{
		base:          base,
		signer:        sigv4.New(),
		region:        cfg.Region,
		service:       serviceName,
		credsProvider: cfg.Credentials,
		awsSDKInfo:    fmt.Sprintf("%s/%s", aws.SDKName, aws.SDKVersion),
	}
}

type signingRoundTripper struct {
	base          http.RoundTripper
	signer        *sigv4.Signer
	region        string
	service       string
	credsProvider aws.CredentialsProvider
	awsSDKInfo    string
}

func (si *signingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("sigv4tripper: request is nil")
	}

	req2, err := si.signRequest(req)
	if err != nil {
		return nil, err
	}

	return si.base.RoundTrip(req2)
}

func (si *signingRoundTripper) signRequest(req *http.Request) (*http.Request, error) {
	payloadHash, err := hashPayload(req)
	if err != nil {
		return nil, fmt.Errorf("unable to hash request body: %w", err)
	}

	// Clone request to ensure thread safety.
	req2 := cloneRequest(req)

	// Add the runtime information to the User-Agent header of the request
	ua := req2.Header.Get("User-Agent")
	if ua != "" {
		ua = ua + " " + si.awsSDKInfo
	} else {
		ua = si.awsSDKInfo
	}
	req2.Header.Set("User-Agent", ua)

	// Use user provided service/region if specified, use inferred service/region if not, then sign the request
	service, region := si.inferServiceAndRegion(req2)
	if si.credsProvider == nil {
		return nil, errors.New("a credentials provider is not set")
	}
	creds, err := si.credsProvider.Retrieve(req2.Context())
	if err != nil {
		return nil, fmt.Errorf("error retrieving credentials: %w", err)
	}

	err = si.signer.SignRequest(&sigv4.SignRequestInput{
		Request:     req2,
		PayloadHash: payloadHash,
		Credentials: credentials.Credentials{
			AccessKeyID:     creds.AccessKeyID,
			SecretAccessKey: creds.SecretAccessKey,
			SessionToken:    creds.SessionToken,
			Expires:         creds.Expires,
		},
		Service:       service,
		Region:        region,
		Time:          time.Now(),
		SignatureType: v4.SignatureTypeHeader,
	})
	if err != nil {
		return nil, fmt.Errorf("error signing the request: %w", err)
	}

	return req2, nil
}

// hashPayload creates a SHA256 hash of the request body
func hashPayload(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		sum := sha256.Sum256(nil)
		return sum[:], nil
	}

	if req.GetBody == nil {
		return nil, errors.New("request body cannot be hashed because GetBody is not set")
	}

	reqBody, err := req.GetBody()
	if err != nil {
		return nil, err
	}

	// Hash the request body
	h := sha256.New()
	_, err = io.Copy(h, reqBody)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), reqBody.Close()
}

// inferServiceAndRegion attempts to infer a service
// and a region from an http.request, and returns either an empty
// string for both or a valid value for both.
func (si *signingRoundTripper) inferServiceAndRegion(r *http.Request) (service, region string) {
	service = si.service
	region = si.region

	// if service and region are already set, return them
	if service != "" && region != "" {
		return service, region
	}

	// infer service and region from host if not set
	host := r.Host
	switch {
	case strings.HasPrefix(host, "aps-workspaces"):
		service, region = extractServiceAndRegion(service, region, host, "aps")
	case strings.HasPrefix(host, "search-"):
		service, region = extractServiceAndRegion(service, region, host, "es")
	case strings.HasPrefix(host, "logs"):
		service, region = extractServiceAndRegion(service, region, host, "logs")
	case strings.HasPrefix(host, "xray"):
		service, region = extractServiceAndRegion(service, region, host, "xray")
	}

	return service, region
}

func extractServiceAndRegion(service, region, host, defaultService string) (string, string) {
	if service == "" {
		service = defaultService
	}
	rest := host[strings.Index(host, ".")+1:]
	if region == "" {
		region = rest[0:strings.Index(rest, ".")]
	}
	return service, region
}

// cloneRequest() is a helper function that makes a shallow copy of the request and a
// deep copy of the header, for thread safety purposes.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}
