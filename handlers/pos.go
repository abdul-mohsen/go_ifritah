package handlers

import (
	"log"
	"net/http"
	"strconv"

	"afrita/helpers"
	"afrita/models"
	"afrita/resources"
)

// HandlePOS renders the standalone cashier workspace. Product loading remains
// client-side so the search field can debounce requests without re-rendering
// the full page.
func HandlePOS(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	stores, err := helpers.FetchStores(token)
	storeError := ""
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		log.Printf("POS store loading failed: %v", err)
		storeError = resources.L("pos.store_load_error")
		stores = []models.Store{}
	}

	selectedStoreID := 0
	if rawID := r.URL.Query().Get("store_id"); rawID != "" {
		selectedStoreID, _ = strconv.Atoi(rawID)
	}

	storeExists := false
	for _, store := range stores {
		if store.ID == selectedStoreID {
			storeExists = true
			break
		}
	}
	if !storeExists && len(stores) > 0 {
		selectedStoreID = stores[0].ID
	}

	helpers.Render(w, r, "pos", map[string]interface{}{
		"title":            resources.L("pos.title"),
		"stores":           stores,
		"default_store_id": selectedStoreID,
		"store_error":      storeError,
	})
}
