package domain

import "errors"

var ErrV2Limit = errors.New("v2 durable storage or execution limit exceeded")
var ErrV2Cursor = errors.New("invalid v2 cursor")
var ErrV2ValueTooLarge = errors.New("v2 fragment exceeds 256 KiB")
