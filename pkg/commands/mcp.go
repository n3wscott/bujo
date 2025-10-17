package commands

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/runner/mcp"
	"tableflip.dev/bujo/pkg/store"
)

func addMCP(topLevel *cobra.Command) {
	var (
		transport   string
		httpHost    string
		httpPort    int
		httpPath    string
		httpTLSCert string
		httpTLSKey  string
	)

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "start the Model Context Protocol server",
		Long: `Launch an MCP server that exposes collections, entries, and journal commands
through the OpenAI-compatible Model Context Protocol.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			persistence, err := store.Load(nil)
			if err != nil {
				return err
			}

			path := strings.TrimSpace(httpPath)
			if path == "" {
				path = "/mcp"
			}
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}

			runner := mcp.Runner{
				Persistence:      persistence,
				Name:             "bujo",
				Version:          "dev",
				HTTPEndpointPath: path,
				HTTPServerCert:   strings.TrimSpace(httpTLSCert),
				HTTPServerKey:    strings.TrimSpace(httpTLSKey),
			}

			switch strings.ToLower(strings.TrimSpace(transport)) {
			case "", string(mcp.TransportHTTP):
				host := strings.TrimSpace(httpHost)
				if host == "" {
					host = "127.0.0.1"
				}
				port := httpPort
				if port < 0 || port > 65535 {
					return fmt.Errorf("invalid http-port %d", port)
				}

				addr := net.JoinHostPort(host, strconv.Itoa(port))
				runner.Transport = mcp.TransportHTTP
				runner.HTTPListenAddr = addr
				runner.OnHTTPListening = func(a net.Addr) {
					tcpAddr, ok := a.(*net.TCPAddr)
					if !ok {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "MCP HTTP server listening on %s%s\n", addr, path)
						return
					}

					displayHost := host
					if displayHost == "" || displayHost == "0.0.0.0" || displayHost == "::" {
						if tcpAddr.IP != nil && !tcpAddr.IP.IsUnspecified() {
							displayHost = tcpAddr.IP.String()
						} else {
							displayHost = "127.0.0.1"
						}
					}

					if strings.Contains(displayHost, ":") && !strings.HasPrefix(displayHost, "[") {
						displayHost = "[" + displayHost + "]"
					}

					scheme := "http"
					if runner.HTTPServerCert != "" && runner.HTTPServerKey != "" {
						scheme = "https"
					}

					_, _ = fmt.Fprintf(cmd.OutOrStdout(),
						"MCP HTTP server listening on %s://%s:%d%s\n",
						scheme,
						displayHost,
						tcpAddr.Port,
						path,
					)
				}
			case string(mcp.TransportStdio):
				runner.Transport = mcp.TransportStdio
			default:
				return fmt.Errorf("unsupported transport %q (expected http or stdio)", transport)
			}

			return runner.Do(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&transport, "transport", string(mcp.TransportHTTP), "transport to use: http or stdio")
	cmd.Flags().StringVar(&httpHost, "http-host", "127.0.0.1", "host/interface for HTTP transport")
	cmd.Flags().IntVar(&httpPort, "http-port", 8080, "port for HTTP transport (use 0 for random)")
	cmd.Flags().StringVar(&httpPath, "http-path", "/mcp", "HTTP endpoint path")
	cmd.Flags().StringVar(&httpTLSCert, "http-tls-cert", "", "TLS certificate file for HTTPS")
	cmd.Flags().StringVar(&httpTLSKey, "http-tls-key", "", "TLS private key file for HTTPS")

	topLevel.AddCommand(cmd)
}
