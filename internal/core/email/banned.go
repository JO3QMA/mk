package email

import "strings"

// IsBannedDomain reports whether domain (already lowercased) matches any
// entry in bannedDomains. Uses subdomain matching to mirror the original
// Misskey's UtilityService.isBlockedHost:
//
//	`.${host}`.endsWith(`.${blocked}`)
//
// This blocks both exact match (example.com) and subdomains
// (mail.example.com) when "example.com" is in the banned list.
func IsBannedDomain(domain string, bannedDomains []string) bool {
	if domain == "" {
		return false
	}
	suffix := "." + domain
	for _, b := range bannedDomains {
		b = strings.TrimSpace(strings.ToLower(b))
		if b == "" {
			continue
		}
		if strings.HasSuffix(suffix, "."+b) {
			return true
		}
	}
	return false
}
