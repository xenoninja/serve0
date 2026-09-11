package integration_test

import (
	"bufio"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "serve0-test-")
	if err != nil {
		panic(err)
	}
	// Keep the executable discoverable through PATHEXT on Windows.
	binary = filepath.Join(dir, "serve0.exe")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = filepath.Join("..", "..")
	if supplied := os.Getenv("SERVE0_TEST_BINARY"); supplied != "" {
		// Resolve relative overrides from the repository root, as before relocation.
		if !filepath.IsAbs(supplied) {
			supplied = filepath.Join(build.Dir, supplied)
		}
		binary, err = filepath.Abs(supplied)
		if err != nil {
			panic(err)
		}
		code := m.Run()
		os.RemoveAll(dir)
		os.Exit(code)
	}
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", output, err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type preview struct {
	cmd     *exec.Cmd
	done    chan error
	urls    []string
	stopped bool
}

func start(t *testing.T, dir string, args ...string) *preview {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	preparePreviewProcess(cmd)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &preview{cmd: cmd, done: make(chan error, 1)}
	lines := make(chan string, 64)
	go func() {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	go func() { p.done <- cmd.Wait() }()
	t.Cleanup(func() {
		if !p.stopped {
			cmd.Process.Kill()
			select {
			case <-p.done:
			case <-time.After(5 * time.Second):
				t.Error("process did not exit")
			}
		}
	})
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("startup ended without ready message")
			}
			if strings.HasPrefix(line, "http://") {
				p.urls = append(p.urls, line)
			}
			if strings.Contains(line, "Ctrl+C") {
				if len(p.urls) == 0 {
					t.Fatal("no URLs")
				}
				return p
			}
		case <-timer.C:
			t.Fatal("startup timed out")
		}
	}
}

func fixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	page := filepath.Join(dir, "chosen.html")
	if err := os.WriteFile(page, []byte("<h1>chosen</h1>"), 0600); err != nil {
		t.Fatal(err)
	}
	return dir, page
}

var client = &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}

func get(t *testing.T, url string, status int, body string) http.Header {
	t.Helper()
	res, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != status || (body != "" && string(data) != body) {
		t.Fatalf("GET %s: %d %q", url, res.StatusCode, data)
	}
	return res.Header
}
func TestPreviewLifecycle(t *testing.T) {
	dir, page := fixture(t)
	p := start(t, dir, filepath.Base(page))
	h := get(t, p.urls[0], 200, "<h1>chosen</h1>")
	if !strings.HasPrefix(h.Get("Content-Type"), "text/html") {
		t.Fatal(h)
	}
	get(t, p.urls[0]+"chosen.html", 200, "")
	if err := os.WriteFile(page, []byte("<h1>edited</h1>"), 0600); err != nil {
		t.Fatal(err)
	}
	get(t, p.urls[0], 200, "<h1>edited</h1>")
	if h.Get("Cache-Control") != "no-store" {
		t.Fatal("refresh may use stale cache", h)
	}
	if err := interruptPreviewProcess(p.cmd.Process); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-p.done:
		p.stopped = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timed out")
	}
	address := strings.TrimSuffix(strings.TrimPrefix(p.urls[0], "http://"), "/")
	conn, err := net.DialTimeout("tcp4", address, time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("listener survived shutdown")
	}
}

func invoke(t *testing.T, dir string, success bool, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if (err == nil) != success {
			t.Fatalf("args %q: %v: %s", args, err, output.String())
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		<-done
		t.Fatalf("args %q did not exit", args)
	}
	if !success && (strings.Contains(output.String(), "http://") || strings.TrimSpace(output.String()) == "") {
		t.Fatalf("bad diagnostic: %s", output.String())
	}
	return output.String()
}

func TestInvalidInvocationsAndHelp(t *testing.T) {
	dir, page := fixture(t)
	for _, args := range [][]string{nil, {"missing.html"}, {dir}, {page, "-1"}, {page, "65536"}, {page, "1.5"}, {page, "abc"}, {page, "0", "extra"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) { invoke(t, dir, false, args...) })
	}
	for _, flag := range []string{"-h", "--help"} {
		out := invoke(t, dir, true, flag)
		for _, word := range []string{"[port]", "zero", "Ctrl+C", "root", "foreground"} {
			if !strings.Contains(out, word) {
				t.Errorf("help missing %q: %s", word, out)
			}
		}
	}
}

