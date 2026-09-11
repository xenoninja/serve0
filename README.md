# serve0

Serve a static HTML preview page from a terminal. Downloaded executables need
neither Go nor another application runtime.

Download a ZIP from [GitHub Releases](https://github.com/xenoninja/serve0/releases)
for your operating system and CPU: Linux, macOS (`darwin`), or Windows;
`amd64` for Intel/AMD 64-bit or `arm64` for ARM64 (including Apple silicon).
Extract it and put `serve0` (Windows: `serve0.exe`) on your PATH, or invoke it
by its extracted path. On Linux/macOS, use `chmod +x serve0` if your extractor
does not preserve executable permissions. Compare the archive's SHA-256 with
`SHA256SUMS` using `sha256sum`, `shasum -a 256`, or PowerShell
`Get-FileHash -Algorithm SHA256`.

With Go 1.25 or newer, install using only the standard library:

```sh
go install github.com/xenoninja/serve0@latest
```

The executable goes in `GOBIN`, or `$(go env GOPATH)/bin` when unset; add that
directory to PATH. Use `@vMAJOR.MINOR.PATCH` to select a published version.
To build the checked-out source and run:

```sh
go build -o serve0 .
./serve0 path/to/mockup.html       # OS allocates an available port
./serve0 path/to/mockup.html 0     # same automatic allocation
./serve0 /absolute/mockup.html 8080
./serve0 --help
./serve0 --version
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

Use this tool only on a trusted network. HTTP listens on all IPv4 interfaces
without authentication or TLS. Candidate URLs use
machine addresses; selecting a port does not make it remotely reachable. Routing,
VPN connectivity, and firewall rules determine which candidate works from your
browser. The wildcard bind address is not a browser destination.

Run the CLI/HTTP integration tests with `go test ./...` and static checks with
`go vet ./...`. Tests build the executable, use temporary fixtures and dynamically
allocated ports, and require permission to open local listening sockets.

The root `main.go` handles process exit and build identification; preview command
implementation lives in `internal/app/`. Executable integration tests and their
platform-specific process helpers live in `tests/integration/`; release packaging
lives in `cmd/package/`. Root build and installation commands remain the same.

Confinement tests require symlink creation on each supported platform (Linux,
macOS, and Windows); failures are reported rather than skipped. On Windows,
enable Developer Mode or run the tests with permission to create symlinks.

Browser refresh smoke check: create a preview page referencing a neighboring
stylesheet and script, open its printed URL, then edit both its heading and the
stylesheet's heading color. An ordinary browser refresh should show the new
heading and color without restarting `serve0`. This was also exercised with
headless Chrome using an ordinary reload with cache bypass disabled.

`serve0 --help` (or `-h`) prints usage; `serve0 --version` prints the running
build without requiring a preview page or opening a listener. Release packages
report their tag; versioned `go install` builds report their module version.
Local builds report `devel+<commit>` with `-dirty` for modified source when Go
embeds VCS metadata, or `devel` when metadata is unavailable.
On Windows PowerShell, invoke a local executable as `.\serve0.exe`.

To keep a preview across SSH disconnections, start `tmux` or `screen`, run
`serve0 page.html` inside it, and detach the multiplexer. Reattach and press
Ctrl+C to end the preview. serve0 itself remains a foreground process.
It does not launch a browser, automatically reload pages, manage a background
service, configure the network, or build or execute an application backend.

Release mechanics: from the repository root, run
`go run ./cmd/package -version vMAJOR.MINOR.PATCH`. This creates six ZIPs and
`SHA256SUMS` in `dist/`, each ZIP containing the executable and this README.
The supported targets are Linux/macOS/Windows on amd64 and arm64. Builds disable
cgo, use baseline CPU features, trim source paths and omit VCS metadata and build
IDs; archives use fixed ordering, permissions and timestamps. Reproduction
requires the same source, version and Go toolchain (CI pins Go 1.25.0).
No platform SDK or runtime is needed to cross-compile these packages.

The GitHub Actions workflow packages all targets twice and compares checksums.
It runs the CLI/HTTP suite against direct builds, `go install .` output, and
extracted packages on Linux amd64, macOS arm64 and Windows amd64. Tests include
path handling, symlink confinement, content types, ports, refresh HTTP behavior
and interrupt shutdown. Windows automation sends console Ctrl+Break, which Go
delivers as the same interrupt used by Ctrl+C; a physical Ctrl+C smoke check
remains a manual check. Windows runners need symlink privileges.
Linux arm64, macOS amd64 and Windows arm64 are cross-compiled and packaged;
native runtime coverage for those targets is not claimed.

A maintainer pushes a `vMAJOR.MINOR.PATCH` tag to trigger these checks and create
a **draft** GitHub Release with the ZIPs and checksums. Review the job results
and release notes, then publish the draft to make the downloads public.
Branch and pull-request builds provide downloadable workflow artifacts with
version `v0.0.0-dev.<commit SHA>` without publishing a release.

Local issue #4 verification: all six targets were cross-compiled and packaged,
and repeat packages had identical checksums. Linux amd64 direct, locally
installed and extracted executables passed the CLI/HTTP suite. Native macOS and
Windows runners were unavailable locally; their workflow results must be checked
before publishing. Headless Chrome also loaded the extracted Linux executable's preview
page, stylesheet and script; an ordinary reload with cache bypass disabled
displayed edited HTML and CSS without restarting the process. Versioned remote `go install ...@<tag>` requires a published tag;
local installation is the installation path exercised before release.
