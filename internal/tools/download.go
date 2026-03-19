package tools

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/phm-dev/phm/internal/httputil"
)

// DownloadFile downloads a file from URL to the specified path with progress
func DownloadFile(destPath string, url string) error {
	resp, err := httputil.Client.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Simple progress indicator
	contentLength := resp.ContentLength
	if contentLength > 0 {
		fmt.Printf("    Downloading %.2f MB...\n", float64(contentLength)/1024/1024)
	}

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if contentLength > 0 && written != contentLength {
		return fmt.Errorf("incomplete download: got %d bytes, expected %d", written, contentLength)
	}

	return nil
}

// ExtractTarGz extracts a .tar.gz file and returns the extracted binary path
func ExtractTarGz(tarPath, destDir, expectedBinaryName string) (string, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	var extractedBinary string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Clean the path to handle both ./binary and binary patterns
		cleanName := strings.TrimPrefix(header.Name, "./")
		target := filepath.Join(destDir, cleanName)

		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("path traversal detected: %q escapes %q", cleanName, destDir)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			// Create parent directory if needed
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}

			outFile, err := os.Create(target)
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()

			// Preserve executable permissions
			if header.Mode&0111 != 0 {
				if err := os.Chmod(target, 0755); err != nil {
					return "", err
				}
			}

			// Check if this is our binary
			if filepath.Base(cleanName) == expectedBinaryName {
				extractedBinary = target
			}
		}
	}

	if extractedBinary == "" {
		return "", fmt.Errorf("binary %s not found in archive", expectedBinaryName)
	}

	return extractedBinary, nil
}

// SetExecutable sets the executable bit on a file
func SetExecutable(path string) error {
	return os.Chmod(path, 0755)
}
