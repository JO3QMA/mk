package email

import "net"

// lookupMX is a package-level var for test injection.
var lookupMX = net.LookupMX

// CheckMX returns nil if the domain has at least one MX record.
// DNS 障害時はベストエフォートで通過させる (利用を妨げないため)。
func CheckMX(domain string) error {
	records, err := lookupMX(domain)
	if err != nil {
		return nil
	}
	if len(records) == 0 {
		return ErrMX
	}
	return nil
}
