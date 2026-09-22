package zip

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type Packager struct{}

func (Packager) Pack(ctx context.Context, dir, zipPath string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("zip: read dir: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	out, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("zip: create: %w", err)
	}
	defer func() { _ = out.Close() }()
	w := zip.NewWriter(out)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := addFile(w, filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("zip: close: %w", err)
	}
	return out.Close()
}

func addFile(w *zip.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("zip: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("zip: stat %s: %w", path, err)
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("zip: header %s: %w", path, err)
	}
	header.Name = filepath.Base(path)
	header.Method = zip.Deflate
	dst, err := w.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("zip: entry %s: %w", path, err)
	}
	_, err = io.Copy(dst, f)
	return err
}
