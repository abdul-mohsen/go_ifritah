package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"
)

func TestHandleNotFound_Returns404(t *testing.T) {
	config.Initialize()

	req := httptest.NewRequest("GET", "/nonexistent-page", nil)
	rr := httptest.NewRecorder()

	HandleNotFound(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("HandleNotFound should return 404, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "404") {
		t.Error("404 page should contain the error code")
	}
	if !strings.Contains(body, "الصفحة غير موجودة") {
		t.Error("404 page should contain Arabic message")
	}
}

func TestHandleMethodNotAllowed_Returns405(t *testing.T) {
	config.Initialize()

	req := httptest.NewRequest("DELETE", "/dashboard", nil)
	rr := httptest.NewRecorder()

	HandleMethodNotAllowed(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("HandleMethodNotAllowed should return 405, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "405") {
		t.Error("405 page should contain the error code")
	}
}

func TestRenderErrorPage_CustomCode(t *testing.T) {
	config.Initialize()

	rr := httptest.NewRecorder()
	RenderErrorPage(rr, "503", http.StatusServiceUnavailable, "الخدمة غير متاحة", "جاري الصيانة")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("RenderErrorPage should return 503, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "503") {
		t.Error("Error page should contain the custom code")
	}
	if !strings.Contains(body, "جاري الصيانة") {
		t.Error("Error page should contain the custom message")
	}
}
