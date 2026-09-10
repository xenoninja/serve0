# serve0

Serve a static HTML preview page from a terminal. Requires Go 1.25 or newer;
uses only the standard library.

```sh
go build -o serve0 .
./serve0 path/to/mockup.html       # OS allocates an available port
./serve0 path/to/mockup.html 0     # same automatic allocation
./serve0 /absolute/mockup.html 8080
./serve0 --help
```

Copy one of the printed candidate browser URLs. The root URL `/` serves the
selected preview page, regardless of its filename. Refresh the browser to see
saved edits. Other URL paths currently return 404; preview assets are not yet
served.

Relative page paths resolve from the invoking working directory. The containing
directory is the preview directory. Hidden pages and links to hidden files or
files outside that directory are rejected. A deleted, unreadable, non-regular,
or disallowed replacement page returns 404 until a valid page is available again.

The process stays in the foreground, including inside tmux or screen. Press
Ctrl+C to stop it and release the port. Invalid arguments, inaccessible pages,
and unavailable explicit ports produce a diagnostic on stderr and a nonzero exit
status. An explicit port never falls back to another port.

HTTP listens on all IPv4 interfaces without authentication. Candidate URLs use
machine addresses; selecting a port does not make it remotely reachable. Routing,
VPN connectivity, and firewall rules determine which candidate works from your
browser. The wildcard bind address is not a browser destination.

Run the CLI/HTTP integration tests with `go test ./...` and static checks with
`go vet ./...`. Tests build the executable, use temporary fixtures and dynamically
allocated ports, and require permission to open local listening sockets.
