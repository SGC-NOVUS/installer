package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sgc-novus/novus-installer/internal/orchestrator"
)

const (
	clientBufferSize   = 256
	pingInterval       = 25 * time.Second
	writeWait          = 10 * time.Second
	readDeadlineWindow = 60 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}

		parsed, err := url.Parse(origin)
		if err != nil {
			return false
		}

		return strings.EqualFold(parsed.Host, r.Host)
	},
}

type Broadcaster struct {
	mu         sync.RWMutex
	clients    map[*wsClient]struct{}
	closed     bool
	lastStatus []byte
}

type wsClient struct {
	conn   *websocket.Conn
	send   chan outboundFrame
	mu     sync.RWMutex
	closed bool
}

type outboundFrame struct {
	messageType int
	payload     []byte
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[*wsClient]struct{}),
	}
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.broadcaster.ServeWS(w, r)
}

func (b *Broadcaster) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan outboundFrame, clientBufferSize),
	}
	if !b.addClient(client) {
		_ = conn.Close()
		return
	}

	go b.writePump(client)
	if payload := b.snapshotLastStatus(); len(payload) > 0 {
		_ = client.enqueue(outboundFrame{messageType: websocket.TextMessage, payload: payload})
	}
	b.readPump(client)
}

func (b *Broadcaster) Write(payload []byte) (int, error) {
	b.BroadcastBinary(payload)
	return len(payload), nil
}

func (b *Broadcaster) EmitStatus(message orchestrator.StatusMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("status_json_encode_failed:%w", err)
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.lastStatus = append([]byte(nil), payload...)
	b.mu.Unlock()

	b.broadcastFrame(websocket.TextMessage, payload)
	return nil
}

func (b *Broadcaster) BroadcastBinary(payload []byte) {
	b.broadcastFrame(websocket.BinaryMessage, payload)
}

func (b *Broadcaster) broadcastFrame(messageType int, payload []byte) {
	if len(payload) == 0 {
		return
	}

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return
	}

	clients := make([]*wsClient, 0, len(b.clients))
	for client := range b.clients {
		clients = append(clients, client)
	}
	b.mu.RUnlock()

	for _, client := range clients {
		if !client.enqueue(outboundFrame{messageType: messageType, payload: payload}) {
			b.removeClient(client)
		}
	}
}

func (b *Broadcaster) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true

	clients := make([]*wsClient, 0, len(b.clients))
	for client := range b.clients {
		clients = append(clients, client)
	}
	b.clients = make(map[*wsClient]struct{})
	b.mu.Unlock()

	for _, client := range clients {
		client.close()
	}
}

func (b *Broadcaster) snapshotLastStatus() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.lastStatus) == 0 {
		return nil
	}

	return append([]byte(nil), b.lastStatus...)
}

func (b *Broadcaster) addClient(client *wsClient) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return false
	}

	b.clients[client] = struct{}{}
	return true
}

func (b *Broadcaster) removeClient(client *wsClient) {
	b.mu.Lock()
	if _, ok := b.clients[client]; ok {
		delete(b.clients, client)
	}
	b.mu.Unlock()

	client.close()
}

func (b *Broadcaster) readPump(client *wsClient) {
	defer b.removeClient(client)

	client.conn.SetReadLimit(1024)
	_ = client.conn.SetReadDeadline(time.Now().Add(readDeadlineWindow))
	client.conn.SetPongHandler(func(string) error {
		return client.conn.SetReadDeadline(time.Now().Add(readDeadlineWindow))
	})

	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (b *Broadcaster) writePump(client *wsClient) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	defer b.removeClient(client)

	for {
		select {
		case frame, ok := <-client.send:
			if !ok {
				_ = client.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(writeWait))
				return
			}

			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(frame.messageType, frame.payload); err != nil {
				return
			}
		case <-ticker.C:
			if err := client.conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
				return
			}
		}
	}
}

func (c *wsClient) enqueue(frame outboundFrame) bool {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return false
	}
	ch := c.send
	c.mu.RUnlock()

	frame.payload = append([]byte(nil), frame.payload...)
	select {
	case ch <- frame:
		return true
	default:
		return false
	}
}

func (c *wsClient) close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.send)
	c.mu.Unlock()

	_ = c.conn.Close()
}
