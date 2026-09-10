package main

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// version is set by the release builder; go install uses module build metadata.
var version string

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		revision := ""
		dirty := false
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				dirty = setting.Value == "true"
			}
		}
		if revision != "" {
			if dirty {
				revision += "-dirty"
			}
			return "devel+" + revision
		}
	}
	return "devel"
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "serve0:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Println("serve0", buildVersion())
		return nil
	}
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(`Usage: serve0 <page.html> [port]
       serve0 --help | -h
       serve0 --version

Serve an HTML preview page at the root URL (/), in the foreground.
Relative paths are resolved from the invoking working directory.
An omitted port or zero lets the OS allocate an available port.
An explicit port (1-65535) must be available; binding errors are fatal.
HTTP listens on all IPv4 interfaces without authentication.
Printed URLs are candidates; firewall and routing determine reachability.
Ordinary files in the preview directory are available by known URL.
Directory listings and hidden paths are disabled. Press Ctrl+C to stop.
`)
		return nil
	}
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: serve0 <page.html> [port]")
	}
	page, err := selectPage(args[0])
	if err != nil {
		return err
	}
	defer page.root.Close()
	port := 0
	if len(args) == 2 {
		port, err = strconv.Atoi(args[1])
		if err != nil || port < 0 || port > 65535 {
			return fmt.Errorf("port must be an integer from 0 through 65535")
		}
	}
	listener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		name := page.name
		if r.URL.Path != "/" {
			name = strings.TrimPrefix(r.URL.Path, "/")
			// URL.Path is already decoded. Reject ambiguous paths before any cleaning.
			if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") {
				http.NotFound(w, r)
				return
			}
			name = filepath.FromSlash(name)
		}
		f, err := page.openPath(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		// A zero modification time disables conditional date responses: even edits
		// within one second must be visible on an ordinary browser refresh.
		http.ServeContent(w, r, name, time.Time{}, f)
	})}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Println("Candidate browser URLs:")
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		server.Close()
		return err
	}
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err == nil && ip.To4() != nil && !ip.IsUnspecified() {
			fmt.Printf("http://%s:%d/\n", ip, listener.Addr().(*net.TCPAddr).Port)
		}
	}
	fmt.Println("Press Ctrl+C to stop.")
	select {
	case <-stop:
		return server.Close()
	case err := <-done:
		return err
	}
}
