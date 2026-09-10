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
saved edits to HTML and preview assets without restarting. Responses use
`Cache-Control: no-store` so an ordinary refresh fetches current contents.
Adjacent and nested stylesheets, scripts, images, fonts, and other ordinary
files are served with content types appropriate to their filenames.

Relative page paths resolve from the invoking working directory. The containing
directory is the preview directory, even when the command starts elsewhere.
Every ordinary file beneath it is available by a known URL, including neighboring
files the preview page does not reference. Use a dedicated preview directory to
limit which files are exposed. Directory listings, dotfiles, and dot-directories
are hidden. Links to hidden paths or files outside the preview directory are
rejected, including replacements made while the command runs. Missing,
unreadable, non-regular, or prohibited paths return 404; unknown routes never
fall back to the preview page.

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

Confinement tests require symlink creation on each supported platform (Linux,
macOS, and Windows); failures are reported rather than skipped. On Windows,
enable Developer Mode or run the tests with permission to create symlinks.

Browser refresh smoke check: create a preview page referencing a neighboring
stylesheet and script, open its printed URL, then edit both its heading and the
stylesheet's heading color. An ordinary browser refresh should show the new
heading and color without restarting `serve0`. This was also exercised with
headless Chrome using an ordinary reload with cache bypass disabled.
