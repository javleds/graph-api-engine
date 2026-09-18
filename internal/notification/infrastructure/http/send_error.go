package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	fetch "github.com/javleds/api-engine/internal/fetch/domain"
	"github.com/javleds/api-engine/internal/notification/domain"
	"net"
	"syscall"
)

func classifySendError(err error) domain.WebhookAttempt {
	var denied *fetch.UpstreamFailure
	if errors.As(err, &denied) && denied.Code == "EGRESS_DENIED" {
		return domain.WebhookAttempt{ErrorCode: "EGRESS_DENIED"}
	}
	var cert *tls.CertificateVerificationError
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var record tls.RecordHeaderError
	if errors.As(err, &cert) || errors.As(err, &authority) || errors.As(err, &hostname) || errors.As(err, &record) {
		return domain.WebhookAttempt{ErrorCode: "WEBHOOK_TLS_ERROR"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.WebhookAttempt{ErrorCode: "WEBHOOK_TIMEOUT", Retryable: true}
	}
	if errors.Is(err, context.Canceled) {
		return domain.WebhookAttempt{ErrorCode: "WEBHOOK_INTERRUPTED", Retryable: true}
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return domain.WebhookAttempt{ErrorCode: "WEBHOOK_TIMEOUT", Retryable: true}
	}
	var dns *net.DNSError
	retryable := errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EPIPE)
	if errors.As(err, &dns) {
		retryable = dns.IsTemporary || dns.IsTimeout
	}
	return domain.WebhookAttempt{ErrorCode: "WEBHOOK_CONNECTION_ERROR", Retryable: retryable}
}
