package helpers

import (
	"afrita/config"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const planCacheTTL = 60 * time.Second

type tenantPlanCacheEntry struct {
	plan      string
	expiresAt time.Time
}

var tenantPlanCache sync.Map

// GetTenantPlan returns a tenant's current plan, cached for 60 seconds.
func GetTenantPlan(tenantID string) string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return config.PlanSolo
	}

	now := time.Now()
	if cached, ok := tenantPlanCache.Load(tenantID); ok {
		entry, valid := cached.(tenantPlanCacheEntry)
		if valid && now.Before(entry.expiresAt) {
			return entry.plan
		}
		tenantPlanCache.Delete(tenantID)
	}

	plan := config.PlanSolo
	if config.MasterDB == nil {
		log.Printf("⚠️  Master DB unavailable; using solo plan for tenant %q", tenantID)
	} else {
		var dbPlan string
		err := config.MasterDB.QueryRow(
			"SELECT plan FROM tenant_plan WHERE tenant_name = ?",
			tenantID,
		).Scan(&dbPlan)
		switch {
		case err == nil && config.PlanLevel(dbPlan) > 0:
			plan = dbPlan
		case err == sql.ErrNoRows:
			log.Printf("⚠️  No plan found for tenant %q; using solo plan", tenantID)
		case err != nil:
			log.Printf("⚠️  Could not read plan for tenant %q: %v; using solo plan", tenantID, err)
		default:
			log.Printf("⚠️  Unknown plan %q for tenant %q; using solo plan", dbPlan, tenantID)
		}
	}

	tenantPlanCache.Store(tenantID, tenantPlanCacheEntry{
		plan:      plan,
		expiresAt: now.Add(planCacheTTL),
	})
	return plan
}

// IsEnabled reports whether a feature is available to a tenant.
func IsEnabled(tenantID, featureID string) bool {
	feature := config.GetFeature(featureID)
	if feature == nil {
		log.Printf("⚠️  Unknown feature %q", featureID)
		return false
	}
	if feature.AlwaysOn {
		return true
	}

	if config.MasterDB != nil {
		var enabled bool
		err := config.MasterDB.QueryRow(
			`SELECT enabled
			 FROM tenant_feature_override
			 WHERE tenant_name = ? AND feature_id = ?
			   AND (expires_at IS NULL OR expires_at > NOW())
			 LIMIT 1`,
			tenantID,
			featureID,
		).Scan(&enabled)
		switch {
		case err == nil:
			return enabled
		case err != sql.ErrNoRows:
			log.Printf("⚠️  Could not read feature override for tenant %q and feature %q: %v", tenantID, featureID, err)
		}
	}

	plan := GetTenantPlan(tenantID)
	return config.PlanLevel(plan) >= config.PlanLevel(feature.MinPlan)
}

// RequireFeature allows the request through when featureID is enabled and
// renders an upgrade prompt otherwise.
func RequireFeature(w http.ResponseWriter, r *http.Request, tenantID, featureID string) bool {
	feature := config.GetFeature(featureID)
	if feature == nil {
		http.Error(w, "Feature not found", http.StatusNotFound)
		return false
	}
	if IsEnabled(tenantID, featureID) {
		return true
	}

	RenderUpgradePrompt(w, r, featureID, GetTenantPlan(tenantID), feature.MinPlan)
	return false
}

// RenderUpgradePrompt provides a safe fallback until the full upgrade page is
// available. If its cached template exists, it is used directly.
func RenderUpgradePrompt(w http.ResponseWriter, r *http.Request, featureID, currentPlan, requiredPlan string) {
	if config.Templates != nil {
		if _, ok := config.Templates["upgrade-prompt"]; ok {
			Render(w, r, "upgrade-prompt", map[string]interface{}{
				"featureID":    featureID,
				"currentPlan":  currentPlan,
				"requiredPlan": requiredPlan,
			})
			return
		}
	}

	http.Error(
		w,
		fmt.Sprintf("Feature %q requires the %s plan (current plan: %s).", featureID, requiredPlan, currentPlan),
		http.StatusForbidden,
	)
}
