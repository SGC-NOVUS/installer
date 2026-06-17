package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed locales/*.json
var localeFiles embed.FS

func (s *Server) handleLocale(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	lang, err := normalizeLocaleName(strings.TrimPrefix(r.URL.Path, "/api/locales/"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	payload, err := fs.ReadFile(localeFiles, "locales/"+lang+".json")
	if err != nil {
		http.Error(w, "locale_not_found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func normalizeLocaleName(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = strings.Trim(normalized, "/")
	if normalized == "" {
		return "", fmt.Errorf("locale_required")
	}

	for _, separator := range []string{"-", "_"} {
		if base, _, ok := strings.Cut(normalized, separator); ok {
			normalized = base
			break
		}
	}

	switch normalized {
	case "en", "ru":
		return normalized, nil
	default:
		return "", fmt.Errorf("locale_not_supported")
	}
}
