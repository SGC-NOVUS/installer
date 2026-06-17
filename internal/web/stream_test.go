package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sgc-novus/novus-installer/internal/orchestrator"
)

func TestDryRunWebSocketSmoke(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := New(Config{
		Address:     "127.0.0.1:0",
		Token:       "test-installer-token",
		BaseContext: ctx,
		DevMode:     true,
		DryRun:      true,
		DryRunDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer server.broadcaster.Close()

	httpServer := httptest.NewServer(server.Handler)
	defer httpServer.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}

	client := &http.Client{Jar: jar}
	resp, err := client.Get(httpServer.URL + "/?token=test-installer-token")
	if err != nil {
		t.Fatalf("GET root error = %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET root status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	serverURL, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	requestHeader := http.Header{}
	for _, cookie := range jar.Cookies(serverURL) {
		requestHeader.Add("Cookie", cookie.String())
	}

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, requestHeader)
	if err != nil {
		t.Fatalf("websocket dial error = %v", err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}

	requestBody := `{"Domain":"https://panel.example.com","AdminEmail":"admin@example.com","AdminPassword":"StrongPass123!","DBRootPassword":"RootPass123!","DBPanelPassword":"PanelPass123!"}`
	setupResponse, err := client.Post(httpServer.URL+"/api/setup", "application/json", strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("POST /api/setup error = %v", err)
	}
	_ = setupResponse.Body.Close()
	if setupResponse.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/setup status = %d, want %d", setupResponse.StatusCode, http.StatusOK)
	}

	var (
		seenBinaryDryRun bool
		seenFinish       bool
		seenHealthStep   bool
	)

	for !seenFinish {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage() error = %v", err)
		}

		switch messageType {
		case websocket.BinaryMessage:
			if strings.Contains(string(payload), "[DRY-RUN] Would execute:") {
				seenBinaryDryRun = true
			}
		case websocket.TextMessage:
			var status orchestrator.StatusMessage
			if err := json.Unmarshal(payload, &status); err != nil {
				t.Fatalf("json.Unmarshal() error = %v payload=%q", err, string(payload))
			}
			if status.Type == "step" && status.Text == "Проверка работоспособности системы" {
				seenHealthStep = true
			}
			if status.Type == "finish" {
				seenFinish = true
				if status.URL != "https://panel.example.com" {
					t.Fatalf("finish URL = %q, want %q", status.URL, "https://panel.example.com")
				}
			}
		default:
			t.Fatalf("unexpected websocket message type: %d", messageType)
		}
	}

	if !seenBinaryDryRun {
		t.Fatal("did not receive binary dry-run PTY frame")
	}
	if !seenHealthStep {
		t.Fatal("did not receive health-check step status")
	}
}