func TestSelectedPageBoundary(t *testing.T) {
	dir, page := fixture(t)
	hidden := filepath.Join(dir, ".secret.html")
	if err := os.WriteFile(hidden, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	invoke(t, dir, false, hidden)
	outside := filepath.Join(t.TempDir(), "outside.html")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.html")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	invoke(t, dir, false, link)
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(hidden, link); err != nil {
		t.Fatal(err)
	}
	invoke(t, dir, false, link)
	p := start(t, dir, page, "0")
	for _, target := range []string{outside, hidden} {
		if err := os.Remove(page); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, page); err != nil {
			t.Fatal(err)
		}
		get(t, p.urls[0], 404, "")
	}
	if err := os.Remove(page); err != nil {
		t.Fatal(err)
	}
	get(t, p.urls[0], 404, "")
	if err := os.Mkdir(page, 0700); err != nil {
		t.Fatal(err)
	}
	get(t, p.urls[0], 404, "")
}

func TestPortsAndAddresses(t *testing.T) {
	dir, page := fixture(t)
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	local := map[string]bool{}
	for _, address := range addresses {
		ip, _, err := net.ParseCIDR(address.String())
		if err == nil && ip.To4() != nil {
			local[ip.String()] = true
		}
	}
	for _, port := range []string{"0", "explicit"} {
		t.Run(port, func(t *testing.T) {
			requested := port
			if port == "explicit" {
				l, err := net.Listen("tcp4", "0.0.0.0:0")
				if err != nil {
					t.Fatal(err)
				}
				requested = fmt.Sprint(l.Addr().(*net.TCPAddr).Port)
				l.Close()
			}
			p := start(t, dir, page, requested)
			connectedNonLoopback := false
			for _, url := range p.urls {
				address := strings.TrimSuffix(strings.TrimPrefix(url, "http://"), "/")
				host, actual, err := net.SplitHostPort(address)
				if err != nil {
					t.Fatal(err)
				}
				if !local[host] || net.ParseIP(host).IsUnspecified() || actual == "0" {
					t.Fatalf("invalid candidate %s", url)
				}
				if port == "explicit" && actual != requested {
					t.Fatalf("port %s, want %s", actual, requested)
				}
				get(t, url, 200, "<h1>chosen</h1>")
				if !net.ParseIP(host).IsLoopback() {
					connectedNonLoopback = true
				}
			}
			if !connectedNonLoopback {
				t.Log("no non-loopback IPv4 candidate available")
			}
		})
	}
	held, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	port := fmt.Sprint(held.Addr().(*net.TCPAddr).Port)
	output := invoke(t, dir, false, page, port)
	if !strings.Contains(output, "bind") && !strings.Contains(output, "listen") {
		t.Fatal(output)
	}
	held.Close()
	rebound, err := net.Listen("tcp4", ":"+port)
	if err != nil {
		t.Fatalf("failed start retained listener: %v", err)
	}
	rebound.Close()
}

func TestReadablePageAndInternalSymlink(t *testing.T) {
	dir, page := fixture(t)
	link := filepath.Join(dir, "alias.html")
	if err := os.Symlink(page, link); err != nil {
		t.Fatal(err)
	}
	p := start(t, dir, link)
	get(t, p.urls[0], 200, "<h1>chosen</h1>")
	if err := os.Chmod(page, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(page, 0600)
	if f, err := os.Open(page); err == nil {
		f.Close()
		t.Skip("environment can read mode-000 files")
	}
	invoke(t, dir, false, page)
	get(t, p.urls[0], 404, "")
}

func TestPreviewDirectoryRename(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "preview")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(dir, "page.html")
	if err := os.WriteFile(page, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	p := start(t, parent, page)
	if runtime.GOOS == "windows" {
		// os.OpenRoot holds a Windows handle without FILE_SHARE_DELETE.
		if err := os.Rename(dir, dir+"-moved"); err == nil {
			t.Fatal("renamed an open preview directory on Windows")
		}
		get(t, p.urls[0], 200, "original")
		return
	}
	if err := os.Rename(dir, dir+"-moved"); err != nil {
		t.Fatal(err)
	}
	get(t, p.urls[0], 200, "original")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	get(t, p.urls[0], 200, "original")
}

func TestAbsoluteSymlinksThroughDirectoryAlias(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "preview")
	writeFile(t, filepath.Join(dir, "chosen.html"), "chosen preview")
	alias := filepath.Join(parent, "directory-alias")
	symlink(t, dir, alias)
	symlink(t, filepath.Join(alias, "chosen.html"), filepath.Join(dir, "page-alias.html"))
	symlink(t, alias, filepath.Join(dir, "self"))
	for _, selectedDir := range []string{dir, alias} {
		t.Run(filepath.Base(selectedDir), func(t *testing.T) {
			p := start(t, parent, filepath.Join(selectedDir, "page-alias.html"))
			get(t, p.urls[0], 200, "chosen preview")
			get(t, p.urls[0]+"self/page-alias.html", 200, "chosen preview")
		})
	}
}

