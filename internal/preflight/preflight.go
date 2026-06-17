package preflight

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const (
	minimumRAMBytes = 2 * 1024 * 1024 * 1024
	osReleasePath   = "/etc/os-release"
	memInfoPath     = "/proc/meminfo"
)

var allowedOS = map[string]map[string]struct{}{
	"ubuntu": {
		"22.04": {},
		"24.04": {},
		"26.04": {},
		"26.10": {},
	},
	"debian": {
		"12": {},
		"13": {},
	},
}

type Options struct {
	DevMode bool
}

type Result struct {
	OS            OSRelease
	TotalRAMBytes uint64
	Warnings      []string
	DevMode       bool
}

type issueKind string

const (
	issueRoot issueKind = "root"
	issueOS   issueKind = "os"
	issueRAM  issueKind = "ram"
	issuePort issueKind = "port"
)

type OSRelease struct {
	ID         string
	VersionID  string
	PrettyName string
	Name       string
	Version    string
}

func (o OSRelease) DisplayName() string {
	if strings.TrimSpace(o.PrettyName) != "" {
		return o.PrettyName
	}

	name := strings.TrimSpace(o.Name)
	version := strings.TrimSpace(o.Version)
	if name == "" {
		name = strings.TrimSpace(o.ID)
	}
	if version == "" {
		version = strings.TrimSpace(o.VersionID)
	}
	if version == "" {
		if name == "" {
			return "unknown"
		}
		return name
	}

	return strings.TrimSpace(name + " " + version)
}

func (r *Result) TotalRAMString() string {
	if r.TotalRAMBytes == 0 {
		return "unknown"
	}

	gb := float64(r.TotalRAMBytes) / float64(1024*1024*1024)
	return fmt.Sprintf("%.2f GiB", gb)
}

func Run(ctx context.Context, options Options) (*Result, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	var failures []error
	result := &Result{DevMode: options.DevMode}
	recordIssue := func(kind issueKind, err error) {
		if err == nil {
			return
		}
		if options.DevMode && isDevWarning(kind) {
			result.Warnings = append(result.Warnings, err.Error())
			return
		}
		failures = append(failures, err)
	}

	recordIssue(issueRoot, checkRoot())

	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	if runtime.GOOS != "linux" {
		recordIssue(issueOS, fmt.Errorf("unsupported_runtime_os:%s", runtime.GOOS))
	} else {
		osRelease, err := parseOSReleaseFile(osReleasePath)
		if err != nil {
			recordIssue(issueOS, err)
		} else {
			result.OS = osRelease
			recordIssue(issueOS, validateOS(osRelease))
		}

		if err := checkContext(ctx); err != nil {
			return nil, err
		}

		totalRAMBytes, err := readMemTotalBytes(memInfoPath)
		if err != nil {
			recordIssue(issueRAM, err)
		} else {
			result.TotalRAMBytes = totalRAMBytes
			if totalRAMBytes < minimumRAMBytes {
				recordIssue(issueRAM, fmt.Errorf(
					"insufficient_memory: have=%d bytes want>=%d bytes",
					totalRAMBytes,
					minimumRAMBytes,
				))
			}
		}

		for _, port := range []int{80, 443} {
			if err := checkContext(ctx); err != nil {
				return nil, err
			}
			recordIssue(issuePort, ensurePortAvailable(ctx, port))
		}
	}

	if len(failures) > 0 {
		return nil, joinFailures(failures)
	}

	return result, nil
}

	isDevWarning := func(kind issueKind) bool {
		switch kind {
		case issueOS, issueRAM, issuePort:
			return true
		default:
			return false
		}
	}

func checkRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("root_required: installer must run with uid 0")
	}

	return nil
}

func validateOS(release OSRelease) error {
	id := strings.ToLower(strings.TrimSpace(release.ID))
	version := strings.TrimSpace(release.VersionID)
	versions, ok := allowedOS[id]
	if !ok {
		return fmt.Errorf("unsupported_os: %s", release.DisplayName())
	}
	if _, ok := versions[version]; !ok {
		return fmt.Errorf("unsupported_os: %s", release.DisplayName())
	}

	return nil
}

func ensurePortAvailable(ctx context.Context, port int) error {
	address := fmt.Sprintf(":%d", port)
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("port_unavailable:%d: %w", port, err)
	}
	defer listener.Close()

	return nil
}

func parseOSReleaseFile(path string) (OSRelease, error) {
	file, err := os.Open(path)
	if err != nil {
		return OSRelease{}, fmt.Errorf("os_release_read_failed: %w", err)
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		} else {
			value = strings.Trim(value, "\"'")
		}

		values[strings.TrimSpace(key)] = value
	}
	if err := scanner.Err(); err != nil {
		return OSRelease{}, fmt.Errorf("os_release_scan_failed: %w", err)
	}

	release := OSRelease{
		ID:         values["ID"],
		VersionID:  values["VERSION_ID"],
		PrettyName: values["PRETTY_NAME"],
		Name:       values["NAME"],
		Version:    values["VERSION"],
	}
	if strings.TrimSpace(release.ID) == "" || strings.TrimSpace(release.VersionID) == "" {
		return OSRelease{}, fmt.Errorf("os_release_invalid: missing ID or VERSION_ID")
	}

	return release, nil
}

func readMemTotalBytes(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("meminfo_read_failed: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("meminfo_invalid: %s", line)
		}

		amountKB, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("meminfo_invalid: %w", err)
		}

		return amountKB * 1024, nil
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("meminfo_scan_failed: %w", err)
	}

	return 0, fmt.Errorf("meminfo_missing: MemTotal")
}

func checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("preflight_timeout: %w", ctx.Err())
	default:
		return nil
	}
}

func joinFailures(failures []error) error {
	parts := make([]string, 0, len(failures))
	for _, failure := range failures {
		if failure == nil {
			continue
		}
		parts = append(parts, failure.Error())
	}
	return fmt.Errorf("preflight_blocked: %s", strings.Join(parts, "; "))
}
