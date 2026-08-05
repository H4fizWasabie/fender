package main

import (
	"embed"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

//go:embed static
var staticFS embed.FS

// dashboard serves the embedded web UI on localhost only.
func dashboard(out io.Writer, cfgPath string) error {
	d, err := newDashState(cfgPath)
	if err != nil {
		return err
	}
	mux, err := newDashboardMux(d)
	if err != nil {
		return err
	}
	const addr = "127.0.0.1:8787"
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if strings.Contains(err.Error(), "in use") {
			return fmt.Errorf("port 8787 is already in use — is another fender dashboard running? (kill it with: pkill -f 'fender dashboard')")
		}
		return err
	}
	if _, err := fmt.Fprintf(out, "fender dashboard at http://%s (ctrl-c to stop)\n", addr); err != nil {
		_ = ln.Close()
		return err
	}
	return http.Serve(ln, mux)
}
