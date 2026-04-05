package helpers

import (
	"afrita/config"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestFetchSuppliersReal(t *testing.T) {
	if os.Getenv("RUN_REAL_TESTS") == "" {
		t.Skip("Debug test — requires RUN_REAL_TESTS=1 env var")
	}
	config.BackendDomain = "https://dev.ifritah.com"

	// Login to get a real token
	loginBody := `{"username":"ssda","password":"Qwerty123"}`
	resp, err := http.Post(config.BackendDomain+"/api/v2/login", "application/json", strings.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login returned status %d", resp.StatusCode)
	}

	var loginResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResp.AccessToken == "" {
		t.Fatal("no access token received")
	}
	t.Logf("Got token: %s...", loginResp.AccessToken[:20])

	// Now call FetchSuppliers
	suppliers, err := FetchSuppliers(loginResp.AccessToken)
	if err != nil {
		t.Fatalf("FetchSuppliers FAILED: %v", err)
	}
	t.Logf("FetchSuppliers returned %d suppliers", len(suppliers))
	for i, s := range suppliers {
		t.Logf("  supplier[%d]: id=%d name=%q phone=%q", i, s.ID, s.Name, s.PhoneNumber)
		if i >= 4 {
			break
		}
	}

	// Also write to file for inspection
	data, _ := json.MarshalIndent(suppliers, "", "  ")
	os.WriteFile("../tmp/suppliers_fetched.json", data, 0644)
	fmt.Printf("Wrote %d suppliers to tmp/suppliers_fetched.json\n", len(suppliers))
}
