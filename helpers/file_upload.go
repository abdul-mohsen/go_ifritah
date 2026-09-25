package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"afrita/config"
)

// FileUploadError preserves the backend upload response so callers can return
// the backend's status and user-facing validation message unchanged.
type FileUploadError struct {
	StatusCode int
	Body       []byte
	Err        error
}

func (e *FileUploadError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("file upload failed with status %d", e.StatusCode)
}

type purchaseBillUploadResponse struct {
	FileKey     string `json:"file_key"`
	DownloadURL string `json:"download_url"`
}

// UploadPurchaseBillFiles uploads the files selected in the purchase-bill
// multipart form before the JSON create request is sent. The backend create
// endpoint accepts references, not browser file bytes.
func UploadPurchaseBillFiles(r *http.Request, token string) (*string, []string, error) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		return nil, []string{}, nil
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, nil, fmt.Errorf("parse purchase bill upload form: %w", err)
	}
	if r.MultipartForm == nil {
		return nil, []string{}, nil
	}

	var pdfLink *string
	if files := r.MultipartForm.File["bill_pdf"]; len(files) > 0 {
		ref, err := uploadPurchaseBillFile(files[0], token)
		if err != nil {
			return nil, nil, err
		}
		pdfLink = &ref
	}

	attachments := make([]string, 0, len(r.MultipartForm.File["documents"]))
	for _, fileHeader := range r.MultipartForm.File["documents"] {
		ref, err := uploadPurchaseBillFile(fileHeader, token)
		if err != nil {
			return nil, nil, err
		}
		attachments = append(attachments, ref)
	}

	return pdfLink, attachments, nil
}

func uploadPurchaseBillFile(fileHeader *multipart.FileHeader, token string) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("open purchase bill attachment: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileHeader.Filename)
	if err != nil {
		return "", fmt.Errorf("create purchase bill upload part: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("read purchase bill attachment: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("finish purchase bill upload: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		config.BackendDomain+"/api/v2/upload",
		bytes.NewReader(body.Bytes()),
	)
	if err != nil {
		return "", fmt.Errorf("create purchase bill upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return "", fmt.Errorf("send purchase bill upload: %w", err)
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", fmt.Errorf("read purchase bill upload response: %w", readErr)
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", &FileUploadError{
			StatusCode: resp.StatusCode,
			Body:       responseBody,
			Err:        fmt.Errorf("backend rejected purchase bill upload"),
		}
	}

	var uploaded purchaseBillUploadResponse
	if err := json.Unmarshal(responseBody, &uploaded); err != nil {
		return "", fmt.Errorf("decode purchase bill upload response: %w", err)
	}
	if uploaded.DownloadURL != "" {
		return uploaded.DownloadURL, nil
	}
	if uploaded.FileKey != "" {
		return "/api/v2/files/" + uploaded.FileKey, nil
	}
	return "", fmt.Errorf("purchase bill upload response has no file reference")
}
