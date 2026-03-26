package main

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/gitpod-io/xterm-go"
)

//go:embed index.html
var staticFS embed.FS

const (
	defaultCols = 120
	defaultRows = 30
)

// --- Terminal session (outlives WebSocket connections) ---

type terminalSession struct {
	id   string
	mu   sync.Mutex
	ptmx *os.File
	cmd  *exec.Cmd
	term *xterm.Terminal
	sa   *xterm.SerializeAddon
	done chan struct{} // closed when PTY exits

	// Attached WebSocket writers. Key is a unique writer ID.
	writers map[string]*wsWriter
}

type wsWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsWriter) sendJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(websocket.TextMessage, data)
}

var (
	sessionsMu sync.Mutex
	sessions   = map[string]*terminalSession{}
)

func newSession() (*terminalSession, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: defaultCols, Rows: defaultRows})
	if err != nil {
		return nil, err
	}

	term := xterm.New(xterm.WithCols(defaultCols), xterm.WithRows(defaultRows), xterm.WithScrollback(1000))

	s := &terminalSession{
		id:      uuid.New().String()[:8],
		ptmx:    ptmx,
		cmd:     cmd,
		term:    term,
		sa:      xterm.NewSerializeAddon(term),
		done:    make(chan struct{}),
		writers: make(map[string]*wsWriter),
	}

	// PTY read loop — feeds the headless terminal and broadcasts raw data.
	go s.readLoop()

	sessionsMu.Lock()
	sessions[s.id] = s
	sessionsMu.Unlock()

	log.Printf("session %s: created", s.id)
	return s, nil
}

func getSession(id string) *terminalSession {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	return sessions[id]
}

func (s *terminalSession) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if err != nil {
			log.Printf("session %s: PTY closed", s.id)
			s.mu.Lock()
			for _, w := range s.writers {
				w.sendJSON(map[string]any{"type": "exited"})
			}
			s.mu.Unlock()
			close(s.done)
			return
		}
		if n > 0 {
			data := buf[:n]

			s.mu.Lock()
			// Feed headless terminal for serialization.
			s.term.Write(data)
			// Broadcast raw PTY data to all attached writers.
			msg := map[string]any{
				"type": "data",
				"data": base64.StdEncoding.EncodeToString(data),
			}
			for _, w := range s.writers {
				w.sendJSON(msg)
			}
			s.mu.Unlock()
		}
	}
}

func (s *terminalSession) addWriter(conn *websocket.Conn) (string, *wsWriter) {
	id := uuid.New().String()[:8]
	w := &wsWriter{conn: conn}
	s.mu.Lock()
	s.writers[id] = w
	s.mu.Unlock()
	log.Printf("session %s: writer %s attached (%d total)", s.id, id, len(s.writers))
	return id, w
}

func (s *terminalSession) removeWriter(id string) {
	s.mu.Lock()
	delete(s.writers, id)
	count := len(s.writers)
	s.mu.Unlock()
	log.Printf("session %s: writer %s detached (%d remaining)", s.id, id, count)
}

func (s *terminalSession) sendReplay(w *wsWriter) {
	s.mu.Lock()
	scrollback := s.term.Scrollback()
	data := s.sa.Serialize(&xterm.SerializeOptions{Scrollback: &scrollback})
	s.mu.Unlock()

	msg := map[string]any{
		"type":      "replay",
		"sessionId": s.id,
		"data":      base64.StdEncoding.EncodeToString(data),
	}
	if err := w.sendJSON(msg); err != nil {
		log.Printf("session %s: send replay error: %v", s.id, err)
	}
}

