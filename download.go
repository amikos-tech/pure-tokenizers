package tokenizers

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	GitHubRepo      = "amikos-tech/pure-tokenizers"
	DefaultTag      = "latest"
	DownloadTimeout = 30 * time.Second
)

// getPlatformAssetName returns the expected asset name for the current platform
func getPlatformAssetName() string {
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	default:
		arch = runtime.GOARCH
	}

	var platform string
	switch runtime.GOOS {
	case "darwin":
		platform = "apple-darwin"
	case "linux":
		if isMusl() {
			platform = "unknown-linux-musl"
		} else {
			platform = "unknown-linux-gnu"
		}
	case "windows":
		platform = "pc-windows-msvc"
	default:
		platform = runtime.GOOS
	}

	return fmt.Sprintf("libtokenizers-%s-%s.tar.gz", arch, platform)
}

// DownloadLibraryFromGitHub downloads the platform-specific library from GitHub releases
func DownloadLibraryFromGitHub(destPath string) error {
	version := getVersionTag()
	return DownloadLibraryFromGitHubWithVersion(destPath, version)
}

// downloadFile downloads a file from the given URL to the destination path
func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: DownloadTimeout}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download from %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", dest, err)
	}
	defer func() {
		_ = out.Close()
	}()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", dest, err)
	}

	return nil
}

// verifyChecksum verifies the SHA256 checksum of the downloaded file
func verifyChecksum(filePath, checksumData string) error {
	// Read the expected checksum

	expectedChecksum := strings.TrimSpace(checksumData)
	// Handle format like "abc123  filename.tar.gz"
	if parts := strings.Fields(expectedChecksum); len(parts) >= 1 {
		expectedChecksum = parts[0]
	}

	// Calculate actual checksum
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// extractLibrary extracts the shared library from the tar.gz archive
func extractLibrary(archivePath, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		_ = gzr.Close()
	}()

	tr := tar.NewReader(gzr)
	libraryName := getLibraryName()

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Look for the library file (could be in subdirectories)
		if strings.HasSuffix(header.Name, libraryName) {
			// Extract this file to the destination
			outFile, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer func() {
				_ = outFile.Close()
			}()

			if _, err := io.Copy(outFile, tr); err != nil {
				return fmt.Errorf("failed to extract library: %w", err)
			}

			// Make the library executable
			if err := os.Chmod(destPath, 0755); err != nil {
				return fmt.Errorf("failed to set library permissions: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("library file %s not found in archive", libraryName)
}

// GitHub API structures
type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest,omitempty"` // Optional field for checksums
}

// fetchLatestRelease fetches the latest release information from GitHub API
func fetchLatestRelease(url string) (*GitHubRelease, error) {
	client := &http.Client{Timeout: DownloadTimeout}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API request failed with status %d: %s (%s)", resp.StatusCode, resp.Status, url)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release JSON: %w", err)
	}

	return &release, nil
}

// getGitHubRepo returns the GitHub repository to download from
func getGitHubRepo() string {
	if repo := os.Getenv("TOKENIZERS_GITHUB_REPO"); repo != "" {
		return repo
	}
	return GitHubRepo
}

// getVersionTag returns the version tag to download
func getVersionTag() string {
	if tag := os.Getenv("TOKENIZERS_VERSION"); tag != "" {
		return tag
	}
	return DefaultTag
}

// DownloadLibraryFromGitHubWithVersion downloads a specific version of the library
func DownloadLibraryFromGitHubWithVersion(destPath, version string) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	repo := getGitHubRepo()
	var releaseURL string

	if version == "latest" || version == "" {
		releaseURL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	} else {
		releaseURL = fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, version)
	}

	release, err := fetchLatestRelease(releaseURL)
	if err != nil {
		return fmt.Errorf("failed to fetch release %s: %w", version, err)
	}

	return downloadAndExtractLibrary(release, destPath)
}

