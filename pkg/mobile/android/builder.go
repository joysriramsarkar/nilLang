package android

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/config"
)

//go:embed template/template.zip
var defaultTemplateZip []byte

type AndroidBuildResult struct {
	Path   string
	Size   int64
	Signed bool
}

// BuildAPK builds and packages an Android APK for the given Nilang project.
func BuildAPK(cfg *config.ProjectConfig, projectDir, outputPath string) (*AndroidBuildResult, error) {
	// 1. Determine base template
	var templateData []byte
	customTemplatePath := filepath.Join(projectDir, "android", "template.apk")
	if data, err := os.ReadFile(customTemplatePath); err == nil && len(data) > 0 {
		templateData = data
	} else if rootApk := filepath.Join(projectDir, cfg.Name+".apk"); fileExists(rootApk) {
		if data, err := os.ReadFile(rootApk); err == nil && len(data) > 0 {
			templateData = data
		}
	}

	if len(templateData) == 0 {
		templateData = defaultTemplateZip
	}

	if len(templateData) == 0 {
		return nil, fmt.Errorf("no Android APK template available")
	}

	zr, err := zip.NewReader(bytes.NewReader(templateData), int64(len(templateData)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse base APK template: %w", err)
	}

	// 2. Ensure output directory exists
	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output dir: %w", err)
	}

	tempUnsigned := filepath.Join(outDir, fmt.Sprintf("temp_%s_unsigned.apk", cfg.Name))
	outF, err := os.Create(tempUnsigned)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp APK: %w", err)
	}

	zw := zip.NewWriter(outF)

	// Copy base APK entries except old source assets and signatures
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "assets/app.nil") || strings.HasPrefix(f.Name, "assets/app.nilax") {
			continue
		}
		// Strip old v1 signatures if any so we can resign cleanly
		if strings.HasPrefix(f.Name, "META-INF/") {
			upper := strings.ToUpper(f.Name)
			if strings.HasSuffix(upper, ".SF") || strings.HasSuffix(upper, ".RSA") || strings.HasSuffix(upper, ".DSA") || strings.HasSuffix(upper, ".MF") {
				continue
			}
		}

		header := &zip.FileHeader{
			Name:   f.Name,
			Method: f.Method,
		}
		w, err := zw.CreateHeader(header)
		if err != nil {
			zw.Close()
			outF.Close()
			_ = os.Remove(tempUnsigned)
			return nil, fmt.Errorf("failed to create entry %s: %w", f.Name, err)
		}
		rc, err := f.Open()
		if err != nil {
			zw.Close()
			outF.Close()
			_ = os.Remove(tempUnsigned)
			return nil, fmt.Errorf("failed to open entry %s: %w", f.Name, err)
		}
		_, _ = io.Copy(w, rc)
		rc.Close()
	}

	// Add assets/app.nil (source code)
	entryPath := cfg.GetEntryPath(projectDir)
	if mainSrc, err := os.ReadFile(entryPath); err == nil {
		appNilWriter, err := zw.Create("assets/app.nil")
		if err == nil {
			_, _ = appNilWriter.Write(mainSrc)
		}
	}

	// Add assets/app.nilax (compiled bundle)
	bundlePath := cfg.GetOutputPath(projectDir)
	if bundleBytes, err := os.ReadFile(bundlePath); err == nil {
		appNilaxWriter, err := zw.Create("assets/app.nilax")
		if err == nil {
			_, _ = appNilaxWriter.Write(bundleBytes)
		}
	}

	// Add assets/resources from project resources folder if present
	resDir := filepath.Join(projectDir, "resources")
	if info, err := os.Stat(resDir); err == nil && info.IsDir() {
		_ = filepath.Walk(resDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(projectDir, path)
			if err != nil {
				return nil
			}
			targetInZip := "assets/" + filepath.ToSlash(rel)
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			w, err := zw.Create(targetInZip)
			if err == nil {
				_, _ = w.Write(data)
			}
			return nil
		})
	}

	_ = zw.Close()
	_ = outF.Close()

	// 3. Align and Sign
	zipalignPath := findSDKTool("zipalign")
	apksignerPath := findSDKTool("apksigner")
	keystorePath := findDebugKeystore()

	sourceApkForSigning := tempUnsigned
	tempAligned := filepath.Join(outDir, fmt.Sprintf("temp_%s_aligned.apk", cfg.Name))

	if zipalignPath != "" {
		_ = os.Remove(tempAligned)
		cmdAlign := exec.Command(zipalignPath, "-f", "-p", "4", tempUnsigned, tempAligned)
		if err := cmdAlign.Run(); err == nil {
			sourceApkForSigning = tempAligned
			_ = os.Remove(tempUnsigned)
		}
	}

	isSigned := false
	if apksignerPath != "" && keystorePath != "" {
		_ = os.Remove(outputPath)
		cmdSign := exec.Command(apksignerPath, "sign",
			"--ks", keystorePath,
			"--ks-pass", "pass:android",
			"--ks-key-alias", "androiddebugkey",
			"--key-pass", "pass:android",
			"--out", outputPath,
			sourceApkForSigning,
		)
		if err := cmdSign.Run(); err == nil {
			isSigned = true
		}
	}

	// If signing was skipped or failed, copy unsigned file to outputPath
	if !isSigned {
		_ = os.Remove(outputPath)
		_ = copyFile(sourceApkForSigning, outputPath)
	}

	// Cleanup temp files
	_ = os.Remove(tempUnsigned)
	_ = os.Remove(tempAligned)

	// Stat final output
	stat, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat final APK: %w", err)
	}

	// If root project has <name>.apk, keep it synced as well
	rootApk := filepath.Join(projectDir, cfg.Name+".apk")
	if fileExists(rootApk) && rootApk != outputPath {
		_ = copyFile(outputPath, rootApk)
	}

	return &AndroidBuildResult{
		Path:   outputPath,
		Size:   stat.Size(),
		Signed: isSigned,
	}, nil
}

