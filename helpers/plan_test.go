package helpers

import (
	"afrita/config"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

const (
	tenantPlanTestQuery      = "SELECT plan FROM tenant_plan WHERE tenant_name = ?"
	featureOverrideTestQuery = "SELECT enabled FROM tenant_feature_override WHERE tenant_name = ? AND feature_id = ? AND (expires_at IS NULL OR expires_at > NOW()) LIMIT 1"
)

func TestGetTenantPlanReturnsCatalogPlans(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		plan     string
		useDB    bool
	}{
		{name: "solo fallback", tenantID: "solo-tenant", plan: config.PlanSolo},
		{name: "growth", tenantID: "growth-tenant", plan: config.PlanGrowth, useDB: true},
		{name: "business", tenantID: "business-tenant", plan: config.PlanBusiness, useDB: true},
		{name: "enterprise", tenantID: "enterprise-tenant", plan: config.PlanEnterprise, useDB: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearPlanTestCache()

			var mock sqlmock.Sqlmock
			if tt.useDB {
				mock = setupPlanTestDB(t)
				mock.ExpectQuery(regexp.QuoteMeta(tenantPlanTestQuery)).
					WithArgs(tt.tenantID).
					WillReturnRows(sqlmock.NewRows([]string{"plan"}).AddRow(tt.plan))
			} else {
				previousDB := config.MasterDB
				config.MasterDB = nil
				t.Cleanup(func() {
					config.MasterDB = previousDB
				})
			}

			if got := GetTenantPlan(tt.tenantID); got != tt.plan {
				t.Fatalf("GetTenantPlan(%q) = %q, want %q", tt.tenantID, got, tt.plan)
			}

			if mock != nil {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatalf("SQL expectations were not met: %v", err)
				}
			}
		})
	}
}

func TestIsEnabledRespectsFeatureOverrides(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		featureID      string
		overrideResult *bool
		fallbackPlan   string
		want           bool
	}{
		{
			name:           "active enabled override grants enterprise feature",
			tenantID:       "enabled-override",
			featureID:      config.FeatureChartOfAccounts,
			overrideResult: boolPointer(true),
			fallbackPlan:   config.PlanSolo,
			want:           true,
		},
		{
			name:           "active disabled override denies enterprise feature",
			tenantID:       "disabled-override",
			featureID:      config.FeatureChartOfAccounts,
			overrideResult: boolPointer(false),
			fallbackPlan:   config.PlanEnterprise,
			want:           false,
		},
		{
			name:         "expired override falls back to plan",
			tenantID:     "expired-override",
			featureID:    config.FeatureChartOfAccounts,
			fallbackPlan: config.PlanGrowth,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearPlanTestCache()
			mock := setupPlanTestDB(t)

			expectation := mock.ExpectQuery(regexp.QuoteMeta(featureOverrideTestQuery)).
				WithArgs(tt.tenantID, tt.featureID)
			if tt.overrideResult == nil {
				expectation.WillReturnError(sql.ErrNoRows)
			} else {
				expectation.WillReturnRows(
					sqlmock.NewRows([]string{"enabled"}).AddRow(*tt.overrideResult),
				)
			}

			if tt.overrideResult == nil {
				mock.ExpectQuery(regexp.QuoteMeta(tenantPlanTestQuery)).
					WithArgs(tt.tenantID).
					WillReturnRows(sqlmock.NewRows([]string{"plan"}).AddRow(tt.fallbackPlan))
			}

			if got := IsEnabled(tt.tenantID, tt.featureID); got != tt.want {
				t.Fatalf("IsEnabled(%q, %q) = %t, want %t", tt.tenantID, tt.featureID, got, tt.want)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("SQL expectations were not met: %v", err)
			}
		})
	}
}

func setupPlanTestDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}

	previousDB := config.MasterDB
	config.MasterDB = db
	t.Cleanup(func() {
		config.MasterDB = previousDB
		_ = db.Close()
	})

	return mock
}

func clearPlanTestCache() {
	tenantPlanCache.Range(func(key, _ interface{}) bool {
		tenantPlanCache.Delete(key)
		return true
	})
}

func boolPointer(value bool) *bool {
	return &value
}
