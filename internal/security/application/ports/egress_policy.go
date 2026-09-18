package ports

import "net/netip"

type EgressPolicy interface {
	ValidateURL(string) error
	ValidateAddresses([]netip.Addr) error
}
