package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	return p.openPath(p.name, 0)
}

// Walk one component at a time, retaining directory handles. Identity checks
// prevent a concurrent symlink replacement from changing the validated target.
func (p *previewPage) openPath(path string, links int) (*os.File, error) {
	if links > 40 {
		return nil, fmt.Errorf("too many page symlinks")
	}
	if filepath.IsAbs(path) {
		// Preserve hidden components before Rel/Join can clean them away.
		// Hidden ancestors of the preview directory are outside the policy.
		if hiddenPath(strings.TrimPrefix(path, p.directory+string(filepath.Separator))) {
			return nil, fmt.Errorf("hidden preview paths are not served")
		}
		var err error
		path, err = filepath.Rel(p.directory, path)
		if err != nil {
			return nil, err
		}
	}
	if !filepath.IsLocal(path) {
		return nil, fmt.Errorf("page is outside the preview directory")
	}
	if hiddenPath(path) {
		return nil, fmt.Errorf("hidden preview paths are not served")
	}
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	current := p.root
	for i, part := range parts {
		info, err := current.Lstat(part)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := current.Readlink(part)
			if err != nil {
				return nil, err
			}
			if !filepath.IsAbs(target) {
				if hiddenPath(target) {
					return nil, fmt.Errorf("hidden preview paths are not served")
				}
				target = filepath.Join(append(parts[:i], target)...)
			}
			if i+1 < len(parts) {
				target += string(filepath.Separator) + filepath.Join(parts[i+1:]...)
			}
			return p.openPath(target, links+1)
		}
		if i < len(parts)-1 {
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
			current = next
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

func hiddenPath(path string) bool {
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") && part != "." && part != ".." {
			return true
		}
	}
	return false
}
