package application

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"net"
	"syscall"
)

func classify(response domain.RawResponse, err error) *domain.UpstreamFailure {
	if err != nil {
		return classifyTransport(err)
	}
	status := response.StatusCode
	failure := &domain.UpstreamFailure{HTTPStatus: status, Code: "UPSTREAM_HTTP_ERROR"}
	switch {
	case status >= 200 && status < 300:
		if status == 204 || !json.Valid(response.Body) {
			failure.Code = "UPSTREAM_INVALID_JSON"
			return failure
		}
		return nil
	case status >= 300 && status < 400:
		failure.Code = "UPSTREAM_REDIRECT_NOT_ALLOWED"
	case status == 400:
		failure.Code = "UPSTREAM_BAD_REQUEST"
	case status == 401:
		failure.Code = "UPSTREAM_UNAUTHORIZED"
	case status == 403:
		failure.Code = "UPSTREAM_FORBIDDEN"
	case status == 404:
		failure.Code = "UPSTREAM_NOT_FOUND"
	case status == 422:
		failure.Code = "UPSTREAM_UNPROCESSABLE_ENTITY"
	case status == 408:
		failure.Code = "UPSTREAM_TIMEOUT"
		failure.Retryable = true
	case status == 429:
		failure.Code = "UPSTREAM_RATE_LIMITED"
		failure.Retryable = true
	case status == 500 || status == 502 || status == 503 || status == 504:
		failure.Code = "UPSTREAM_UNAVAILABLE"
		failure.Retryable = true
	}
	return failure
}

func classifyTransport(err error) *domain.UpstreamFailure {
	var failure *domain.UpstreamFailure
	if errors.As(err, &failure) {
		copy := *failure
		return &copy
	}
	var cert *tls.CertificateVerificationError
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var record tls.RecordHeaderError
	if errors.As(err, &cert) || errors.As(err, &authority) || errors.As(err, &hostname) || errors.As(err, &record) {
		return &domain.UpstreamFailure{Code: "UPSTREAM_TLS_ERROR"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &domain.UpstreamFailure{Code: "UPSTREAM_TIMEOUT", Retryable: true}
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return &domain.UpstreamFailure{Code: "UPSTREAM_TIMEOUT", Retryable: true}
	}
	var dns *net.DNSError
	retryable := errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EPIPE)
	if errors.As(err, &dns) {
		retryable = dns.IsTemporary || dns.IsTimeout
	}
	return &domain.UpstreamFailure{Code: "UPSTREAM_CONNECTION_ERROR", Retryable: retryable}
}
