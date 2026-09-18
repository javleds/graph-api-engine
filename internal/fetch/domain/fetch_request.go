package domain

import "context"

type FetchRequest struct {
	BeforeAttempt      func(context.Context) error
	OnAttempt          func()
	Request            OutboundRequest
	OwnerID            string
	CredentialIdentity string
	EndpointTemplate   string
	Admission          AdmissionRequest
	Policy             FetchPolicy
}