func TestConcurrentReplacementDoesNotExposeHiddenFile(t *testing.T) {
	for _, route := range []string{"", "chosen.html"} {
		t.Run("/"+route, func(t *testing.T) {
			dir, page := fixture(t)
			hidden := filepath.Join(dir, ".secret.html")
			if err := os.WriteFile(hidden, []byte("secret"), 0600); err != nil {
				t.Fatal(err)
			}
			p := start(t, dir, page)
			// Separate staging names prevent a failed rename from leaving a symlink
			// that a later WriteFile could follow and overwrite the hidden marker.
			swapLink := filepath.Join(dir, "swap-link")
			swapFile := filepath.Join(dir, "swap-file")
			symlink(t, ".secret.html", swapLink)
			if err := os.Remove(swapLink); err != nil {
				t.Fatal(err)
			}
			stop := make(chan struct{})
			type replacements struct {
				hidden, public int
				lastError      error
			}
			done := make(chan replacements, 1)
			go func() {
				result := replacements{}
				defer func() { done <- result }()
				for {
					select {
					case <-stop:
						return
					default:
					}
					if err := os.Symlink(".secret.html", swapLink); err != nil {
						result.lastError = err
					} else if err := os.Rename(swapLink, page); err != nil {
						result.lastError = err
					} else {
						result.hidden++
					}
					if err := os.Remove(swapLink); err != nil && !os.IsNotExist(err) {
						result.lastError = err
					}
					if err := os.WriteFile(swapFile, []byte("public"), 0600); err != nil {
						result.lastError = err
					} else if err := os.Rename(swapFile, page); err != nil {
						result.lastError = err
					} else {
						result.public++
					}
				}
			}()
			defer func() {
				close(stop)
				result := <-done
				if result.hidden == 0 || result.public == 0 {
					t.Errorf("confinement replacements were not exercised: hidden=%d public=%d last error=%v", result.hidden, result.public, result.lastError)
				} else if result.lastError != nil {
					t.Logf("concurrent replacement encountered transient filesystem error: %v", result.lastError)
				}
			}()
			for i := 0; i < 200; i++ {
				res, err := client.Get(p.urls[0] + route)
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(res.Body)
				res.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if res.StatusCode != 200 && res.StatusCode != 404 {
					t.Fatalf("unexpected status %d", res.StatusCode)
				}
				if strings.Contains(string(data), "secret") {
					t.Fatal("exposed hidden preview page")
				}
			}
		})
	}
}

func TestPreviewAssetsAndRefresh(t *testing.T) {
	dir, page := fixture(t)
	writeFile(t, filepath.Join(dir, "style.css"), "body { color: red; }")
	p := start(t, t.TempDir(), page)
	h := get(t, p.urls[0]+"style.css", 200, "body { color: red; }")
	if !strings.HasPrefix(h.Get("Content-Type"), "text/css") || h.Get("Cache-Control") != "no-store" {
		t.Fatal(h)
	}
	writeFile(t, filepath.Join(dir, "style.css"), "body { color: blue; }")
	get(t, p.urls[0]+"style.css", 200, "body { color: blue; }")
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestNestedAndUnreferencedPreviewAssets(t *testing.T) {
	dir, page := fixture(t)
	files := []struct{ path, body, contentType string }{
		{"assets/theme.css", "h1 { color: green; }", "text/css"},
		{"app.js", "document.title = 'preview';", "text/javascript"},
		{"assets/icon.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>", "image/svg+xml"},
		{"assets/font.woff2", "wOF2", "font/woff2"},
		{"unreferenced.txt", "neighbor", "text/plain"},
		{"assets/space name.txt", "space", "text/plain"},
	}
	for _, file := range files {
		writeFile(t, filepath.Join(dir, file.path), file.body)
	}
	p := start(t, t.TempDir(), page)
	for _, file := range files {
		t.Run(file.path, func(t *testing.T) {
			h := get(t, p.urls[0]+file.path, 200, file.body)
			typ, _, err := mime.ParseMediaType(h.Get("Content-Type"))
			if err != nil || (typ != file.contentType && !(file.path == "app.js" && typ == "application/javascript")) {
				t.Fatal(h)
			}
			if h.Get("Cache-Control") != "no-store" {
				t.Fatal(h)
			}
		})
	}
	for _, path := range []string{"assets", "assets/", "missing", "missing.html", "assets/missing.css"} {
		get(t, p.urls[0]+path, 404, "404 page not found\n")
	}
}

func symlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("confinement test requires symlinks (on Windows enable Developer Mode or run elevated): %v", err)
	}
}

