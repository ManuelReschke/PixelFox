package jobqueue

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
)

func isLocalLikeStoragePool(pool *models.StoragePool) bool {
	if pool == nil {
		return false
	}
	return pool.StorageType == models.StorageTypeLocal || pool.StorageType == models.StorageTypeNFS
}

func buildReplicateURL(uploadAPIURL string) (string, error) {
	repURL := strings.TrimSpace(uploadAPIURL)
	if repURL == "" {
		return "", fmt.Errorf("target pool missing upload_api_url for replication")
	}
	repURL = strings.TrimRight(repURL, "/")
	if strings.HasSuffix(repURL, "/upload") {
		return strings.TrimSuffix(repURL, "/upload") + "/replicate", nil
	}
	return repURL + "/replicate", nil
}

func normalizeVariantRelativePath(filePath string, sourcePool *models.StoragePool) string {
	rel := filepath.ToSlash(strings.TrimSpace(filePath))
	if rel == "" {
		return ""
	}
	if idx := strings.Index(rel, "variants"); idx >= 0 {
		return strings.TrimLeft(rel[idx:], "/")
	}
	if sourcePool != nil {
		base := filepath.ToSlash(strings.TrimRight(sourcePool.BasePath, string(filepath.Separator)))
		if base != "" && strings.HasPrefix(rel, base+"/") {
			rel = strings.TrimPrefix(rel, base+"/")
		}
	}
	return strings.TrimLeft(rel, "/")
}

func replicateFileToRemotePool(sourceFullPath, storedPath string, targetPoolID uint, uploadAPIURL string) error {
	info, err := os.Stat(sourceFullPath)
	if err != nil {
		return fmt.Errorf("stat source failed: %w", err)
	}

	file, err := os.Open(sourceFullPath)
	if err != nil {
		return fmt.Errorf("open source failed: %w", err)
	}

	repURL, err := buildReplicateURL(uploadAPIURL)
	if err != nil {
		_ = file.Close()
		return err
	}

	cleanStoredPath := path.Clean("/" + filepath.ToSlash(storedPath))
	cleanStoredPath = strings.TrimPrefix(cleanStoredPath, "/")

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer file.Close()
		defer pw.Close()
		defer mw.Close()

		_ = mw.WriteField("pool_id", fmt.Sprintf("%d", targetPoolID))
		_ = mw.WriteField("stored_path", cleanStoredPath)
		_ = mw.WriteField("size", fmt.Sprintf("%d", info.Size()))

		part, err := mw.CreateFormFile("file", path.Base(cleanStoredPath))
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		hasher := sha256.New()
		tee := io.TeeReader(file, hasher)
		if _, err := io.Copy(part, tee); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = mw.WriteField("sha256", hex.EncodeToString(hasher.Sum(nil)))
	}()

	client := &http.Client{Timeout: 300 * time.Second}
	req, err := http.NewRequest(http.MethodPut, repURL, pr)
	if err != nil {
		_ = pw.CloseWithError(err)
		<-writerDone
		return fmt.Errorf("create replicate request failed: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	secret := strings.TrimSpace(env.GetEnv("REPLICATION_SECRET", ""))
	if secret == "" {
		_ = pw.CloseWithError(fmt.Errorf("missing replication secret"))
		<-writerDone
		return fmt.Errorf("REPLICATION_SECRET is not set")
	}
	req.Header.Set("Authorization", "Bearer "+secret)

	resp, err := client.Do(req)
	if err != nil {
		_ = pw.CloseWithError(err)
		<-writerDone
		return fmt.Errorf("replicate HTTP error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		_ = pw.CloseWithError(fmt.Errorf("bad status %d", resp.StatusCode))
		<-writerDone
		return fmt.Errorf("replicate failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	<-writerDone
	return nil
}
