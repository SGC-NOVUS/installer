package orchestrator

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

// validateInstallHost enforces that the install target is either a valid DNS
// hostname or an IP address. The domain is interpolated into shell commands,
// nginx config and filesystem paths, so rejecting metacharacters here closes
// the shell/path/config-injection surface at the boundary.
func validateInstallHost(value string) error {
	host := strings.TrimSpace(value)
	if strings.Contains(host, "://") {
		if parsed, err := url.Parse(host); err == nil && parsed.Hostname() != "" {
			host = parsed.Hostname()
		}
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")

	if host == "" {
		return fmt.Errorf("setup_request_invalid:domain_required")
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if len(host) > 253 {
		return fmt.Errorf("setup_request_invalid:domain_invalid")
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return fmt.Errorf("setup_request_invalid:domain_invalid")
	}
	for _, label := range labels {
		if !domainLabelPattern.MatchString(label) {
			return fmt.Errorf("setup_request_invalid:domain_invalid")
		}
	}
	return nil
}

// validateAdminEmail enforces RFC 5322 addr-spec form and rejects shell/control
// characters before the value reaches certbot/panel bootstrap commands.
func validateAdminEmail(value string) error {
	email := strings.TrimSpace(value)
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return fmt.Errorf("setup_request_invalid:admin_email_invalid")
	}
	if strings.ContainsAny(email, " \t\r\n'\"`$;&|<>()\\") {
		return fmt.Errorf("setup_request_invalid:admin_email_invalid")
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || !strings.Contains(email[at+1:], ".") {
		return fmt.Errorf("setup_request_invalid:admin_email_invalid")
	}
	return nil
}

// validatePasswordStrength enforces a minimum strength policy for credentials
// that are later embedded into bootstrap SQL and panel provisioning. The policy
// requires at least 12 characters spanning three of four character classes
// (lowercase, uppercase, digit, symbol) and rejects control characters that
// could corrupt downstream command rendering.
func validatePasswordStrength(label string, value string) error {
	if strings.ContainsAny(value, "\x00\r\n") {
		return fmt.Errorf("setup_request_invalid:%s_invalid", label)
	}
	if len([]rune(value)) < 12 {
		return fmt.Errorf("setup_request_invalid:%s_too_weak", label)
	}

	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSymbol = true
		}
	}

	classes := 0
	for _, present := range []bool{hasLower, hasUpper, hasDigit, hasSymbol} {
		if present {
			classes++
		}
	}
	if classes < 3 {
		return fmt.Errorf("setup_request_invalid:%s_too_weak", label)
	}
	return nil
}

// validateCustomTLSMaterial verifies that the operator-supplied certificate and
// private key are well-formed PEM, parse as a usable x509 keypair, and that the
// leaf certificate is not already expired.
func validateCustomTLSMaterial(certPEM string, keyPEM string) error {
	cert := []byte(strings.TrimSpace(certPEM) + "\n")
	key := []byte(strings.TrimSpace(keyPEM) + "\n")

	block, _ := pem.Decode(cert)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("setup_request_invalid:custom_certificate_invalid")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("setup_request_invalid:custom_certificate_invalid")
	}

	keyBlock, _ := pem.Decode(key)
	if keyBlock == nil || !strings.Contains(keyBlock.Type, "PRIVATE KEY") {
		return fmt.Errorf("setup_request_invalid:custom_private_key_invalid")
	}

	if _, err := tls.X509KeyPair(cert, key); err != nil {
		return fmt.Errorf("setup_request_invalid:custom_certificate_key_mismatch")
	}

	if now := time.Now(); now.After(leaf.NotAfter) {
		return fmt.Errorf("setup_request_invalid:custom_certificate_expired")
	}
	return nil
}

// validateSecurityEntranceConfig validates the Security Entrance configuration
// fields: path format, reserved segments, port range, and rate-limit parameters.
func validateSecurityEntranceConfig(cfg SecurityEntranceConfig) error {
	path := strings.Trim(strings.ToLower(cfg.Path), " /")
	if path == "" {
		return fmt.Errorf("setup_request_invalid:security_entrance_path_required")
	}
	if _, reserved := securityEntranceReservedSegments[path]; reserved {
		return fmt.Errorf("setup_request_invalid:security_entrance_path_reserved")
	}
	if len(path) < 3 || len(path) > 64 {
		return fmt.Errorf("setup_request_invalid:security_entrance_path_invalid")
	}
	for _, symbol := range path {
		if (symbol < 'a' || symbol > 'z') && (symbol < '0' || symbol > '9') && symbol != '-' && symbol != '_' {
			return fmt.Errorf("setup_request_invalid:security_entrance_path_invalid")
		}
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("setup_request_invalid:security_entrance_port_invalid")
	}
	if cfg.WindowSeconds < 0 || cfg.WindowSeconds > 86400 {
		return fmt.Errorf("setup_request_invalid:security_entrance_window_invalid")
	}
	if cfg.MaxAttempts < 0 || cfg.MaxAttempts > 1000 {
		return fmt.Errorf("setup_request_invalid:security_entrance_attempts_invalid")
	}
	if cfg.BlockSeconds < 0 || cfg.BlockSeconds > 604800 {
		return fmt.Errorf("setup_request_invalid:security_entrance_block_invalid")
	}

	return nil
}
