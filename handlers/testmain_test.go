package handlers

import (
	"afrita/config"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Set BaseDir to project root so templates are found from handlers/ test dir
	config.BaseDir = ".."
	config.LoadTemplates()
	os.Exit(m.Run())
}

// seedTestSession registers the canonical "test-session" cookie used by
// most handler tests. Lives here (instead of the deleted users_page_test.go)
// so all *_test.go files can share it.
func seedTestSession() {
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()
}
