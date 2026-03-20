package sandbox

import "errors"

var ErrAccessDenied = errors.New("access denied by sandbox")

// FSSandbox controls access to the filesystem.
type FSSandbox interface {
	CheckRead(path string) error
	CheckWrite(path string) error
}

// NetSandbox controls HTTP access.
type NetSandbox interface {
	CheckDomain(domain string) error
}
