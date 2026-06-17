package installerassets

import (
	"embed"
	"fmt"
)

//go:embed logo.svg
var embeddedLogo embed.FS

func LogoBytes() ([]byte, error) {
	payload, err := embeddedLogo.ReadFile("logo.svg")
	if err != nil {
		return nil, fmt.Errorf("embedded_logo_not_found: %w", err)
	}

	return payload, nil
}