// --- WebSocket handler ---

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	// Start the Node.js backend as a child process.
	nodeBackendPort := "9091"
	startNodeBackend(nodeBackendPort)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := staticFS.ReadFile("index.html")
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

	http.HandleFunc("/ws", handleWS)
	http.HandleFunc("/ws-node", func(w http.ResponseWriter, r *http.Request) {
		proxyWebSocket(w, r, "localhost:"+nodeBackendPort)
	})

	port := "9090"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	log.Printf("Demo server on :%s (Go backend on /ws, Node.js backend on /ws-node)", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// startNodeBackend spawns the Node.js backend server as a child process.
func startNodeBackend(port string) {
	// Find the node-backend directory relative to the binary or working dir.
	scriptPath := "node-backend/server.js"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		// Try relative to the source directory.
		log.Printf("node-backend/server.js not found in working directory")
	}

	cmd := exec.Command("node", scriptPath)
	cmd.Env = append(os.Environ(), "PORT="+port)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("WARNING: failed to start Node.js backend: %v", err)
		return
	}
	log.Printf("Node.js backend started (pid %d) on :%s", cmd.Process.Pid, port)

	// Clean up on exit.
	go func() {
		cmd.Wait()
		log.Printf("Node.js backend exited")
	}()
}

// proxyWebSocket proxies a WebSocket connection to a backend server.
func proxyWebSocket(w http.ResponseWriter, r *http.Request, backendAddr string) {
	// Upgrade the client connection.
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("proxy upgrade error: %v", err)
		return
	}
	defer clientConn.Close()

	// Connect to the backend.
	backendURL := "ws://" + backendAddr + "/ws" + "?" + r.URL.RawQuery
	backendConn, resp, err := websocket.DefaultDialer.Dial(backendURL, nil)
	if err != nil {
		log.Printf("proxy backend dial error: %v", err)
		return
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	defer backendConn.Close()

	// Bidirectional copy.
	done := make(chan struct{}, 2)

	// Backend → client.
	go func() {
		for {
			msgType, data, err := backendConn.ReadMessage()
			if err != nil {
				done <- struct{}{}
				return
			}
			if err := clientConn.WriteMessage(msgType, data); err != nil {
				done <- struct{}{}
				return
			}
		}
	}()

	// Client → backend.
	go func() {
		for {
			msgType, data, err := clientConn.ReadMessage()
			if err != nil {
				done <- struct{}{}
				return
			}
			if err := backendConn.WriteMessage(msgType, data); err != nil {
				done <- struct{}{}
				return
			}
		}
	}()

	<-done
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Look up or create session.
	sessionID := r.URL.Query().Get("session")
	var sess *terminalSession
	if sessionID != "" {
		sess = getSession(sessionID)
	}
	if sess == nil {
		sess, err = newSession()
		if err != nil {
			log.Printf("create session error: %v", err)
			return
		}
	}

	// Check if session's PTY is still alive.
	select {
	case <-sess.done:
		log.Printf("session %s: PTY already exited, creating new session", sess.id)
		sess, err = newSession()
		if err != nil {
			log.Printf("create session error: %v", err)
			return
		}
	default:
	}

	// Attach this WebSocket as a writer.
	writerID, writer := sess.addWriter(conn)
	defer sess.removeWriter(writerID)

	// Send snapshot immediately.
	sess.sendReplay(writer)

	// Read client input → forward to PTY.
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return // WebSocket closed — detach writer, keep session alive.
		}
		var input struct {
			Type string `json:"type"`
			Data string `json:"data"`
			Cols int    `json:"cols"`
			Rows int    `json:"rows"`
		}
		if err := json.Unmarshal(message, &input); err != nil {
			continue
		}
		switch input.Type {
		case "input":
			sess.ptmx.Write([]byte(input.Data))
		case "resize":
			if input.Cols > 0 && input.Rows > 0 {
				pty.Setsize(sess.ptmx, &pty.Winsize{
					Cols: uint16(input.Cols),
					Rows: uint16(input.Rows),
				})
				sess.mu.Lock()
				sess.term.Resize(input.Cols, input.Rows)
				sess.mu.Unlock()
			}
		case "replay":
			sess.sendReplay(writer)
		}
	}
}
