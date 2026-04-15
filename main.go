package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"afrita/config"
	"afrita/handlers"
	"afrita/helpers"
	"afrita/middleware"
	"afrita/models"

	"github.com/gorilla/mux"
)

func main() {
	config.Initialize()
	config.LoadTemplates()
	helpers.LoadPersistedTokens()
	go helpers.PeriodicTokenCleanup()

	router := mux.NewRouter()

	// RBAC helper functions — wrap handlers with permission/role checks
	protect := func(resource, action string, h http.HandlerFunc) http.HandlerFunc {
		return handlers.RequirePermission(resource, action)(h).ServeHTTP
	}
	adminOnly := func(h http.HandlerFunc) http.HandlerFunc {
		return handlers.RequireRole(models.RoleAdmin)(h).ServeHTTP
	}
	managerUp := func(h http.HandlerFunc) http.HandlerFunc {
		return handlers.RequireRole(models.RoleAdmin, models.RoleManager)(h).ServeHTTP
	}

	// Static files
	staticFS := http.FileServer(http.Dir("static"))
	staticHandler := http.StripPrefix("/static/", staticFS)
	router.PathPrefix("/static/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		staticHandler.ServeHTTP(w, r)
	}))

	// Auth routes (public — no RBAC)
	router.HandleFunc("/", handlers.HandleLogin).Methods("GET")
	router.HandleFunc("/login", handlers.HandleLogin).Methods("GET")
	router.HandleFunc("/login", handlers.HandleLoginPost).Methods("POST")
	router.HandleFunc("/register", handlers.HandleRegister).Methods("GET")
	router.HandleFunc("/api/register", handlers.HandleRegisterPost).Methods("POST")
	router.HandleFunc("/forgot-password", handlers.HandleForgotPassword).Methods("GET")
	router.HandleFunc("/api/forgot-password", handlers.HandleForgotPasswordPost).Methods("POST")
	router.HandleFunc("/logout", handlers.HandleLogout).Methods("GET")
	router.HandleFunc("/api/refresh", handlers.HandleRefreshToken).Methods("POST")

	// Dashboard routes (auth only — no resource-level RBAC)
	router.HandleFunc("/dashboard", handlers.HandleDashboard).Methods("GET")
	router.HandleFunc("/dashboard/export-pdf", handlers.HandleDashboardExportPDF).Methods("GET")
	router.HandleFunc("/dashboard/compare", handlers.HandleDashboardCompare).Methods("GET")

	// Invoice routes — RBAC protected
	router.HandleFunc("/dashboard/invoices", protect("invoices", "view", handlers.HandleInvoices)).Methods("GET")
	router.HandleFunc("/dashboard/invoices/add-invoice", protect("invoices", "add", handlers.HandleAddInvoice)).Methods("GET")
	router.HandleFunc("/dashboard/invoices/create-draft", protect("invoices", "add", handlers.HandleCreateDraftInvoice)).Methods("POST")
	router.HandleFunc("/dashboard/invoices/credit/{id}", protect("invoices", "add", handlers.HandleAddCreditNote)).Methods("GET")
	router.HandleFunc("/dashboard/invoices/edit/{id}", protect("invoices", "edit", handlers.HandleEditInvoice)).Methods("GET")
	router.HandleFunc("/bill/{id}", protect("invoices", "view", handlers.HandleGetInvoice)).Methods("GET")
	router.HandleFunc("/bill/{id}/preview", protect("invoices", "view", handlers.HandleInvoicePreview)).Methods("GET")
	router.HandleFunc("/bill/{id}/print", protect("invoices", "view", handlers.HandleInvoicePrint)).Methods("GET")
	router.HandleFunc("/bill/pdf/{id}", protect("invoices", "view", handlers.HandleBillPDF)).Methods("GET")
	router.HandleFunc("/api/v2/bill/pdf/{id}", protect("invoices", "view", handlers.HandleBillPDF)).Methods("GET")
	router.HandleFunc("/credit_bill/{id}", protect("invoices", "view", handlers.HandleCreditBill)).Methods("GET")
	router.HandleFunc("/credit_bill/pdf/{id}", protect("invoices", "view", handlers.HandleCreditBillPDF)).Methods("GET")
	router.HandleFunc("/bill/credit/pdf/{id}", protect("invoices", "view", handlers.HandleCreditBillPDF)).Methods("GET")
	router.HandleFunc("/dashboard/invoices/import-csv", protect("invoices", "add", handlers.HandleImportBillsPage)).Methods("GET")
	router.HandleFunc("/api/invoices/import-csv", protect("invoices", "add", handlers.HandleImportBillsUpload)).Methods("POST")
	router.HandleFunc("/api/data-import/preview", adminOnly(handlers.HandleDataImportPreview)).Methods("POST")
	router.HandleFunc("/api/data-import/execute", adminOnly(handlers.HandleDataImportExecute)).Methods("POST")
	router.HandleFunc("/api/invoices", protect("invoices", "add", handlers.HandleCreateInvoice)).Methods("POST")
	router.HandleFunc("/api/invoices/credit", protect("invoices", "add", handlers.HandleCreateCreditNote)).Methods("POST")
	router.HandleFunc("/api/invoices/{id}", protect("invoices", "view", handlers.HandleGetInvoice)).Methods("GET")
	router.HandleFunc("/api/invoices/{id}/edit", protect("invoices", "edit", handlers.HandleEditInvoice)).Methods("GET")
	router.HandleFunc("/api/invoices/{id}", protect("invoices", "edit", handlers.HandleUpdateInvoice)).Methods("PUT")
	router.HandleFunc("/api/invoices/{id}/submit", protect("invoices", "edit", handlers.HandleSubmitDraftInvoice)).Methods("POST")
	router.HandleFunc("/api/invoices/{id}", protect("invoices", "delete", handlers.HandleDeleteInvoice)).Methods("DELETE")

	// Purchase bill routes — RBAC protected
	router.HandleFunc("/dashboard/purchase-bills", protect("purchase_bills", "view", handlers.HandlePurchaseBills)).Methods("GET")
	router.HandleFunc("/dashboard/purchase-bills/add", protect("purchase_bills", "add", handlers.HandleAddPurchaseBill)).Methods("GET")
	router.HandleFunc("/dashboard/purchase-bills/edit/{id}", protect("purchase_bills", "edit", handlers.HandleEditPurchaseBill)).Methods("GET")
	router.HandleFunc("/dashboard/purchase-bills/{id}", protect("purchase_bills", "view", handlers.HandleGetPurchaseBill)).Methods("GET")
	router.HandleFunc("/api/purchase-bills/{id}", protect("purchase_bills", "view", handlers.HandleGetPurchaseBill)).Methods("GET")
	router.HandleFunc("/api/purchase-bills/{id}", protect("purchase_bills", "edit", handlers.HandleUpdatePurchaseBill)).Methods("PUT")
	router.HandleFunc("/api/purchase-bills/{id}", protect("purchase_bills", "delete", handlers.HandleDeletePurchaseBill)).Methods("DELETE")
	router.HandleFunc("/api/purchase-bills", protect("purchase_bills", "add", handlers.HandleCreatePurchaseBill)).Methods("POST")

	// Cash voucher routes — RBAC protected
	router.HandleFunc("/dashboard/cash-vouchers", protect("invoices", "view", handlers.HandleCashVouchers)).Methods("GET")
	router.HandleFunc("/dashboard/cash-vouchers/add", protect("invoices", "add", handlers.HandleAddCashVoucher)).Methods("GET")
	router.HandleFunc("/dashboard/cash-vouchers/edit/{id}", protect("invoices", "edit", handlers.HandleEditCashVoucher)).Methods("GET")
	router.HandleFunc("/dashboard/cash-vouchers/{id}", protect("invoices", "view", handlers.HandleGetCashVoucher)).Methods("GET")
	router.HandleFunc("/api/cash-vouchers", protect("invoices", "add", handlers.HandleCreateCashVoucher)).Methods("POST")
	router.HandleFunc("/api/cash-vouchers/{id}", protect("invoices", "edit", handlers.HandleUpdateCashVoucher)).Methods("PUT")
	router.HandleFunc("/api/cash-vouchers/{id}", protect("invoices", "delete", handlers.HandleDeleteCashVoucher)).Methods("DELETE")
	router.HandleFunc("/api/cash-vouchers/{id}/approve", managerUp(handlers.HandleApproveCashVoucher)).Methods("POST")
	router.HandleFunc("/api/cash-vouchers/{id}/post", managerUp(handlers.HandlePostCashVoucher)).Methods("POST")

	// Stock routes — RBAC protected
	router.HandleFunc("/dashboard/stock/adjustments", protect("products", "view", handlers.HandleStockAdjustments)).Methods("GET")
	router.HandleFunc("/api/stock/adjust", protect("products", "edit", handlers.HandleCreateStockAdjustment)).Methods("POST")
	router.HandleFunc("/api/stock/movements/{id}", protect("products", "view", handlers.HandleProductStockMovements)).Methods("GET")
	router.HandleFunc("/api/stock/check", protect("products", "view", handlers.HandleStockCheck)).Methods("POST")
	router.HandleFunc("/api/stock/enforcement", protect("products", "view", handlers.HandleStockEnforcement)).Methods("GET")

	// Product routes — RBAC protected
	router.HandleFunc("/dashboard/products", protect("products", "view", handlers.HandleProducts)).Methods("GET")
	router.HandleFunc("/dashboard/products/add", protect("products", "add", handlers.HandleAddProduct)).Methods("GET")
	router.HandleFunc("/dashboard/products/create", protect("products", "add", handlers.HandleCreateProduct)).Methods("POST")
	router.HandleFunc("/dashboard/products/{id}", protect("products", "view", handlers.HandleProductDetail)).Methods("GET")
	router.HandleFunc("/dashboard/products/{id}/edit", protect("products", "edit", handlers.HandleEditProduct)).Methods("GET")
	router.HandleFunc("/dashboard/products/{id}/update", protect("products", "edit", handlers.HandleUpdateProduct)).Methods("POST")
	router.HandleFunc("/dashboard/products/{id}/delete", protect("products", "delete", handlers.HandleDeleteProduct)).Methods("POST")

	// Client routes — RBAC protected
	router.HandleFunc("/dashboard/clients", protect("clients", "view", handlers.HandleClients)).Methods("GET")
	router.HandleFunc("/dashboard/clients/add", protect("clients", "add", handlers.HandleAddClient)).Methods("GET")
	router.HandleFunc("/dashboard/clients/create", protect("clients", "add", handlers.HandleCreateClient)).Methods("POST")
	router.HandleFunc("/dashboard/clients/{id}", protect("clients", "view", handlers.HandleClientDetail)).Methods("GET")
	router.HandleFunc("/dashboard/clients/{id}/edit", protect("clients", "edit", handlers.HandleEditClient)).Methods("GET")
	router.HandleFunc("/dashboard/clients/{id}/update", protect("clients", "edit", handlers.HandleUpdateClient)).Methods("POST")
	router.HandleFunc("/dashboard/clients/{id}/delete", protect("clients", "delete", handlers.HandleDeleteClient)).Methods("POST")

	// Order routes — RBAC protected
	router.HandleFunc("/dashboard/orders", protect("orders", "view", handlers.HandleOrders)).Methods("GET")
	router.HandleFunc("/dashboard/orders/add", protect("orders", "add", handlers.HandleAddOrder)).Methods("GET")
	router.HandleFunc("/dashboard/orders/create", protect("orders", "add", handlers.HandleCreateOrder)).Methods("POST")
	router.HandleFunc("/dashboard/orders/{id}", protect("orders", "view", handlers.HandleOrderDetail)).Methods("GET")
	router.HandleFunc("/dashboard/orders/{id}/edit", protect("orders", "edit", handlers.HandleEditOrder)).Methods("GET")
	router.HandleFunc("/dashboard/orders/{id}/update", protect("orders", "edit", handlers.HandleUpdateOrder)).Methods("POST")
	router.HandleFunc("/dashboard/orders/{id}/delete", protect("orders", "delete", handlers.HandleDeleteOrder)).Methods("POST")

	// Branch routes — RBAC protected
	router.HandleFunc("/dashboard/branches", protect("branches", "view", handlers.HandleBranches)).Methods("GET")
	router.HandleFunc("/dashboard/branches/add", protect("branches", "add", handlers.HandleAddBranch)).Methods("GET")
	router.HandleFunc("/dashboard/branches/create", protect("branches", "add", handlers.HandleCreateBranch)).Methods("POST")
	router.HandleFunc("/dashboard/branches/{id}", protect("branches", "view", handlers.HandleBranchDetail)).Methods("GET")
	router.HandleFunc("/dashboard/branches/{id}/edit", protect("branches", "edit", handlers.HandleEditBranch)).Methods("GET")
	router.HandleFunc("/dashboard/branches/{id}/update", protect("branches", "edit", handlers.HandleUpdateBranch)).Methods("POST")
	router.HandleFunc("/dashboard/branches/{id}/delete", protect("branches", "delete", handlers.HandleDeleteBranch)).Methods("POST")

	// User routes — Admin/Manager only
	router.HandleFunc("/dashboard/users", managerUp(handlers.HandleUsers)).Methods("GET")
	router.HandleFunc("/dashboard/users/add", managerUp(handlers.HandleAddUser)).Methods("GET")
	router.HandleFunc("/dashboard/users/create", managerUp(handlers.HandleCreateUser)).Methods("POST")
	router.HandleFunc("/dashboard/users/{id}/edit", managerUp(handlers.HandleEditUser)).Methods("GET")
	router.HandleFunc("/dashboard/users/{id}/update", managerUp(handlers.HandleUpdateUser)).Methods("POST")
	router.HandleFunc("/dashboard/users/{id}/delete", adminOnly(handlers.HandleDeleteUser)).Methods("POST")
	router.HandleFunc("/dashboard/users/{id}/permissions", adminOnly(handlers.HandleUpdateUserPermissions)).Methods("POST")

	// Settings routes — Admin only
	router.HandleFunc("/dashboard/settings", adminOnly(handlers.HandleSettingsPage)).Methods("GET")
	router.HandleFunc("/dashboard/settings", adminOnly(handlers.HandleSaveSettings)).Methods("POST")

	// ZATCA API routes (JSON) — Admin only
	router.HandleFunc("/api/zatca/branch/{id}", adminOnly(handlers.HandleGetZatcaConfig)).Methods("GET")
	router.HandleFunc("/api/zatca/branch/{id}", adminOnly(handlers.HandleSaveZatcaConfig)).Methods("PUT")
	router.HandleFunc("/api/zatca/branch/{id}/onboard", adminOnly(handlers.HandleZatcaOnboard)).Methods("POST")

	// ZATCA Monitor page — Admin only
	router.HandleFunc("/dashboard/zatca-monitor", adminOnly(handlers.HandleZatcaMonitor)).Methods("GET")

	// Notification routes
	router.HandleFunc("/dashboard/notifications", handlers.HandleNotifications).Methods("GET")
	router.HandleFunc("/api/notifications/{id}/read", handlers.HandleMarkNotificationRead).Methods("POST")
	router.HandleFunc("/api/notifications/read-all", handlers.HandleMarkAllNotificationsRead).Methods("POST")
	router.HandleFunc("/api/notification-config", handlers.HandleNotificationConfig).Methods("POST")
	router.HandleFunc("/api/notification-config", handlers.HandleGetNotificationConfig).Methods("GET")

	// Store routes — RBAC protected
	router.HandleFunc("/dashboard/stores", protect("stores", "view", handlers.HandleStores)).Methods("GET")
	router.HandleFunc("/dashboard/stores/add", protect("stores", "add", handlers.HandleAddStore)).Methods("GET")
	router.HandleFunc("/dashboard/stores/create", protect("stores", "add", handlers.HandleCreateStore)).Methods("POST")
	router.HandleFunc("/dashboard/stores/{id}", protect("stores", "view", handlers.HandleStoreDetail)).Methods("GET")
	router.HandleFunc("/dashboard/stores/{id}/edit", protect("stores", "edit", handlers.HandleEditStore)).Methods("GET")
	router.HandleFunc("/dashboard/stores/{id}/update", protect("stores", "edit", handlers.HandleUpdateStore)).Methods("POST")
	router.HandleFunc("/dashboard/stores/{id}/delete", protect("stores", "delete", handlers.HandleDeleteStore)).Methods("POST")

	// Company invoice route — RBAC protected
	router.HandleFunc("/dashboard/invoices/create-company", protect("invoices", "add", handlers.HandleCreateCompanyInvoice)).Methods("POST")

	// CSV Export routes — RBAC protected
	router.HandleFunc("/dashboard/invoices/export-csv", protect("invoices", "view", handlers.HandleExportInvoicesCSV)).Methods("GET")
	router.HandleFunc("/dashboard/products/export-csv", protect("products", "view", handlers.HandleExportProductsCSV)).Methods("GET")
	router.HandleFunc("/dashboard/clients/export-csv", protect("clients", "view", handlers.HandleExportClientsCSV)).Methods("GET")
	router.HandleFunc("/dashboard/suppliers/export-csv", protect("suppliers", "view", handlers.HandleExportSuppliersCSV)).Methods("GET")

	// Search routes (auth only — no resource RBAC)
	router.HandleFunc("/dashboard/parts", handlers.HandlePartsSearch).Methods("GET")
	router.HandleFunc("/api/parts/search", handlers.HandlePartsSearchResults).Methods("POST")
	router.HandleFunc("/api/parts/search-json", handlers.HandlePartsSearchJSON).Methods("POST")
	router.HandleFunc("/api/products/search-json", handlers.HandleProductsSearchJSON).Methods("POST")
	router.HandleFunc("/dashboard/cars", handlers.HandleCarsSearch).Methods("GET")
	router.HandleFunc("/api/cars/search", handlers.HandleCarsSearchResults).Methods("GET")
	router.HandleFunc("/api/vin/verify", handlers.HandleVerifyVIN).Methods("GET")

	// Supplier routes — RBAC protected
	router.HandleFunc("/dashboard/suppliers", protect("suppliers", "view", handlers.HandleSuppliers)).Methods("GET")
	router.HandleFunc("/dashboard/suppliers/add", protect("suppliers", "add", handlers.HandleAddSupplier)).Methods("GET")
	router.HandleFunc("/dashboard/suppliers/create", protect("suppliers", "add", handlers.HandleCreateSupplier)).Methods("POST")
	router.HandleFunc("/dashboard/suppliers/{id}", protect("suppliers", "view", handlers.HandleSupplierDetail)).Methods("GET")
	router.HandleFunc("/dashboard/suppliers/{id}/edit", protect("suppliers", "edit", handlers.HandleEditSupplier)).Methods("GET")
	router.HandleFunc("/dashboard/suppliers/{id}/get", protect("suppliers", "view", handlers.HandleGetSupplier)).Methods("GET")
	router.HandleFunc("/dashboard/suppliers/{id}/update", protect("suppliers", "edit", handlers.HandleUpdateSupplier)).Methods("POST")
	router.HandleFunc("/dashboard/suppliers/{id}/delete", protect("suppliers", "delete", handlers.HandleDeleteSupplier)).Methods("POST")

	// Custom error pages
	router.NotFoundHandler = http.HandlerFunc(handlers.HandleNotFound)
	router.MethodNotAllowedHandler = http.HandlerFunc(handlers.HandleMethodNotAllowed)

	log.Printf("✅ Server listening on http://localhost:%s", config.AppPort)

	// Apply middleware chain (outermost runs first):
	// Recovery → RequestID → Logging → SecurityHeaders → RateLimit → BodySizeLimit → CSRF → TokenRefresh → Gzip
	handler := middleware.GzipMiddleware(router)
	handler = middleware.TokenRefreshMiddleware(handler)
	handler = middleware.CSRFMiddleware(handler)
	handler = middleware.BodySizeLimitMiddleware(handler)
	handler = middleware.RateLimitMiddleware(handler)
	handler = middleware.SecurityHeadersMiddleware(handler)
	handler = middleware.LoggingMiddleware(handler)
	handler = middleware.RequestIDMiddleware(handler)
	handler = middleware.RecoveryMiddleware(handler)

	server := &http.Server{
		Addr:         ":" + config.AppPort,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		log.Printf("🛑 Received signal %v, shutting down gracefully...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("❌ Graceful shutdown failed: %v", err)
		} else {
			log.Println("✅ Server stopped gracefully")
		}
	}()

	log.Fatal(server.ListenAndServe())
}