func TestPreviewAssetBoundary(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "preview")
	page := filepath.Join(dir, "chosen.html")
	writeFile(t, page, "public")
	writeFile(t, filepath.Join(parent, "outside.txt"), "OUTSIDE MARKER")
	writeFile(t, filepath.Join(dir, ".secret"), "HIDDEN MARKER")
	writeFile(t, filepath.Join(dir, ".hidden", "secret.txt"), "HIDDEN MARKER")
	writeFile(t, filepath.Join(dir, "nested", "public.txt"), "public asset")
	symlink(t, "../outside.txt", filepath.Join(dir, "escape.txt"))
	symlink(t, parent, filepath.Join(dir, "escape-dir"))
	symlink(t, ".secret", filepath.Join(dir, "hidden-alias"))
	symlink(t, ".hidden", filepath.Join(dir, "hidden-dir"))
	symlink(t, "nested/public.txt", filepath.Join(dir, "alias.txt"))
	symlink(t, "../alias.txt", filepath.Join(dir, "nested", "alias.txt"))
	// The hidden component must be checked before cleaning the symlink target.
	symlink(t, ".hidden/../nested/public.txt", filepath.Join(dir, "hidden-hop.txt"))
	p := start(t, t.TempDir(), page)
	for _, path := range []string{
		".secret", "%2esecret", ".hidden/secret.txt", "%2ehidden/secret.txt",
		"../outside.txt", "%2e%2e/outside.txt", "%2e%2e%2foutside.txt",
		"nested/../../outside.txt", "nested/%2e%2e/%2e%2e/outside.txt",
		"%252e%252e/outside.txt", "..%5coutside.txt", "%2f../outside.txt",
		"nested/./public.txt", "nested//public.txt", "nested/%00public.txt",
		"C:%5cWindows%5cwin.ini", "chosen.html::$DATA",
		"escape.txt", "escape-dir/outside.txt", "hidden-alias", "hidden-dir/secret.txt", "hidden-hop.txt",
	} {
		t.Run(path, func(t *testing.T) { get(t, p.urls[0]+path, 404, "404 page not found\n") })
	}
	get(t, p.urls[0]+"alias.txt", 200, "public asset")
	get(t, p.urls[0]+"nested/alias.txt", 200, "public asset")
}

func TestSymlinkParentResolution(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		t.Run(fmt.Sprintf("absolute=%t", absolute), func(t *testing.T) {
			dir, page := fixture(t)
			writeFile(t, filepath.Join(dir, "nested", "child", "placeholder"), "")
			writeFile(t, filepath.Join(dir, "nested", "asset.html"), "correct preview")
			writeFile(t, filepath.Join(dir, "asset.html"), "wrong preview")
			symlink(t, filepath.Join("nested", "child"), filepath.Join(dir, "jump"))
			// Join would erase the parent component that this test exercises.
			target := "jump" + string(filepath.Separator) + ".." + string(filepath.Separator) + "asset.html"
			if absolute {
				target = dir + string(filepath.Separator) + target
			}
			alias := filepath.Join(dir, "alias.html")
			symlink(t, target, alias)
			p := start(t, dir, page)
			get(t, p.urls[0]+"alias.html", 200, "correct preview")
			selected := start(t, dir, alias)
			get(t, selected.urls[0], 200, "correct preview")
			symlink(t, dir, filepath.Join(dir, "directory-alias"))
			get(t, p.urls[0]+"directory-alias/alias.html", 200, "correct preview")

			// A parent component must not erase a prohibited or missing hop.
			outside := t.TempDir()
			writeFile(t, filepath.Join(outside, "child", "placeholder"), "")
			writeFile(t, filepath.Join(outside, "asset.html"), "outside preview")
			writeFile(t, filepath.Join(dir, ".hidden", "child", "placeholder"), "")
			writeFile(t, filepath.Join(dir, ".hidden", "asset.html"), "hidden preview")
			for _, destination := range []string{
				filepath.Join(outside, "child"),
				filepath.Join(".hidden", "child"),
				"missing",
				"..",
			} {
				if err := os.Remove(filepath.Join(dir, "jump")); err != nil {
					t.Fatal(err)
				}
				symlink(t, destination, filepath.Join(dir, "jump"))
				get(t, p.urls[0]+"alias.html", 404, "404 page not found\n")
				get(t, selected.urls[0], 404, "404 page not found\n")
			}
		})
	}
}

