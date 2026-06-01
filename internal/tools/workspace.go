package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Workspace struct {
	Root string
}

func (w Workspace) ResolveRead(name string) (string, error) {
	return w.resolveSafe(name, false)
}

func (w Workspace) ResolveWrite(name string) (string, error) {
	return w.resolveSafe(name, true)
}

func (w Workspace) resolveSafe(name string, write bool) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	requested := filepath.Join(w.Root, filepath.Clean(name))
	relative, err := filepath.Rel(w.Root, requested)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes workspace")
	}
	if err := os.MkdirAll(w.Root, 0o755); err != nil {
		return "", err
	}
	existing := requested
	if write {
		existing = filepath.Dir(requested)
		nearest, err := nearestExisting(existing)
		if err != nil {
			return "", err
		}
		if err := w.assertInsideRoot(nearest); err != nil {
			return "", err
		}
		if err := os.MkdirAll(existing, 0o755); err != nil {
			return "", err
		}
		if info, err := os.Lstat(requested); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("writing through symlinks is not allowed")
		}
	}
	if err := w.assertInsideRoot(existing); err != nil {
		return "", err
	}
	return requested, nil
}

func (w Workspace) assertInsideRoot(existing string) error {
	rootReal, err := filepath.EvalSymlinks(w.Root)
	if err != nil {
		return err
	}
	existingReal, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(rootReal, existingReal)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("resolved path escapes workspace")
	}
	return nil
}

func nearestExisting(name string) (string, error) {
	current := name
	for {
		if _, err := os.Lstat(current); err == nil {
			return current, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing parent for %s", name)
		}
		current = parent
	}
}
