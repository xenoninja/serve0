package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// previewPage retains the preview directory even if its original pathname changes.
type previewPage struct {
	root            *os.Root
	directory, name string
}

func selectPage(path string) (*previewPage, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	directory, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	page := &previewPage{root: root, directory: directory, name: filepath.Base(absolute)}
	f, err := page.open()
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("preview page %q: %w", path, err)
	}
	f.Close()
	return page, nil
}

func (p *previewPage) open() (*os.File, error) {
	return p.openPath(p.name)
}

// Walk one component at a time, retaining directory handles. Identity checks
// prevent a concurrent symlink replacement from changing the validated target.
func (p *previewPage) openPath(path string) (*os.File, error) {
	parts, err := p.pathParts(path)
	if err != nil {
		return nil, err
	}
	directories := []*os.Root{p.root}
	links := 0
	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(directories) == 1 {
				return nil, fmt.Errorf("page is outside the preview directory")
			}
			directories = directories[:len(directories)-1]
			continue
		}
		current := directories[len(directories)-1]
		info, err := current.Lstat(part)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			links++
			if links > 40 {
				return nil, fmt.Errorf("too many page symlinks")
			}
			target, err := current.Readlink(part)
			if err != nil {
				return nil, err
			}
			targetParts, err := p.pathParts(target)
			if err != nil {
				return nil, err
			}
			if filepath.IsAbs(target) {
				directories = directories[:1]
			}
			// Resolve the target before processing its remaining parent components.
			parts = append(targetParts, parts...)
			continue
		}
		if len(parts) > 0 {
			if !info.IsDir() {
				return nil, fmt.Errorf("page path component must be a directory")
			}
			next, err := current.OpenRoot(part)
			if err != nil {
				return nil, err
			}
			defer next.Close()
			opened, err := next.Stat(".")
			if err != nil || !os.SameFile(info, opened) {
				return nil, fmt.Errorf("preview directory changed while opening page")
			}
			directories = append(directories, next)
			continue
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("page must be a regular file")
		}
		f, err := current.Open(part)
		if err != nil {
			return nil, err
		}
		opened, err := f.Stat()
		if err != nil || !os.SameFile(info, opened) {
			f.Close()
			return nil, fmt.Errorf("preview page changed while opening")
		}
		return f, nil
	}
	return nil, fmt.Errorf("page must be a regular file")
}

// Split without cleaning: a symlink before ".." changes which parent it names.
func (p *previewPage) pathParts(path string) ([]string, error) {
	path = filepath.FromSlash(path)
	if filepath.IsAbs(path) {
		if path == p.directory || (runtime.GOOS == "windows" && strings.EqualFold(path, p.directory)) {
			return []string{"."}, nil
		}
		prefix := strings.TrimRight(p.directory, string(filepath.Separator)) + string(filepath.Separator)
		inside := strings.HasPrefix(path, prefix)
		if runtime.GOOS == "windows" && len(path) >= len(prefix) {
			inside = strings.EqualFold(path[:len(prefix)], prefix)
		}
		if !inside {
			return nil, fmt.Errorf("page is outside the preview directory")
		}
		path = path[len(prefix):]
	} else if filepath.VolumeName(path) != "" || strings.HasPrefix(path, string(filepath.Separator)) {
		return nil, fmt.Errorf("page is outside the preview directory")
	}
	if hiddenPath(path) {
		return nil, fmt.Errorf("hidden preview paths are not served")
	}
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		if part != "" && part != "." && part != ".." && !filepath.IsLocal(part) {
			return nil, fmt.Errorf("page is outside the preview directory")
		}
	}
	return parts, nil
}

func hiddenPath(path string) bool {
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") && part != "." && part != ".." {
			return true
		}
	}
	return false
}
