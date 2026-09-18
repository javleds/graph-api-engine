package ports

import "context"

type AdmissionPermit interface{ Release(context.Context) error }
