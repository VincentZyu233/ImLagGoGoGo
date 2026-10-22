package gsi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

type PayloadHandler func(payload *GSIPayload)

type Server struct {
	startPort  int
	maxPort    int
	actualPort int

	httpServer *http.Server
	listener   net.Listener
	handlers   []PayloadHandler
	mu         sync.RWMutex

	lastConnected time.Time
	lastPayload   *GSIPayload
}

func NewServer(startPort, maxPort int) *Server {
	if startPort <= 0 {
		startPort = 4000
	}
	if maxPort < startPort {
		maxPort = 4999
	}

	return &Server{
		startPort: startPort,
		maxPort:   maxPort,
		handlers:  make([]PayloadHandler, 0),
	}
}

// OnPayload 注册 GSI 快照处理器
func (s *Server) OnPayload(h PayloadHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, h)
}

// Start 在指定区间内自动探测可用端口并启动监听
func (s *Server) Start() (int, error) {
	var l net.Listener
	var err error
	var boundPort int

	for port := s.startPort; port <= s.maxPort; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		l, err = net.Listen("tcp", addr)
		if err == nil {
			boundPort = port
			break
		}
	}

	if l == nil {
		return 0, fmt.Errorf("unable to bind any port in range [%d, %d]: %w", s.startPort, s.maxPort, err)
	}

	s.listener = l
	s.actualPort = boundPort

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleGSI)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		_ = s.httpServer.Serve(l)
	}()

	return boundPort, nil
}

func (s *Server) handleGSI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload GSIPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.lastConnected = time.Now()
	s.lastPayload = &payload
	handlers := make([]PayloadHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.Unlock()

	for _, h := range handlers {
		go h(&payload)
	}

	w.WriteHeader(http.StatusOK)
}

// ActualPort 获取当前实际绑定的端口
func (s *Server) ActualPort() int {
	return s.actualPort
}

// IsConnected 判断最近 10 秒内是否收到过 CS2 的 GSI 心跳
func (s *Server) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.lastConnected.IsZero() && time.Since(s.lastConnected) < 10*time.Second
}

// LastPayload 获取最新的快照数据
func (s *Server) LastPayload() *GSIPayload {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastPayload
}

// Stop 优雅关闭 GSI 服务端
func (s *Server) Stop() error {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
