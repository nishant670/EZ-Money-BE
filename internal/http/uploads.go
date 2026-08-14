package http

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// uploadDir is the on-disk root for receipts. Deployments mount a persistent
// volume here; without one the files vanish on redeploy.
const uploadDir = "uploads"

// sniffLength is what http.DetectContentType reads, and is also enough to cover
// the ISO base media file format header used by HEIC.
const sniffLength = 512

// allowedUploadTypes maps an accepted media type to the extension we store it
// under. The extension is always derived from the sniffed bytes, never from the
// client-supplied filename, so a file cannot be served back as something that
// executes in a browser.
var allowedUploadTypes = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/webp":      ".webp",
	"image/heic":      ".heic",
	"application/pdf": ".pdf",
}

// heicBrands are the ISO-BMFF major brands that indicate HEIC/HEIF payloads.
// http.DetectContentType does not recognise them and reports octet-stream.
var heicBrands = map[string]bool{
	"heic": true,
	"heix": true,
	"hevc": true,
	"hevx": true,
	"mif1": true,
	"msf1": true,
}

func (s *Server) handleUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		if requestBodyTooLarge(err) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file provided"})
		return
	}
	if file.Size > s.cfg.MaxUploadMB*1024*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file_too_large"})
		return
	}

	extension, err := detectUploadExtension(file)
	if err != nil {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{
			"error":  "unsupported_file_type",
			"fields": gin.H{"file": "attach a JPEG, PNG, HEIC, WebP image or a PDF"},
		})
		return
	}

	name, err := randomUploadName(extension)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	path := filepath.Join(uploadDir, name)
	if err := c.SaveUploadedFile(file, path); err != nil {
		log.Printf("failed_save_upload err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": fmt.Sprintf("%s/%s/%s", publicOrigin(c.Request), uploadDir, name)})
}

// detectUploadExtension sniffs the leading bytes and returns the extension the
// file may be stored under. The client's declared Content-Type is ignored.
func detectUploadExtension(file *multipart.FileHeader) (string, error) {
	opened, err := file.Open()
	if err != nil {
		return "", err
	}
	defer opened.Close()

	header := make([]byte, sniffLength)
	read, err := io.ReadFull(opened, header)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	header = header[:read]

	if isHEIC(header) {
		return allowedUploadTypes["image/heic"], nil
	}

	mediaType, _, _ := strings.Cut(http.DetectContentType(header), ";")
	extension, ok := allowedUploadTypes[strings.TrimSpace(mediaType)]
	if !ok {
		return "", fmt.Errorf("unsupported upload type %q", mediaType)
	}
	return extension, nil
}

// isHEIC reports whether the bytes carry an ISO-BMFF ftyp box with a HEIC brand,
// which is what iPhone cameras produce by default.
func isHEIC(header []byte) bool {
	if len(header) < 12 {
		return false
	}
	if string(header[4:8]) != "ftyp" {
		return false
	}
	return heicBrands[string(header[8:12])]
}

// randomUploadName returns an unguessable filename. Receipts are served from a
// static route with no authentication, so the name is the only thing keeping
// one user's documents from being enumerable.
func randomUploadName(extension string) (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer) + extension, nil
}
