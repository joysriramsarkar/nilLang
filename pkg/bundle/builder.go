package bundle

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/config"
)

// Builder creates .nilax bundles
type Builder struct {
	config     *config.ProjectConfig
	projectDir string
	manifest   *Manifest
	files      map[string][]byte // path in bundle -> content

	// buildTime, when set via SetBuildTime, replaces the wall clock for the
	// manifest timestamp and every zip entry's mtime, so two builds of the
	// same inputs are byte-identical (reproducible build, L9.6).
	buildTime    time.Time
	hasBuildTime bool
}

// NewBuilder creates a new bundle builder
func NewBuilder(cfg *config.ProjectConfig, projectDir string) *Builder {
	manifest := NewManifest(cfg.Name, cfg.Version, cfg.Author)
	manifest.Targets = cfg.Targets
	manifest.Description = cfg.Description
	manifest.Dependencies = cfg.Dependencies
	manifest.Permissions = cfg.Capabilities
	manifest.Metadata = cfg.Metadata

	return &Builder{
		config:     cfg,
		projectDir: projectDir,
		manifest:   manifest,
		files:      make(map[string][]byte),
	}
}

// AddFile adds a file to the bundle
func (b *Builder) AddFile(bundlePath string, content []byte) {
	b.files[filepath.ToSlash(bundlePath)] = content
}

// AddFileFromDisk adds a file from the filesystem to the bundle
func (b *Builder) AddFileFromDisk(bundlePath, diskPath string) error {
	data, err := os.ReadFile(diskPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", diskPath, err)
	}
	b.files[filepath.ToSlash(bundlePath)] = data
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

		bundlePath := filepath.ToSlash(filepath.Join(bundlePrefix, relPath))
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

// SetBuildTime pins the manifest timestamp and zip entry mtimes, making
// Build deterministic. A zero value is ignored.
func (b *Builder) SetBuildTime(t time.Time) {
	if t.IsZero() {
		return
	}
	b.buildTime = t.UTC()
	b.hasBuildTime = true
}

// Build creates the .nilax bundle file
func (b *Builder) Build(outputPath string) error {
	// Create output directory
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Deterministic manifest timestamp (L9.6): without this, two builds of
	// identical inputs differ by the wall clock embedded in manifest.json.
	if b.hasBuildTime {
		b.manifest.CreatedAt = b.buildTime
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

	// Write all files in sorted order: Go map iteration is random, so an
	// unsorted walk makes entry order (and the output hash) differ per build.
	names := make([]string, 0, len(b.files))
	for name := range b.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, bundlePath := range names {
		if err := b.writeToZip(zipWriter, bundlePath, b.files[bundlePath]); err != nil {
			return fmt.Errorf("failed to write %s to bundle: %w", bundlePath, err)
		}
	}

	return nil
}

func (b *Builder) writeToZip(zipWriter *zip.Writer, path string, content []byte) error {
	// With a pinned build time every entry carries the same mtime, so the
	// archive bytes are a pure function of the inputs. Without one, keep the
	// previous behaviour (library-default timestamp).
	if !b.hasBuildTime {
		writer, err := zipWriter.Create(path)
		if err != nil {
			return err
		}
		_, err = writer.Write(content)
		return err
	}
	header := &zip.FileHeader{
		Name:     path,
		Method:   zip.Deflate,
		Modified: b.buildTime,
	}
	header.SetMode(0644)
	writer, err := zipWriter.CreateHeader(header)
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
