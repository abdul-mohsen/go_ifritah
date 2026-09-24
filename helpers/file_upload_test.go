package helpers

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"afrita/config"
)

func TestUploadPurchaseBillFilesReturnsBackendReferences(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/upload" {
			t.Fatalf("path = %q, want /api/v2/upload", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer upload-token" {
			t.Fatalf("authorization = %q, want bearer token", got)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read uploaded file: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"download_url":"/api/v2/files/` + header.Filename + `","file_key":"` + header.Filename + `","file_size":` + strconv.Itoa(len(data)) + `}`))
	}))
	defer backend.Close()

	originalDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = originalDomain })

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writeMultipartFile(t, writer, "bill_pdf", "invoice.pdf", []byte("pdf"))
	writeMultipartFile(t, writer, "documents", "photo.jpg", []byte("image"))
	if err := writer.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/purchase-bills", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	pdfLink, attachments, err := UploadPurchaseBillFiles(req, "upload-token")
	if err != nil {
		t.Fatalf("UploadPurchaseBillFiles: %v", err)
	}
	if pdfLink == nil || *pdfLink != "/api/v2/files/invoice.pdf" {
		t.Fatalf("pdf link = %v, want invoice URL", pdfLink)
	}
	if len(attachments) != 1 || attachments[0] != "/api/v2/files/photo.jpg" {
		t.Fatalf("attachments = %#v, want photo URL", attachments)
	}
}

func TestUploadPurchaseBillFilesPreservesBackendError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"UPLOAD_REJECTED","detail":"ملف غير صالح"}`))
	}))
	defer backend.Close()

	originalDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = originalDomain })

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writeMultipartFile(t, writer, "bill_pdf", "invoice.pdf", []byte("pdf"))
	if err := writer.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/purchase-bills", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, _, err := UploadPurchaseBillFiles(req, "upload-token")
	if err == nil {
		t.Fatal("expected backend upload error")
	}
	uploadErr, ok := err.(*FileUploadError)
	if !ok {
		t.Fatalf("error type = %T, want *FileUploadError", err)
	}
	if uploadErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", uploadErr.StatusCode, http.StatusBadRequest)
	}
	if !strings.Contains(string(uploadErr.Body), "UPLOAD_REJECTED") {
		t.Fatalf("body = %q, want backend error code", uploadErr.Body)
	}
}

func writeMultipartFile(t *testing.T, writer *multipart.Writer, field, name string, content []byte) {
	t.Helper()
	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatalf("CreateFormFile(%s): %v", field, err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write %s: %v", field, err)
	}
}
