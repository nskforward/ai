package sandbox

import "strings"

type DefaultNetSandbox struct {
	allowedDomains []string
}

// NewNetSandbox creates a sandbox that only permits checking domains
// exactly matching or being a subdomain of the allowed domains.
func NewNetSandbox(domains []string) *DefaultNetSandbox {
	var clean []string
	for _, d := range domains {
		clean = append(clean, strings.ToLower(d))
	}
	return &DefaultNetSandbox{allowedDomains: clean}
}

func (s *DefaultNetSandbox) CheckDomain(domain string) error {
	lower := strings.ToLower(domain)
	for _, d := range s.allowedDomains {
		if lower == d || strings.HasSuffix(lower, "."+d) {
			return nil
		}
	}
	return ErrAccessDenied
}