func TestPreviewAssetTargetsChangeWhileRunning(t *testing.T) {
	dir, page := fixture(t)
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "asset.txt"), "OUTSIDE MARKER")
	writeFile(t, filepath.Join(dir, ".hidden", "asset.txt"), "HIDDEN MARKER")
	writeFile(t, filepath.Join(dir, "public", "asset.txt"), "public asset")
	alias := filepath.Join(dir, "alias")
	symlink(t, "public", alias)
	p := start(t, t.TempDir(), page)
	get(t, p.urls[0]+"alias/asset.txt", 200, "public asset")
	for _, target := range []string{outside, ".hidden", "public"} {
		if err := os.Remove(alias); err != nil {
			t.Fatal(err)
		}
		symlink(t, target, alias)
		status, body := 404, "404 page not found\n"
		if target == "public" {
			status, body = 200, "public asset"
		}
		get(t, p.urls[0]+"alias/asset.txt", status, body)
	}
	// Replace the target itself, leaving the alias unchanged.
	asset := filepath.Join(dir, "public", "asset.txt")
	for _, target := range []string{filepath.Join(outside, "asset.txt"), filepath.Join(dir, ".hidden", "asset.txt")} {
		if err := os.Remove(asset); err != nil {
			t.Fatal(err)
		}
		symlink(t, target, asset)
		get(t, p.urls[0]+"alias/asset.txt", 404, "404 page not found\n")
	}
	if err := os.Remove(asset); err != nil {
		t.Fatal(err)
	}
	writeFile(t, asset, "restored")
	get(t, p.urls[0]+"alias/asset.txt", 200, "restored")
	// A real directory can also be replaced with a link during the preview.
	if err := os.Rename(filepath.Join(dir, "public"), filepath.Join(dir, "moved")); err != nil {
		t.Fatal(err)
	}
	symlink(t, outside, filepath.Join(dir, "public"))
	get(t, p.urls[0]+"alias/asset.txt", 404, "404 page not found\n")
}

func TestRefreshIgnoresConditionalDates(t *testing.T) {
	dir, page := fixture(t)
	asset := filepath.Join(dir, "style.css")
	writeFile(t, asset, "body { color: red; }")
	p := start(t, dir, page)
	get(t, p.urls[0]+"style.css", 200, "body { color: red; }")
	writeFile(t, asset, "body { color: blue; }")
	writeFile(t, page, "<h1>edited</h1>")
	for path, want := range map[string]string{"": "<h1>edited</h1>", "style.css": "body { color: blue; }"} {
		req, err := http.NewRequest(http.MethodGet, p.urls[0]+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("If-Modified-Since", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != 200 || string(body) != want || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("refresh %s: status %d, body %q, headers %v, error %v", path, res.StatusCode, body, res.Header, err)
		}
	}
}

func TestVersionWithoutPreviewPage(t *testing.T) {
	out := invoke(t, t.TempDir(), true, "--version")
	if !strings.HasPrefix(out, "serve0 ") || len(strings.Fields(out)) < 2 || strings.Contains(out, "http://") {
		t.Fatalf("invalid version output: %q", out)
	}
	if want := os.Getenv("SERVE0_TEST_VERSION"); want != "" && strings.TrimSpace(out) != "serve0 "+want {
		t.Fatalf("version %q, want %q", out, want)
	}
	help := invoke(t, t.TempDir(), true, "--help")
	if !strings.Contains(help, "--version") || !strings.Contains(help, "--help") {
		t.Fatalf("help missing command flags: %s", help)
	}
}
