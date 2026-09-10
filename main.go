package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "serve0:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(`Usage: serve0 <page.html> [port]

Serve an HTML preview page at the root URL (/), in the foreground.
Relative paths are resolved from the invoking working directory.
An omitted port or zero lets the OS allocate an available port.
An explicit port (1-65535) must be available; binding errors are fatal.
HTTP listens on all IPv4 interfaces without authentication.
Printed URLs are candidates; firewall and routing determine reachability.
Only the root preview URL is served. Press Ctrl+C to stop.
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
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		f, err := page.open()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method != http.MethodHead {
			io.Copy(w, f)
		}
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
