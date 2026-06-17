package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed dist
var dist embed.FS

func DistFS() (fs.FS, error) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("embedded_dist_not_found: %w", err)
	}

	return sub, nil
}
