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
