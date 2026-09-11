# serve0

**Preview a static HTML page from your terminal.**

serve0 is a small command-line tool for viewing HTML mockups and their assets in
a browser, locally or from a remote development machine. Point it at a page,
open a printed URL, and refresh to see your edits.

- **One executable.** No application runtime required.
- **No configuration.** Choose a page and optionally a port.
- **Assets included.** Serve nearby stylesheets, scripts, images, and fonts.
- **Refresh to update.** Saved edits appear on an ordinary browser refresh.
- **Cross-platform.** Available for Linux, macOS, and Windows on amd64 and arm64.

## Quick start

After [installing serve0](#installation), run:

```sh
serve0 path/to/mockup.html
```

serve0 picks an available port and prints candidate browser URLs. Open one that
is reachable from your browser. The root URL (`/`) displays your selected preview
page, whatever its filename.

Edit the page or its preview assets, save, and refresh your browser to see the
changes. Press **Ctrl+C** in the terminal to stop the server.

## Installation

### Download an executable

Download a ZIP from [GitHub Releases](https://github.com/xenoninja/serve0/releases)
that matches your operating system and processor:

| System | Archive OS | Processor |
| --- | --- | --- |
| Linux | `linux` | `amd64` for Intel/AMD 64-bit; `arm64` for ARM64 |
| macOS | `darwin` | `arm64` for Apple silicon; `amd64` for Intel |
| Windows | `windows` | `amd64` for Intel/AMD 64-bit; `arm64` for ARM64 |

Extract the archive and put `serve0` (`serve0.exe` on Windows) on your `PATH`,
or run it directly from the extracted directory:

```sh
# Linux / macOS
./serve0 path/to/mockup.html
```

```powershell
# Windows PowerShell
.\serve0.exe path\to\mockup.html
```

On Linux or macOS, run `chmod +x serve0` if needed. To verify the download,
compare the archive's SHA-256 with the release's `SHA256SUMS` file using
`sha256sum`, `shasum -a 256`, or PowerShell's `Get-FileHash -Algorithm SHA256`.

### Install with Go

Requires Go 1.25 or newer:

```sh
go install github.com/xenoninja/serve0@latest
```

Add `GOBIN` to your `PATH`, or `$(go env GOPATH)/bin` if `GOBIN` is unset.
Replace `@latest` with a published version such as `@vMAJOR.MINOR.PATCH` to
install a specific release.

## Usage

```text
serve0 <page.html> [port]
```

```sh
serve0 mockup.html               # Choose an available port
serve0 mockup.html 0             # Same as omitting the port
serve0 /absolute/mockup.html 8080 # Use a specific port
serve0 --help                    # Show usage (also: -h)
serve0 --version                 # Show the build version
```

Relative page paths resolve from the directory where you run the command. An
explicit port must be available; serve0 exits with an error if it cannot bind
that port.

### Pages and assets

The directory containing your preview page is the **preview directory**. For
example:

```text
preview/
├── mockup.html
├── styles.css
└── images/
    └── logo.png
```

Running `serve0 preview/mockup.html` serves the page at `/`, the stylesheet at
`/styles.css`, and the image at `/images/logo.png`. References such as
`href="styles.css"` and `src="images/logo.png"` work directly.

Every ordinary file beneath the preview directory is available by its URL,
including files the page does not reference. Use a dedicated preview directory
to control what you share. Directory listings, dotfiles, dot-directories, and
links outside the preview directory are blocked. Missing or prohibited paths
return 404; unknown routes do not fall back to the preview page.

Responses use `Cache-Control: no-store`, so an ordinary refresh fetches saved
changes without restarting serve0. It does not open a browser, automatically
reload pages, or build or run an application backend.

### Remote previews

serve0 listens on all IPv4 interfaces **without authentication or TLS**. Use it
only on a trusted network. The printed URLs are candidates: routing, VPN
connectivity, and firewall rules determine which ones your browser can reach.

To keep a preview running across SSH disconnections, start `tmux` or `screen`,
run `serve0 page.html` inside it, and detach. Reattach and press Ctrl+C when you
are done. serve0 stays in the foreground and does not manage a background service.

## Development

Build from the repository root:

```sh
go build -o serve0 .
```

Run the integration tests and static checks:

```sh
go test ./...
go vet ./...
```

Tests build the executable and open local listening sockets. Confinement tests
require symlink privileges; on Windows, enable Developer Mode or run with
permission to create symlinks.

The command implementation lives in `internal/app/`, executable integration
tests in `tests/integration/`, and release packaging in `cmd/package/`.

To build ZIPs for all six supported targets and a `SHA256SUMS` file in `dist/`:

```sh
go run ./cmd/package -version vMAJOR.MINOR.PATCH
```

The release workflow checks reproducible packaging and runs integration tests
on Linux amd64, macOS arm64, and Windows amd64. The remaining targets are
cross-compiled. Pushing a version tag creates a draft GitHub Release for a
maintainer to review and publish.

## Feedback

Report bugs or suggest improvements in
[GitHub Issues](https://github.com/xenoninja/serve0/issues). For bugs, include your
operating system, `serve0 --version` output, the command you ran, and what you
expected to happen.
