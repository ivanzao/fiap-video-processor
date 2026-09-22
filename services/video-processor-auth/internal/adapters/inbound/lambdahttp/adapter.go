package lambdahttp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

type Handler func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)

func Wrap(h http.Handler) Handler {
	return func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		r, err := toRequest(ctx, req)
		if err != nil {
			return events.APIGatewayV2HTTPResponse{}, err
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return toResponse(rec), nil
	}
}

func toRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	body := req.Body
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return nil, fmt.Errorf("lambdahttp: decode body: %w", err)
		}
		body = string(decoded)
	}
	target := req.RawPath
	if target == "" {
		target = "/"
	}
	if req.RawQueryString != "" {
		target += "?" + req.RawQueryString
	}
	r, err := http.NewRequestWithContext(ctx, req.RequestContext.HTTP.Method, target, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("lambdahttp: build request: %w", err)
	}
	for name, value := range req.Headers {
		r.Header.Set(name, value)
	}
	r.RemoteAddr = req.RequestContext.HTTP.SourceIP
	return r, nil
}

func toResponse(rec *httptest.ResponseRecorder) events.APIGatewayV2HTTPResponse {
	headers := make(map[string]string, len(rec.Header()))
	for name, values := range rec.Header() {
		headers[name] = strings.Join(values, ",")
	}
	return events.APIGatewayV2HTTPResponse{StatusCode: rec.Code, Headers: headers, Body: rec.Body.String()}
}
