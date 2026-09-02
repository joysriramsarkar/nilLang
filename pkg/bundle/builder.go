package bundle

import (
	"archive/zip"
	"fmt"
		"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/config"
)

// Builder creates .nilax bundles
type Builder struct {
	config    *config.ProjectConfig
	projectDir string
	manifest  *Manifest
	files     map[string][]byte // path in bundle -> content
}

// NewBuilder creates a new bundle builder
func NewBuilder(cfg *config.ProjectConfig, projectDir string) *Builder {
	manifest := NewManifest(cfg.Name, cfg.Version, cfg.Author)
	manifest.Targets = cfg.Targets
	manifest.Description = cfg.Description
	manifest.Dependencies = cfg.Dependencies

	return &Builder{
		config:     cfg,
		projectDir: projectDir,
		manifest:   manifest,
		files:      make(map[string][]byte),
	}
}

// AddFile adds a file to the bundle
func (b *Builder) AddFile(bundlePath string, content []byte) {
	b.files[bundlePath] = content
}

// AddFileFromDisk adds a file from the filesystem to the bundle
func (b *Builder) AddFileFromDisk(bundlePath, diskPath string) error {
	data, err := os.ReadFile(diskPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", diskPath, err)
	}
	b.files[bundlePath] = data
	return nil
}

// AddDirectory recursively adds a directory to the bundle
func (b *Builder) AddDirectory(bundlePrefix, diskPath string) error {
	return filepath.Walk(diskPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(diskPath, path)
		if err != nil {
			return err
		}

		bundlePath := filepath.Join(bundlePrefix, filepath.ToSlash(relPath))
		return b.AddFileFromDisk(bundlePath, path)
	})
}

// AddResources adds all resource files defined in the config
func (b *Builder) AddResources() error {
	for _, pattern := range b.config.Resources {
		fullPattern := filepath.Join(b.projectDir, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			return fmt.Errorf("invalid resource pattern %s: %w", pattern, err)
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}

			if info.IsDir() {
				relPath, _ := filepath.Rel(b.projectDir, match)
				bundlePath := filepath.Join("resources", filepath.Base(relPath))
				if err := b.AddDirectory(bundlePath, match); err != nil {
					return err
				}
			} else {
				relPath, _ := filepath.Rel(b.projectDir, match)
				bundlePath := filepath.Join("resources", filepath.ToSlash(relPath))
				if err := b.AddFileFromDisk(bundlePath, match); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Build creates the .nilax bundle file
func (b *Builder) Build(outputPath string) error {
	// Create output directory
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create the ZIP file
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create bundle file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Write manifest
	manifestJSON, err := b.manifest.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}
	if err := b.writeToZip(zipWriter, "manifest.json", manifestJSON); err != nil {
		return err
	}

	// Write all files
	for bundlePath, content := range b.files {
		if err := b.writeToZip(zipWriter, bundlePath, content); err != nil {
			return fmt.Errorf("failed to write %s to bundle: %w", bundlePath, err)
		}
	}

	return nil
}

func (b *Builder) writeToZip(zipWriter *zip.Writer, path string, content []byte) error {
	writer, err := zipWriter.Create(path)
	if err != nil {
		return err
	}
	_, err = writer.Write(content)
	return err
}

// GetBundleInfo returns information about the bundle
func (b *Builder) GetBundleInfo() *BundleInfo {
	totalSize := 0
	fileCount := len(b.files) + 1 // +1 for manifest

	for _, content := range b.files {
		totalSize += len(content)
	}

	return &BundleInfo{
		AppName:   b.config.Name,
		Version:   b.config.Version,
		FileCount: fileCount,
		TotalSize: totalSize,
		Targets:   b.config.Targets,
	}
}

type BundleInfo struct {
	AppName   string
	Version   string
	FileCount int
	TotalSize int
	Targets   []string
}

func (bi *BundleInfo) String() string {
	return fmt.Sprintf("📦 %s v%s | %d files | %d bytes | targets: %s",
		bi.AppName, bi.Version, bi.FileCount, bi.TotalSize,
		strings.Join(bi.Targets, ", "))
}