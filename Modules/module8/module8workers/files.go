package module8workers

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxRequestSize = 25 << 20 // 25 MB total request
	maxFileSize    = 10 << 20 // 10 MB per file
	maxFilesPerKey = 5
	uploadDir      = "../../Uploads/03_BudgetReq/"
)

func validateAndSaveFiles(doctype, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, fileHeaders []*multipart.FileHeader, dstDir string, projectName string) ([]SavedFile, error) {

	dirPath := filepath.Join(dstDir, projectName)
	if err := os.MkdirAll(dirPath, 0o700); err != nil {
		return nil, fmt.Errorf("no se pudo crear el directorio de subida")
	}

	var out []SavedFile

	for _, fh := range fileHeaders {
		saved, err := validateAndSaveOneFile(doctype, apiKey, client, w, r, fh, dirPath)
		if err != nil {
			return nil, err
		}
		out = append(out, saved)
	}

	return out, nil
}

func validateAndSaveOneFile(doctype, apiKey string, client *http.Client, w http.ResponseWriter, r *http.Request, fh *multipart.FileHeader, dstDir string) (SavedFile, error) {

	_, username, _ := getUserInfo(apiKey, client, w, r)

	var result SavedFile

	if fh.Size <= 0 {
		return result, fmt.Errorf("archivo vacío no permitido")
	}
	if fh.Size > maxFileSize {
		return result, fmt.Errorf("el archivo %q supera el tamaño máximo permitido", fh.Filename)
	}

	originalName := filepath.Base(fh.Filename)

	ext := strings.ToLower(filepath.Ext(originalName))
	if !allowedExtensions[ext] {
		return result, fmt.Errorf("extensión no permitida en %q", originalName)
	}

	src, err := fh.Open()
	if err != nil {
		return result, fmt.Errorf("no se pudo abrir %q", originalName)
	}
	defer src.Close()

	header := make([]byte, 512)
	n, err := io.ReadFull(src, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return result, fmt.Errorf("no se pudo inspeccionar %q", originalName)
	}
	header = header[:n]

	detectedMime := http.DetectContentType(header)
	if !allowedMimeTypes[detectedMime] {
		return result, fmt.Errorf("tipo MIME no permitido en %q (%s)", originalName, detectedMime)
	}

	if !mimeMatchesExtension(detectedMime, ext) {
		return result, fmt.Errorf("el contenido de %q no coincide con su extensión", originalName)
	}
	timestamp := time.Now().Format("20060102_150405")
	storedName := doctype + "_" + username + "_" + timestamp + ext
	dstPath := filepath.Join(dstDir, storedName)

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return result, fmt.Errorf("no se pudo crear el archivo destino")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, bytes.NewReader(header)); err != nil {
		_ = os.Remove(dstPath)
		return result, fmt.Errorf("error guardando %q", originalName)
	}

	remaining := maxFileSize - int64(len(header))
	written, err := io.Copy(dst, io.LimitReader(src, remaining+1))
	if err != nil {
		_ = os.Remove(dstPath)
		return result, fmt.Errorf("error guardando %q", originalName)
	}
	if int64(len(header))+written > maxFileSize {
		_ = os.Remove(dstPath)
		return result, fmt.Errorf("el archivo %q excede el tamaño máximo", originalName)
	}

	result = SavedFile{
		OriginalName: originalName,
		StoredName:   storedName,
		Path:         dstPath,
		Size:         int64(len(header)) + written,
		MIME:         detectedMime,
	}
	return result, nil
}

func mimeMatchesExtension(mimeType, ext string) bool {
	switch ext {
	case ".pdf":
		return mimeType == "application/pdf"
	case ".png":
		return mimeType == "image/png"
	case ".jpg", ".jpeg":
		return mimeType == "image/jpeg"
	case ".webp":
		return mimeType == "image/webp"
	default:
		return false
	}
}
