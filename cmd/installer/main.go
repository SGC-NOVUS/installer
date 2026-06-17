package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sgc-novus/novus-installer/internal/preflight"
	webserver "github.com/sgc-novus/novus-installer/internal/web"
)

const (
	listenAddr        = ":8080"
	preflightDeadline = time.Second
	shutdownTimeout   = 5 * time.Second
)

func main() {
	devMode := flag.Bool("dev", false, "run installer with relaxed preflight checks")
	dryRun := flag.Bool("dry-run", false, "simulate installer commands without mutating the host")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	checkCtx, cancel := context.WithTimeout(ctx, preflightDeadline)
	defer cancel()

	result, err := preflight.Run(checkCtx, preflight.Options{DevMode: *devMode})
	if err != nil {
		log.Fatalf("novus-installer preflight failed: %v", err)
	}

	printWarnings(result)

	token, err := generateToken()
	if err != nil {
		log.Fatalf("installer_token_generate_failed: %v", err)
	}

	server, err := webserver.New(webserver.Config{
		Address:     listenAddr,
		Token:       token,
		BaseContext: ctx,
		DevMode:     *devMode,
		DryRun:      *dryRun,
	})
	if err != nil {
		log.Fatalf("installer_web_init_failed: %v", err)
	}

	printBanner(result, token, *dryRun)

	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("installer_web_shutdown_failed: %v", err)
		}
	case err := <-errCh:
		if err != nil {
			log.Fatalf("installer_web_failed: %v", err)
		}
	}
}

func generateToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}

func printWarnings(result *preflight.Result) {
	for _, warning := range result.Warnings {
		_, _ = fmt.Fprintf(os.Stderr, "[WARNING] %s\n", warning)
	}

}

func printBanner(result *preflight.Result, token string, dryRun bool) {
	hostHint := detectHostHint()
	installerURL := fmt.Sprintf("http://%s%s/?token=%s", hostHint, listenAddr, token)
	preflightStatus := "Pre-flight: OK"
	executionMode := "live execution"
	osLine := result.OS.DisplayName()
	if osLine == "" {
		osLine = "unknown"
	}
	ramLine := result.TotalRAMString()
	if result.TotalRAMBytes == 0 {
		ramLine = "unknown"
	}
	portsLine := "80/tcp and 443/tcp are available"

	if result.DevMode {
		preflightStatus = "Pre-flight: DEV MODE"
		if len(result.Warnings) > 0 {
			preflightStatus = "Pre-flight: DEV MODE (warnings ignored)"
		}
		portsLine = "dev-mode override active for OS/RAM/port checks"
	}
	if dryRun {
		executionMode = "dry-run (commands are simulated)"
	}

	_, _ = fmt.Fprintf(os.Stdout, "\n")
	_, _ = fmt.Fprintf(os.Stdout, "============================================================\n")
	_, _ = fmt.Fprintf(os.Stdout, " NOVUS-OS Installer: Phase A bootstrap ready\n")
	_, _ = fmt.Fprintf(os.Stdout, "============================================================\n")
	_, _ = fmt.Fprintf(os.Stdout, " %s\n", preflightStatus)
	_, _ = fmt.Fprintf(os.Stdout, " Execution:  %s\n", executionMode)
	_, _ = fmt.Fprintf(os.Stdout, " OS:         %s\n", osLine)
	_, _ = fmt.Fprintf(os.Stdout, " RAM:        %s\n", ramLine)
	_, _ = fmt.Fprintf(os.Stdout, " Ports:      %s\n", portsLine)
	_, _ = fmt.Fprintf(os.Stdout, "\n")
	_, _ = fmt.Fprintf(os.Stdout, " NOVUS-OS Installer запущен. Для продолжения установки\n")
	_, _ = fmt.Fprintf(os.Stdout, " перейдите в браузере по ссылке:\n")
	_, _ = fmt.Fprintf(os.Stdout, " %s\n", installerURL)
	_, _ = fmt.Fprintf(os.Stdout, "\n")
	_, _ = fmt.Fprintf(os.Stdout, " Token protection is enabled for every HTTP request.\n")
	_, _ = fmt.Fprintf(os.Stdout, " Stop the installer with Ctrl+C when the session is complete.\n")
	_, _ = fmt.Fprintf(os.Stdout, "============================================================\n")
	_, _ = fmt.Fprintf(os.Stdout, "\n")
}

func detectHostHint() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}

			ip := ipNet.IP.To4()
			if ip == nil || !ip.IsGlobalUnicast() {
				continue
			}

			return ip.String()
		}
	}

	return "127.0.0.1"
}
