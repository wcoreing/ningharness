package toolhost

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

const DefaultHTTPAddr = "127.0.0.1:51020"

type HTTPService struct {
	baseURL    string
	listenAddr string
	httpSrv    *http.Server
}

func (s *HTTPService) ListenAddr() string {
	if s == nil {
		return ""
	}
	return s.listenAddr
}

func (s *HTTPService) EndpointURL() string {
	if s == nil {
		return ""
	}
	return s.baseURL + "/mcp"
}

func (s *HTTPService) HealthURL() string {
	if s == nil {
		return ""
	}
	return s.baseURL + "/health"
}

type HTTPConfig struct {
	Addr         string
	ServerName   string
	Version      string
	Instructions string
	HealthName   string
	ExtraRoutes  func(mux *http.ServeMux, h *ToolHost)
}

func StartHTTP(cfg HTTPConfig, h *ToolHost) (*HTTPService, error) {
	if h == nil {
		return nil, fmt.Errorf("nil hub")
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = strings.TrimSpace(os.Getenv("NINGHARNESS_MCP_ADDR"))
	}
	if addr == "" {
		addr = DefaultHTTPAddr
	}
	name := strings.TrimSpace(cfg.ServerName)
	if name == "" {
		name = "ningharness"
	}
	version := strings.TrimSpace(cfg.Version)
	if version == "" {
		version = ServerVersion
	}
	healthName := strings.TrimSpace(cfg.HealthName)
	if healthName == "" {
		healthName = name
	}

	mcpHTTP := server.NewStreamableHTTPServer(NewMCPServer(h, name, version, cfg.Instructions),
		server.WithEndpointPath("/mcp"),
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", withCORS(mcpHTTP))
	if cfg.ExtraRoutes != nil {
		cfg.ExtraRoutes(mux, h)
	}
	mux.HandleFunc("/health", withCORSFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"ok":true,"name":%q,"mcp":"/mcp"}`, healthName)))
	}))

	ln, err := listenTCP(addr)
	if err != nil {
		return nil, fmt.Errorf("mcp listen %s: %w", addr, err)
	}

	httpSrv := &http.Server{Handler: mux}
	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[mcp] serve error: %v", err)
		}
	}()

	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = httpSrv.Close()
		return nil, err
	}
	bound := net.JoinHostPort(host, port)
	svc := &HTTPService{
		baseURL:    fmt.Sprintf("http://%s:%s", host, port),
		listenAddr: bound,
		httpSrv:    httpSrv,
	}
	log.Printf("[mcp] http %s", svc.EndpointURL())
	return svc, nil
}

// listenTCP binds addr; if that port is busy (and not already :0), retries host:0.
func listenTCP(addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		return ln, nil
	}
	if strings.HasSuffix(addr, ":0") {
		return nil, err
	}
	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr != nil || host == "" {
		host = "127.0.0.1"
	}
	fallback := net.JoinHostPort(host, "0")
	ln2, err2 := net.Listen("tcp", fallback)
	if err2 != nil {
		return nil, err
	}
	log.Printf("[mcp] %s busy, using %s", addr, ln2.Addr().String())
	return ln2, nil
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withCORSFunc(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		fn(w, r)
	}
}

func writeCORS(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, Mcp-Session-Id")
	w.Header().Set("Vary", "Origin")
}

func (s *HTTPService) Stop(ctx context.Context) error {
	if s == nil || s.httpSrv == nil {
		return nil
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
	}
	return s.httpSrv.Shutdown(ctx)
}