// downloadAndExtractLibrary handles the common download and extraction logic
func downloadAndExtractLibrary(release *GitHubRelease, destPath string) error {
	assetName := getPlatformAssetName()
	// Find the asset URLs
	var assetURL, assetChecksum string
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			assetURL = asset.BrowserDownloadURL
			assetChecksum = asset.Digest // Optional checksum field
		}
	}

	if assetURL == "" {
		return fmt.Errorf("asset %s not found in release %s", assetName, release.TagName)
	}

	// Create temporary files for download
	tempDir := filepath.Join(os.TempDir(), "tokenizers-download")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir) // Clean up temp files
	}()

	tempAsset := filepath.Join(tempDir, assetName)

	// Download both files
	if err := downloadFile(assetURL, tempAsset); err != nil {
		return fmt.Errorf("failed to download asset: %w", err)
	}

	// Verify checksum
	if assetChecksum != "" {
		tempChecksum := strings.SplitAfter(assetChecksum, "sha256:")[1]
		if err := verifyChecksum(tempAsset, tempChecksum); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Extract the library from the tar.gz file
	if err := extractLibrary(tempAsset, destPath); err != nil {
		return fmt.Errorf("failed to extract library: %w", err)
	}

	return nil
}

// DownloadAndCacheLibrary downloads and caches the library for the current platform
func DownloadAndCacheLibrary() error {
	cacheDir := getCacheDir()
	cachedPath := filepath.Join(cacheDir, getLibraryName())

	// Check if already cached and valid
	if isLibraryValid(cachedPath) {
		return nil
	}

	return DownloadLibraryFromGitHub(cachedPath)
}

// DownloadAndCacheLibraryWithVersion downloads and caches a specific version of the library
func DownloadAndCacheLibraryWithVersion(version string) error {
	cacheDir := getCacheDir()
	cachedPath := filepath.Join(cacheDir, getLibraryName())

	return DownloadLibraryFromGitHubWithVersion(cachedPath, version)
}

// GetCachedLibraryPath returns the path where the library would be cached
func GetCachedLibraryPath() string {
	cacheDir := getCacheDir()
	return filepath.Join(cacheDir, getLibraryName())
}

// ClearLibraryCache removes the cached library file
func ClearLibraryCache() error {
	cachedPath := GetCachedLibraryPath()
	if _, err := os.Stat(cachedPath); os.IsNotExist(err) {
		return nil // Already doesn't exist
	}
	return os.Remove(cachedPath)
}

// GetAvailableVersions fetches available versions from GitHub releases
func GetAvailableVersions() ([]string, error) {
	repo := getGitHubRepo()
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", repo)

	client := &http.Client{Timeout: DownloadTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases JSON: %w", err)
	}

	versions := make([]string, len(releases))
	for i, release := range releases {
		versions[i] = release.TagName
	}

	return versions, nil
}

// IsLibraryCached checks if the library is already cached and valid
func IsLibraryCached() bool {
	cachedPath := GetCachedLibraryPath()
	return isLibraryValid(cachedPath)
}

// GetLibraryInfo returns information about the current library setup
func GetLibraryInfo() map[string]interface{} {
	info := make(map[string]interface{})

	info["platform_asset_name"] = getPlatformAssetName()
	info["library_name"] = getLibraryName()
	info["cache_path"] = GetCachedLibraryPath()
	info["cache_dir"] = getCacheDir()
	info["is_cached"] = IsLibraryCached()
	info["github_repo"] = getGitHubRepo()
	info["version"] = getVersionTag()

	// Check environment variables
	env := make(map[string]string)
	if path := os.Getenv("TOKENIZERS_LIB_PATH"); path != "" {
		env["TOKENIZERS_LIB_PATH"] = path
	}
	if repo := os.Getenv("TOKENIZERS_GITHUB_REPO"); repo != "" {
		env["TOKENIZERS_GITHUB_REPO"] = repo
	}
	if version := os.Getenv("TOKENIZERS_VERSION"); version != "" {
		env["TOKENIZERS_VERSION"] = version
	}
	info["environment"] = env

	return info
}
