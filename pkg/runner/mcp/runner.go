package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"tableflip.dev/bujo/pkg/store"
)

// Transport selects the mechanism used to expose the MCP server.
type Transport string

const (
	// TransportHTTP serves MCP via the streamable HTTP transport.
	TransportHTTP Transport = "http"
	// TransportStdio serves MCP over stdio.
	TransportStdio Transport = "stdio"
)

// Runner coordinates MCP server startup.
type Runner struct {
	Persistence store.Persistence
	Name        string
	Version     string

	Transport        Transport
	HTTPListenAddr   string
	HTTPEndpointPath string
	OnHTTPListening  func(net.Addr)
	HTTPServerCert   string
	HTTPServerKey    string
}

// Run starts the Model Context Protocol server using stdio transport.
func Run(ctx context.Context, persistence store.Persistence) error {
	r := Runner{
		Persistence: persistence,
		Name:        "bujo",
		Version:     "dev",
		Transport:   TransportStdio,
	}
	return r.Do(ctx)
}

// RunHTTP starts the MCP server over HTTP at the provided address.
func RunHTTP(ctx context.Context, persistence store.Persistence, addr string) error {
	r := Runner{
		Persistence:      persistence,
		Name:             "bujo",
		Version:          "dev",
		Transport:        TransportHTTP,
		HTTPListenAddr:   addr,
		HTTPEndpointPath: "/mcp",
	}
	return r.Do(ctx)
}

// Do executes the runner.
func (r Runner) Do(ctx context.Context) error {
	if r.Persistence == nil {
		return errors.New("mcp runner requires persistence")
	}
	name := r.Name
	if name == "" {
		name = "bujo"
	}
	version := r.Version
	if version == "" {
		version = "dev"
	}

	srv := server.NewMCPServer(
		fmt.Sprintf("%s MCP", name),
		version,
		server.WithResourceCapabilities(false, false),
		server.WithToolCapabilities(false),
		server.WithInstructions("Access bullet journal collections, entries, and mutations via MCP."),
		server.WithResourceRecovery(),
		server.WithRecovery(),
	)

	svc := NewService(r.Persistence)
	registerResources(srv, svc)
	registerTools(srv, svc)

	switch t := r.Transport; t {
	case "", TransportHTTP:
		return r.serveHTTP(ctx, srv)
	case TransportStdio:
		return server.ServeStdio(srv)
	default:
		return fmt.Errorf("unknown MCP transport %q", t)
	}
}

func (r Runner) serveHTTP(ctx context.Context, srv *server.MCPServer) error {
	if (r.HTTPServerCert != "" && r.HTTPServerKey == "") || (r.HTTPServerCert == "" && r.HTTPServerKey != "") {
		return errors.New("both http tls cert and key must be provided")
	}

	handler := server.NewStreamableHTTPServer(srv)

	path := r.HTTPEndpointPath
	if path == "" {
		path = "/mcp"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	listenAddr := r.HTTPListenAddr
	if listenAddr == "" {
		listenAddr = "127.0.0.1:8080"
	}

	mux := http.NewServeMux()
	mux.Handle(path, handler)

	httpSrv := &http.Server{
		Handler: mux,
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	if r.OnHTTPListening != nil {
		r.OnHTTPListening(ln.Addr())
	}

	if ctx != nil {
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpSrv.Shutdown(shutdownCtx)
		}()
	}

	if r.HTTPServerCert != "" && r.HTTPServerKey != "" {
		err = httpSrv.ServeTLS(ln, r.HTTPServerCert, r.HTTPServerKey)
	} else {
		err = httpSrv.Serve(ln)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
