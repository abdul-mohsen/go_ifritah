package handlers

// salesImportEnabled is false until the duplicate-check preview is live. While it is false, every sales import (XLSX,
// CSV and the data import) answers 503 before it reads its upload, and the sales import buttons are disabled.
var salesImportEnabled bool

// salesImportOffMessage answers a refused sales import. The import pages show the same text on their disabled buttons.
const salesImportOffMessage = "Sales import is off until the duplicate check is live."
