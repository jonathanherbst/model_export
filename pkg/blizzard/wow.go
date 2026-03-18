package blizzard

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v84/github"
)

func IsWOWCasc(casc *Casc) bool {
	return strings.HasPrefix(casc.ProductName, "wow")
}

func OpenWOWCasc(casc *Casc, cachePath string) (*WOWCasc, error) {
	if !IsWOWCasc(casc) {
		panic("attemted to open a wow casc that isn's a wow casc")
	}

	listfilePath, err := WOWGetLatestListfile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open wow casc: %w", err)
	}
	casc.ListFilePath = &listfilePath

	dbdPath, err := WOWGetLatestDBD(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open wow casc: %w", err)
	}

	tables := make(map[string]string)
	for file_data := range func(yield func(FileData) bool) { casc.SearchFiles("*.db2", yield) } {
		sanitized_name := strings.ReplaceAll(file_data.Name, "\\", "/")
		table_name := strings.TrimSuffix(path.Base(sanitized_name), ".db2")
		tables[table_name] = file_data.Name
	}

	dbd_repo, err := OpenDBDZipRepo(dbdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open wow casc: %w", err)
	}

	return &WOWCasc{casc, dbd_repo, tables}, nil
}

type WOWCasc struct {
	casc     *Casc
	dbd_repo *DBDZipRepo
	tables   map[string]string
}

func (wow *WOWCasc) Close() {
	wow.casc.Close()
	wow.dbd_repo.Close()
}

func (wow *WOWCasc) GetTables(yield func(string) bool) {
	for k := range wow.tables {
		if !yield(k) {
			return
		}
	}
}

func WOWGetLatestListfile(cachePath string) (string, error) {
	destPath := filepath.Join(cachePath, "wow-listfile.csv")
	if err := downloadLatestGHRelease("wowdev", "wow-listfile", "verified-listfile-withcapitals.csv", destPath); err != nil {
		return "", err
	}
	return destPath, nil
}

func WOWGetLatestDBD(cachePath string) (string, error) {
	destPath := filepath.Join(cachePath, "wow-dbd.zip")
	if err := downloadLatestGHRelease("wowdev", "WoWDBDefs", "dbd.zip", destPath); err != nil {
		return "", err
	}
	return destPath, nil
}

func downloadLatestGHRelease(owner, repo, assetName, destPath string) error {
	ctx := context.Background()
	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return err
	}
	var assetURL string
	var expectedSHA256 string
	for _, asset := range release.Assets {
		if *asset.Name == assetName {
			assetURL = *asset.BrowserDownloadURL
			if asset.Digest != nil && strings.HasPrefix(*asset.Digest, "sha256:") {
				expectedSHA256 = (*asset.Digest)[7:]
			}
			break
		}
	}
	if assetURL == "" {
		return fmt.Errorf("asset %s not found in latest release", assetName)
	}

	// Check if file exists and SHA256 matches
	if expectedSHA256 != "" {
		actualSHA256, err := computeFileSHA256(destPath)
		if err == nil && actualSHA256 == expectedSHA256 {
			return nil // Already have the correct file
		}
	}

	resp, err := http.Get(assetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

func computeFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
