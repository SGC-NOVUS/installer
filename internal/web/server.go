package web

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/sgc-novus/novus-installer/internal/orchestrator"
	assets "github.com/sgc-novus/novus-installer/web"
)

const tokenCookieName = "novus_installer_token"

type Config struct {
	Address     string
	Token       string
	BaseContext context.Context
	DevMode     bool
	DryRun      bool
	DryRunDelay time.Duration
	OnComplete  func()
}

type Server struct {
	*http.Server

	token       string
	logo        []byte
	baseContext context.Context
	broadcaster *Broadcaster
	runner      *orchestrator.Runner
}

func New(cfg Config) (*Server, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, fmt.Errorf("installer_web_address_required")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, fmt.Errorf("installer_web_token_required")
	}
	if cfg.BaseContext == nil {
		cfg.BaseContext = context.Background()
	}

	distFS, err := assets.DistFS()
	if err != nil {
		return nil, err
	}
	faviconBytes, err := fs.ReadFile(distFS, "assets/novus-logo-320.png")
	if err != nil {
		return nil, fmt.Errorf("embedded_favicon_not_found: %w", err)
	}

	server := &Server{
		token:       cfg.Token,
		logo:        faviconBytes,
		baseContext: cfg.BaseContext,
		broadcaster: NewBroadcaster(),
	}
	server.runner = orchestrator.NewRunner(server.broadcaster, orchestrator.RunnerOptions{
		DevMode:     cfg.DevMode,
		DryRun:      cfg.DryRun,
		DryRunDelay: cfg.DryRunDelay,
		OnComplete:  cfg.OnComplete,
	})

	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", server.staticAssetHandler("image/png", server.logo))
	mux.Handle("/logo.png", server.staticAssetHandler("image/png", server.logo))
	mux.Handle("/api/locales/", server.requireAuthorizedSession(http.HandlerFunc(server.handleLocale)))
	mux.Handle("/api/stream", server.requireAuthorizedSession(http.HandlerFunc(server.handleStream)))
	mux.Handle("/api/setup", server.requireAuthorizedSession(http.HandlerFunc(server.handleSetup)))
	mux.Handle("/", server.protectedFileServer(distFS))

	server.Server = &http.Server{
		Addr:              cfg.Address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	return server, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.broadcaster.Close()
	return s.Server.Shutdown(ctx)
}

func (s *Server) protectedFileServer(distFS fs.FS) http.Handler {
	fileServer := http.FileServerFS(distFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryToken := strings.TrimSpace(r.URL.Query().Get("token"))
		cookieAuthorized := s.hasAuthorizedCookie(r)
		queryAuthorized := tokenMatches(queryToken, s.token)
		if !cookieAuthorized && !queryAuthorized {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if queryAuthorized {
			http.SetCookie(w, &http.Cookie{
				Name:     tokenCookieName,
				Value:    s.token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				MaxAge:   3600,
			})
		}

		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) staticAssetHandler(contentType string, payload []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(payload)
	})
}

func (s *Server) requireAuthorizedSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hasAuthorizedCookie(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})

}

func (s *Server) hasAuthorizedCookie(r *http.Request) bool {
	return tokenMatches(cookieToken(r), s.token)
}

func tokenMatches(candidate string, expected string) bool {
	if strings.TrimSpace(candidate) == "" || strings.TrimSpace(expected) == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(candidate)), []byte(strings.TrimSpace(expected))) == 1

}

func cookieToken(r *http.Request) string {
	cookie, err := r.Cookie(tokenCookieName)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(cookie.Value)
}