func findSDKTool(name string) string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	// 1. Check direct path via PATH
	if p, err := exec.LookPath(name + ext); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath(name + ".bat"); err == nil {
			return p
		}
	}

	// 2. Scan standard Android SDK build-tools directories
	var searchBases []string
	if sdk := os.Getenv("ANDROID_HOME"); sdk != "" {
		searchBases = append(searchBases, filepath.Join(sdk, "build-tools"))
	}
	if sdk := os.Getenv("ANDROID_SDK_ROOT"); sdk != "" {
		searchBases = append(searchBases, filepath.Join(sdk, "build-tools"))
	}
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			searchBases = append(searchBases, filepath.Join(localAppData, "Android", "Sdk", "build-tools"))
		}
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			searchBases = append(searchBases, filepath.Join(userProfile, "AppData", "Local", "Android", "Sdk", "build-tools"))
		}
	} else {
		if home := os.Getenv("HOME"); home != "" {
			searchBases = append(searchBases, filepath.Join(home, "Android", "Sdk", "build-tools"))
		}
	}

	for _, base := range searchBases {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		// Check newest version first by traversing in reverse
		for i := len(entries) - 1; i >= 0; i-- {
			if !entries[i].IsDir() {
				continue
			}
			verDir := filepath.Join(base, entries[i].Name())
			if runtime.GOOS == "windows" {
				candidateBat := filepath.Join(verDir, name+".bat")
				if fileExists(candidateBat) {
					return candidateBat
				}
				candidateExe := filepath.Join(verDir, name+".exe")
				if fileExists(candidateExe) {
					return candidateExe
				}
			} else {
				candidate := filepath.Join(verDir, name)
				if fileExists(candidate) {
					return candidate
				}
			}
		}
	}

	return ""
}

func findDebugKeystore() string {
	var candidates []string
	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		candidates = append(candidates, filepath.Join(userProfile, ".android", "debug.keystore"))
	}
	if home := os.Getenv("HOME"); home != "" {
		candidates = append(candidates, filepath.Join(home, ".android", "debug.keystore"))
	}

	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
