package resources

import "testing"

func TestReleaseNoteLabelsHaveArabicAndEnglishText(t *testing.T) {
	keys := []string{
		"release_note.title",
		"release_note.close",
		"release_note.intro",
		"release_note.capability_sales",
		"release_note.capability_inventory",
		"release_note.capability_reports",
		"release_note.zatca_pending",
		"release_note.continue",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			label := GetLabel(key)
			if label.Ar == "" || label.Ar == key {
				t.Fatalf("Arabic release note label %q is missing", key)
			}
			if label.En == "" || label.En == key {
				t.Fatalf("English release note label %q is missing", key)
			}
		})
	}
}
