//go:build ignore
// +build ignore

// Disabled: depends on NewMockStoreAddressStore and models.NationalAddress which are not yet implemented.

package handlers

import (
	"afrita/models"
	"testing"
)

// ============================================================================
// Store National Address Tests
// ============================================================================

func TestStoreAddressStoreSetAndGet(t *testing.T) {
	store := NewMockStoreAddressStore()

	addr := models.NationalAddress{
		BuildingNumber:   "1234",
		StreetName:       "شارع الملك فهد",
		District:         "العليا",
		City:             "الرياض",
		PostalCode:       "12345",
		AdditionalNumber: "6789",
		UnitNumber:       "10",
	}

	store.Set(1, addr)

	got, found := store.Get(1)
	if !found {
		t.Fatal("expected to find address for store 1")
	}
	if got.BuildingNumber != "1234" {
		t.Errorf("BuildingNumber = %q, want %q", got.BuildingNumber, "1234")
	}
	if got.StreetName != "شارع الملك فهد" {
		t.Errorf("StreetName = %q, want %q", got.StreetName, "شارع الملك فهد")
	}
	if got.District != "العليا" {
		t.Errorf("District = %q, want %q", got.District, "العليا")
	}
	if got.City != "الرياض" {
		t.Errorf("City = %q, want %q", got.City, "الرياض")
	}
	if got.PostalCode != "12345" {
		t.Errorf("PostalCode = %q, want %q", got.PostalCode, "12345")
	}
	if got.AdditionalNumber != "6789" {
		t.Errorf("AdditionalNumber = %q, want %q", got.AdditionalNumber, "6789")
	}
	if got.UnitNumber != "10" {
		t.Errorf("UnitNumber = %q, want %q", got.UnitNumber, "10")
	}
}

func TestStoreAddressStoreGetNotFound(t *testing.T) {
	store := NewMockStoreAddressStore()

	_, found := store.Get(999)
	if found {
		t.Fatal("expected not to find address for non-existent store")
	}
}

func TestStoreAddressStoreUpdate(t *testing.T) {
	store := NewMockStoreAddressStore()

	addr := models.NationalAddress{
		BuildingNumber: "1111",
		City:           "جدة",
	}
	store.Set(5, addr)

	updated := models.NationalAddress{
		BuildingNumber: "2222",
		City:           "الدمام",
		StreetName:     "شارع الأمير محمد",
	}
	store.Set(5, updated)

	got, found := store.Get(5)
	if !found {
		t.Fatal("expected to find updated address")
	}
	if got.BuildingNumber != "2222" {
		t.Errorf("BuildingNumber = %q, want %q", got.BuildingNumber, "2222")
	}
	if got.City != "الدمام" {
		t.Errorf("City = %q, want %q", got.City, "الدمام")
	}
	if got.StreetName != "شارع الأمير محمد" {
		t.Errorf("StreetName = %q, want %q", got.StreetName, "شارع الأمير محمد")
	}
}

func TestStoreAddressStoreDelete(t *testing.T) {
	store := NewMockStoreAddressStore()

	addr := models.NationalAddress{City: "الرياض"}
	store.Set(3, addr)

	store.Delete(3)

	_, found := store.Get(3)
	if found {
		t.Fatal("expected address to be deleted")
	}
}

func TestStoreAddressStoreEmptyAddress(t *testing.T) {
	store := NewMockStoreAddressStore()

	// Empty address is valid (all fields optional)
	addr := models.NationalAddress{}
	store.Set(10, addr)

	got, found := store.Get(10)
	if !found {
		t.Fatal("expected to find empty address")
	}
	if got.City != "" {
		t.Errorf("City = %q, want empty", got.City)
	}
}
