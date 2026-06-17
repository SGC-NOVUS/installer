package orchestrator

import "testing"

func TestValidateInstallHost(t *testing.T) {
	valid := []string{
		"example.com",
		"panel.novus.fun",
		"a.b.c.example.co.uk",
		"192.168.1.10",
		"2001:db8::1",
		"https://panel.example.com",
		"[2001:db8::1]",
	}
	for _, host := range valid {
		if err := validateInstallHost(host); err != nil {
			t.Errorf("expected %q valid, got %v", host, err)
		}
	}

	invalid := []string{
		"",
		"localhost",
		"example.com; rm -rf /",
		"exa mple.com",
		"example.com && curl evil",
		"$(whoami).com",
		"-bad.example.com",
		"bad-.example.com",
		"a..b.com",
	}
	for _, host := range invalid {
		if err := validateInstallHost(host); err == nil {
			t.Errorf("expected %q invalid", host)
		}
	}
}

func TestValidateAdminEmail(t *testing.T) {
	valid := []string{
		"admin@example.com",
		"user.name+tag@sub.example.co",
	}
	for _, email := range valid {
		if err := validateAdminEmail(email); err != nil {
			t.Errorf("expected %q valid, got %v", email, err)
		}
	}

	invalid := []string{
		"",
		"not-an-email",
		"a@b",
		"admin@example.com; rm -rf /",
		"\"a b\"@example.com",
		"admin@example.com\nBcc: evil@x.com",
		"admin@example.com`id`",
	}
	for _, email := range invalid {
		if err := validateAdminEmail(email); err == nil {
			t.Errorf("expected %q invalid", email)
		}
	}
}

func TestEscapeSQLLiteral(t *testing.T) {
	cases := map[string]string{
		"plain":     "plain",
		"o'reilly":  "o''reilly",
		`back\slash`: `back\\slash`,
		`\' inject`:  `\\'' inject`,
	}
	for in, want := range cases {
		if got := escapeSQLLiteral(in); got != want {
			t.Errorf("escapeSQLLiteral(%q) = %q, want %q", in, got, want)
		}
	}
}
