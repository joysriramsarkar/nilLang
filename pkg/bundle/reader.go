package bundle

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Reader reads .nilax bundles
type Reader struct {
	zipReader io.Closer
	manifest  *Manifest
	files     map[string][]byte
}

// OpenBundle opens a .nilax bundle for reading
func OpenBundle(path string) (*Reader, error) {
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open bundle: %w", err)
	}

	reader := &Reader{
		zipReader: zipReader,
		files:     make(map[string][]byte),
	}

	// Read all files into memory
	for _, file := range zipReader.File {
		rc, err := file.Open()
		if err != nil {
			zipReader.Close()
			return nil, fmt.Errorf("failed to open %s in bundle: %w", file.Name, err)
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			zipReader.Close()
			return nil, fmt.Errorf("failed to read %s in bundle: %w", file.Name, err)
		}

		cleanName := filepath.ToSlash(file.Name)
		reader.files[cleanName] = data

		// Parse manifest
		if cleanName == "manifest.json" {
			manifest, err := FromJSON(data)
			if err != nil {
				zipReader.Close()
				return nil, fmt.Errorf("failed to parse manifest: %w", err)
			}
			reader.manifest = manifest
		}
	}

	if reader.manifest == nil {
		zipReader.Close()
		return nil, ErrInvalidBundle
	}

	return reader, nil
}

// OpenBundleBytes opens a .nilax bundle directly from memory bytes
func OpenBundleBytes(data []byte) (*Reader, error) {
	bytesReader := bytes.NewReader(data)
	zipReader, err := zip.NewReader(bytesReader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open bundle from memory: %w", err)
	}

	reader := &Reader{
		zipReader: nil,
		files:     make(map[string][]byte),
	}

	for _, file := range zipReader.File {
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open %s in bundle: %w", file.Name, err)
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read %s in bundle: %w", file.Name, err)
		}

		cleanName := filepath.ToSlash(file.Name)
		reader.files[cleanName] = content

		if cleanName == "manifest.json" {
			manifest, err := FromJSON(content)
			if err != nil {
				return nil, fmt.Errorf("failed to parse manifest: %w", err)
			}
			reader.manifest = manifest
		}
	}

	if reader.manifest == nil {
		return nil, ErrInvalidBundle
	}

	return reader, nil
}

// Close closes the bundle reader
func (r *Reader) Close() error {
	if r.zipReader != nil {
		return r.zipReader.Close()
	}
	return nil
}

// GetManifest returns the bundle manifest
func (r *Reader) GetManifest() *Manifest {
	return r.manifest
}

// GetFile returns the content of a file in the bundle
func (r *Reader) GetFile(path string) ([]byte, error) {
	norm := filepath.ToSlash(path)
	data, ok := r.files[norm]
	if !ok {
		for k, v := range r.files {
			if filepath.ToSlash(k) == norm || strings.TrimPrefix(filepath.ToSlash(k), "/") == strings.TrimPrefix(norm, "/") {
				return v, nil
			}
		}
		return nil, fmt.Errorf("file not found in bundle: %s", path)
	}
	return data, nil
}

// GetBytecode returns the compiled bytecode from the bundle
func (r *Reader) GetBytecode() ([]byte, error) {
	return r.GetFile(r.manifest.EntryBytecode)
}

// ListFiles returns all file paths in the bundle
func (r *Reader) ListFiles() []string {
	paths := make([]string, 0, len(r.files))
	for path := range r.files {
		paths = append(paths, path)
	}
	return paths
}

// ExtractTo extracts the bundle to a directory
func (r *Reader) ExtractTo(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for path, content := range r.files {
		fullPath := fmt.Sprintf("%s/%s", outputDir, path)
		dir := fullPath[:len(fullPath)-len(filepath.Base(fullPath))]

		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}

		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			return err
		}
	}

	return nil
}
