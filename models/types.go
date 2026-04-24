package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// ============================================================================
// Authentication & Authorization
// ============================================================================

// Token represents stored token data with metadata
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// AuthResponse represents the login response from backend
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error,omitempty"`
}

// AuthRequest represents the login request
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Role types for user access control
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleManager  Role = "manager"
	RoleEmployee Role = "employee"
)

// User represents a system user with RBAC
type User struct {
	ID          int          `json:"id"`
	Username    string       `json:"username"`
	Email       string       `json:"email"`
	Role        Role         `json:"role"`
	ManagerID   *int         `json:"manager_id,omitempty"`
	Permissions []Permission `json:"permissions"`
	Active      bool         `json:"active"`
	CreatedAt   time.Time    `json:"created_at"`
	LastLogin   *time.Time   `json:"last_login,omitempty"`
}

// Permission defines access control for a resource
type Permission struct {
	Resource  string `json:"resource"`
	CanView   bool   `json:"can_view"`
	CanAdd    bool   `json:"can_add"`
	CanEdit   bool   `json:"can_edit"`
	CanDelete bool   `json:"can_delete"`
}

// ============================================================================
// Core Business Entities
// ============================================================================

// CashVoucher represents a cash disbursement or receipt voucher
type CashVoucher struct {
	ID                   int     `json:"id"`
	VoucherNumber        int     `json:"voucher_number"`
	VoucherType          string  `json:"voucher_type"`
	EffectiveDate        string  `json:"effective_date"`
	Amount               float64 `json:"amount"`
	PaymentMethod        string  `json:"payment_method"`
	State                int     `json:"state"`
	RecipientType        string  `json:"recipient_type"`
	RecipientID          int     `json:"recipient_id"`
	RecipientName        string  `json:"recipient_name"`
	ReferenceType        string  `json:"reference_type"`
	ReferenceID          int     `json:"reference_id"`
	Description          string  `json:"description"`
	Note                 string  `json:"note"`
	BankName             string  `json:"bank_name"`
	BankAccount          string  `json:"bank_account"`
	TransactionReference string  `json:"transaction_reference"`
	StoreID              int     `json:"store_id"`
}

// CashVoucherPayload represents the payload for creating/updating cash vouchers
type CashVoucherPayload struct {
	VoucherType          string `json:"voucher_type"`
	EffectiveDate        string `json:"effective_date"`
	Amount               string `json:"amount"`
	PaymentMethod        string `json:"payment_method"`
	RecipientType        string  `json:"recipient_type"`
	RecipientID          int     `json:"recipient_id,omitempty"`
	RecipientName        string  `json:"recipient_name"`
	ReferenceType        string  `json:"reference_type,omitempty"`
	ReferenceID          int     `json:"reference_id,omitempty"`
	Description          string  `json:"description,omitempty"`
	Note                 string  `json:"note,omitempty"`
	BankName             string  `json:"bank_name,omitempty"`
	BankAccount          string  `json:"bank_account,omitempty"`
	TransactionReference string  `json:"transaction_reference,omitempty"`
	StoreID              int     `json:"store_id"`
	BranchID             int     `json:"branch_id"`
}

// Invoice represents the invoice/bill structure from the API
type Invoice struct {
	ID                     int `json:"id"`
	SequenceNumber         int `json:"sequence_number"`
	SupplierSequenceNumber int `json:"supplier_sequence_number"`
	EffectiveDate          struct {
		Time  string `json:"Time"`
		Valid bool   `json:"Valid"`
	} `json:"effective_date"`
	PaymentDueDate interface{} `json:"payment_due_date"`
	State          int         `json:"state"`
	Subtotal       float64     `json:"subtotal"`
	Total          float64     `json:"total"`
	TotalVAT       float64     `json:"total_vat"`
	TotalBeforeVAT float64     `json:"total_before_vat"`
	Discount       float64     `json:"discount"`
	VAT            float64     `json:"vat"`
	Type           bool        `json:"type"`
	CreditState    int         `json:"credit_state"`
}

// InvoiceDisplay represents formatted invoice data for display
type InvoiceDisplay struct {
	Order          int     `json:"order"`
	ID             int     `json:"id"`
	SequenceNumber int     `json:"sequence_number"`
	Date           string  `json:"date"`
	DueDate        string  `json:"due_date"`
	Total          float64 `json:"total"`
	State          int     `json:"state"`
	StateLabel     string  `json:"state_label"`
	Type           bool    `json:"type"`
	TypeLabel      string  `json:"type_label"`
	CreditState    int     `json:"credit_state"`
}

// Product represents a product/part
type Product struct {
	ID          int    `json:"id"`
	PartName    string `json:"part_name"`
	Name        string `json:"name"`
	Price       string `json:"price"`
	Quantity    string `json:"quantity"`
	PartID      int    `json:"article_id"`
	StoreID     int    `json:"store_id"`
	CostPrice   string `json:"cost_price"`
	ShelfNumber string `json:"shelf_number"`
	MinStock    string `json:"min_stock"`
}

// ProductPayload represents the payload for creating/updating products
type ProductPayload struct {
	PartID  int     `json:"part_id"`
	Price   float64 `json:"price"`
	StoreID int     `json:"store_id"`
}

// Client represents a client/customer
type Client struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	CompanyName            string `json:"company_name"`
	Email                  string `json:"email"`
	Phone                  string `json:"phone"`
	Address                string `json:"address"`
	ShortAddress           string `json:"short_address"`
	Number                 string `json:"number"`
	VATNumber              string `json:"vat_number"`
	CR                     string `json:"commercial_registration"`
	TaxNumber              string `json:"tax_number"`
	BankAccount            string `json:"bank_account"`
	PreferredPaymentMethod int    `json:"preferred_payment_method"`
	IsPostPaid             bool   `json:"is_post_paid"`
	PaymentTermsDays       int    `json:"payment_terms_days"`
	CreditLimit            int    `json:"credit_limit"`
	CreatedAt              string `json:"created_at"`
	UpdatedAt              string `json:"updated_at"`
}

// parseNullableString decodes a JSON value that may be a plain string,
// a sql.NullString object {"String":"...","Valid":true/false}, or null.
func parseNullableString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// Try plain string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try NullString format
	var ns struct {
		String string `json:"String"`
		Valid  bool   `json:"Valid"`
	}
	if err := json.Unmarshal(raw, &ns); err == nil && ns.Valid {
		return ns.String
	}
	return ""
}

// UnmarshalJSON handles both plain JSON fields and sql.NullString format
// that the backend returns for nullable columns.
func (c *Client) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID                     interface{}     `json:"id"`
		Name                   json.RawMessage `json:"name"`
		CompanyName            json.RawMessage `json:"company_name"`
		Email                  json.RawMessage `json:"email"`
		Phone                  json.RawMessage `json:"phone"`
		Address                json.RawMessage `json:"address"`
		ShortAddress           json.RawMessage `json:"short_address"`
		VATNumber              json.RawMessage `json:"vat_number"`
		Number                 json.RawMessage `json:"number"`
		CR                     json.RawMessage `json:"commercial_registration"`
		BankAccount            json.RawMessage `json:"bank_account"`
		PreferredPaymentMethod interface{}     `json:"preferred_payment_method"`
		IsPostPaid             interface{}     `json:"is_post_paid"`
		PaymentTermsDays       interface{}     `json:"payment_terms_days"`
		CreditLimit            interface{}     `json:"credit_limit"`
		CreatedAt              json.RawMessage `json:"created_at"`
		UpdatedAt              json.RawMessage `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	// Handle ID which can be int or string
	switch v := raw.ID.(type) {
	case float64:
		c.ID = fmt.Sprintf("%d", int(v))
	case string:
		c.ID = v
	default:
		if raw.ID != nil {
			c.ID = fmt.Sprintf("%v", raw.ID)
		}
	}
	c.Name = parseNullableString(raw.Name)
	c.CompanyName = parseNullableString(raw.CompanyName)
	c.Email = parseNullableString(raw.Email)
	c.Phone = parseNullableString(raw.Phone)
	c.Address = parseNullableString(raw.Address)
	c.ShortAddress = parseNullableString(raw.ShortAddress)
	c.VATNumber = parseNullableString(raw.VATNumber)
	c.TaxNumber = c.VATNumber // keep backward compat
	c.Number = parseNullableString(raw.Number)
	c.CR = parseNullableString(raw.CR)
	c.BankAccount = parseNullableString(raw.BankAccount)
	c.CreatedAt = parseNullableString(raw.CreatedAt)
	c.UpdatedAt = parseNullableString(raw.UpdatedAt)
	// Numeric fields
	if v, ok := raw.PreferredPaymentMethod.(float64); ok {
		c.PreferredPaymentMethod = int(v)
	}
	if v, ok := raw.IsPostPaid.(bool); ok {
		c.IsPostPaid = v
	}
	if v, ok := raw.PaymentTermsDays.(float64); ok {
		c.PaymentTermsDays = int(v)
	}
	if v, ok := raw.CreditLimit.(float64); ok {
		c.CreditLimit = int(v)
	}
	return nil
}

// Supplier represents a supplier/vendor
type Supplier struct {
	ID                     int    `json:"id"`
	Name                   string `json:"name"`
	Email                  string `json:"email"`
	Address                string `json:"address"`
	ShortAddress           string `json:"short_address"`
	PhoneNumber            string `json:"phone_number"`
	Number                 string `json:"number"`
	VATNumber              string `json:"vat_number"`
	CR                     string `json:"commercial_registration"`
	BankAccount            string `json:"bank_account"`
	PreferredPaymentMethod int    `json:"preferred_payment_method"`
	IsPostPaid             bool   `json:"is_post_paid"`
	PaymentTermsDays       int    `json:"payment_terms_days"`
	CreditLimit            int    `json:"credit_limit"`
	CreatedAt              string `json:"created_at"`
	UpdatedAt              string `json:"updated_at"`
}

// Store represents a store/branch location
type Store struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Branch represents a business branch
type Branch struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Phone       string `json:"phone"`
	ManagerName string `json:"manager_name"`
	ManagerID   *int   `json:"manager_id,omitempty"`
	IsActive    bool   `json:"is_active"`
	StoreCount  int    `json:"store_count"`
	Stores      []Store `json:"stores,omitempty"`
}

// Order represents a customer order
type Order struct {
	ID             int       `json:"id"`
	CustomerName   string    `json:"customer_name"`
	Total          float64   `json:"total"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	SequenceNumber string    `json:"sequence_number"`
}

// ============================================================================
// Stock Management
// ============================================================================

const (
	StockEnforcementDisable = "disable"
	StockEnforcementWarn    = "warn"
	StockEnforcementEnforce = "enforce"
)

// StockAdjustRequest is the payload for manual stock adjustments
type StockAdjustRequest struct {
	ProductID      int    `json:"product_id"`
	StoreID        int    `json:"store_id"`
	QuantityChange int    `json:"quantity_change"`
	Reason         string `json:"reason"`
	Note           string `json:"note"`
}

// StockMovement represents a stock movement history entry
type StockMovement struct {
	ID                int     `json:"id"`
	ProductID         int     `json:"product_id"`
	StoreID           int     `json:"store_id"`
	Quantity          string  `json:"quantity"`
	MovementType      string  `json:"movement_type"`
	MovementTypeLabel string  `json:"-"`
	ReferenceType     string  `json:"reference_type"`
	ReferenceID       *int    `json:"reference_id"`
	Reason            *string `json:"reason"`
	Note              string  `json:"note"`
	CreatedAt         string  `json:"created_at"`
}

// StockCheckItem is a single item in a stock check request
type StockCheckItem struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

// StockCheckResponse is the response from a stock availability check
type StockCheckResponse struct {
	Enforcement   string `json:"enforcement"`
	AllSufficient bool   `json:"all_sufficient"`
}

// StockEnforcementResponse holds the current enforcement mode
type StockEnforcementResponse struct {
	Mode string `json:"mode"`
}

// ============================================================================
// Notifications
// ============================================================================

// Notification represents a single notification
type Notification struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Message    string    `json:"message"`
	IsRead     bool      `json:"is_read"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// NotificationConfig holds per-user notification preferences
type NotificationConfig struct {
	UserID             int  `json:"user_id"`
	LowStockAlert      bool `json:"low_stock_alert"`
	LowStockThreshold  int  `json:"low_stock_threshold"`
	PendingInvoiceDays int  `json:"pending_invoice_days"`
	NewOrderAlert      bool `json:"new_order_alert"`
	PaymentDueAlert    bool `json:"payment_due_alert"`
	DailySummary       bool `json:"daily_summary"`
	EmailEnabled       bool `json:"email_enabled"`
}

// ============================================================================
// Bills & Transactions
// ============================================================================

// BillItem represents a line item in a bill (used for reading/display)
type BillItem struct {
	ProductID      int     `json:"product_id,omitempty"`
	PartName       string  `json:"name"`
	PartNumber     string  `json:"part_name"`
	Price          float64 `json:"price"`
	Quantity       int     `json:"quantity"`
	Discount       float64 `json:"discount"`
	TotalBeforeVAT float64 `json:"total_before_vat"`
}

// BillProductItem represents a linked product in a bill payload.
// Backend requires "product_id" (number), "name" (string), "price" (string), "quantity" (string).
type BillProductItem struct {
	ID        int    `json:"product_id"`
	PartName  string `json:"name,omitempty"`
	Price     string `json:"price"`
	Quantity  string `json:"quantity"`
	CostPrice string `json:"cost_price,omitempty"`
	Discount  string `json:"discount,omitempty"`
}

// BillManualItem represents a manual (text-based) product in a bill payload.
// Backend requires "product_id": null, "name" (string), "price" (string), "quantity" (string).
type BillManualItem struct {
	ProductID  *int   `json:"product_id"`
	PartName   string `json:"name"`
	PartNumber string `json:"part_name,omitempty"`
	Price      string `json:"price"`
	Quantity   string `json:"quantity"`
	CostPrice  string `json:"cost_price,omitempty"`
	Discount   string `json:"discount,omitempty"`
}

// BillPayload represents the payload for creating/updating bills.
// CRITICAL: Backend requires discount, maintenance_cost, and product prices as STRINGS.
// Products use BillProductItem (with "product_id"), ManualProducts use BillManualItem (with "product_id": null).
type BillPayload struct {
	StoreID         int               `json:"store_id"`
	Products        []BillProductItem `json:"products"`
	ManualProducts  []BillManualItem  `json:"manual_products"`
	TotalAmount     float64           `json:"total_amount,omitempty"`
	Discount        string            `json:"discount"`
	MaintenanceCost string            `json:"maintenance_cost"`
	State           int               `json:"state"`
	VIN             string            `json:"vin,omitempty"`
	UserName        string            `json:"user_name"`
	UserPhoneNumber string            `json:"user_phone_number,omitempty"`
	Note            string            `json:"note,omitempty"`
	EffectiveDate   *string           `json:"effective_date,omitempty"`
	PaymentDueDate  *string           `json:"payment_due_date,omitempty"`
	DeliverDate     *string           `json:"deliver_date,omitempty"`
	PaymentMethod   int               `json:"payment_method"`
	ClientID        *int              `json:"client_id,omitempty"`
	BranchID        int               `json:"branch_id"`
}

// BillDetail represents detailed bill information
type BillDetail struct {
	ID              int        `json:"id"`
	SequenceNumber  int        `json:"sequence_number"`
	StoreID         int        `json:"store_id"`
	BranchID        int        `json:"branch_id"`
	Products        []BillItem `json:"products"`
	ManualProducts  []BillItem `json:"manual_products"`
	TotalAmount     float64    `json:"total"`
	Discount        float64    `json:"discount"`
	MaintenanceCost float64    `json:"maintenance_cost"`
	State           int        `json:"state"`
	UserName        string     `json:"user_name"`
	UserPhoneNumber string     `json:"user_phone_number"`
	Note            string     `json:"note"`
}

// InvoiceFull combines Invoice fields with product details for print/preview pages.
type InvoiceFull struct {
	Invoice
	Products        []BillItem `json:"products"`
	ManualProducts  []BillItem `json:"manual_products"`
	UserName        string     `json:"user_name"`
	UserPhoneNumber string     `json:"user_phone_number"`
	Note            string     `json:"note"`
}

// PurchaseBill represents a purchase bill from a supplier
type PurchaseBill struct {
	ID                     int       `json:"id"`
	SequenceNumber         int       `json:"sequence_number"`
	SupplierID             int       `json:"supplier_id"`
	SupplierName           string    `json:"supplier_name"`
	SupplierSequenceNumber string    `json:"supplier_sequence_number"`
	Total                  float64   `json:"total"`
	Discount               float64   `json:"discount"`
	PaymentDate            time.Time `json:"payment_date"`
	CreatedAt              time.Time `json:"created_at"`
}

// PurchaseBillDisplay represents formatted purchase bill for display
type PurchaseBillDisplay struct {
	Order                  int     `json:"order"`
	ID                     int     `json:"id"`
	SequenceNumber         int     `json:"sequence_number"`
	SupplierName           string  `json:"supplier_name"`
	SupplierSequenceNumber string  `json:"supplier_sequence_number"`
	Total                  float64 `json:"total"`
	PaymentDate            string  `json:"payment_date"`
	Date                   string  `json:"date"`
}

// PurchaseBillPayload represents the payload for purchase bills.
// JSON tags use the backend API field names (from GET response).
type PurchaseBillPayload struct {
	StoreID                int               `json:"store_id"`
	MerchantID             int               `json:"merchant_id"`
	SupplierID             int               `json:"supplier_id"`
	SupplierSequenceNumber uint64            `json:"supplier_sequence_number"`
	State                  int               `json:"state"`
	EffectiveDate          string            `json:"effective_date"`
	Products               []BillProductItem `json:"products"`
	ManualProducts         []BillManualItem  `json:"manual_products"`
	Discount               string            `json:"discount"`
	Subtotal               float64           `json:"subtotal"`
	PaymentDueDate         *string           `json:"payment_due_date"`
	DeliverDate            *string           `json:"deliver_date"`
	PaymentMethod          int               `json:"payment_method"`
	PaidAmount             string            `json:"paid_amount"`
	PDFLink                *string           `json:"pdf_link"`
	Attachments            []string          `json:"attachments"`
}

// CreditNote represents a credit note for invoice adjustments
type CreditNote struct {
	ID             int       `json:"id"`
	InvoiceID      int       `json:"invoice_id"`
	SequenceNumber int       `json:"sequence_number"`
	Amount         float64   `json:"amount"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}

// ============================================================================
// Supplier Report
// ============================================================================

// SupplierBillSummary holds aggregated stats for a supplier's bills
type SupplierBillSummary struct {
	BillCount        int     `json:"bill_count"`
	TotalSpent       float64 `json:"total_spent"`
	TotalBeforeVAT   float64 `json:"total_before_vat"`
	TotalVAT         float64 `json:"total_vat"`
	UnpaidTotal      float64 `json:"unpaid_total"`
	PaidTotal        float64 `json:"paid_total"`
	ReceivedCount    int     `json:"received_count"`
	AvgBill          float64 `json:"avg_bill"`
	TotalDiscount    float64 `json:"total_discount"`
	CreditUtilPct    float64 `json:"credit_utilization"`
	TotalPayments    float64 `json:"total_payments"`    // sum of cash voucher payments
	PaymentCount     int     `json:"payment_count"`     // number of payments
	ClosingBalance   float64 `json:"closing_balance"`   // unpaid after payments
	OpeningBalance   float64 `json:"opening_balance"`   // balance before date range
	OverdueAmount    float64 `json:"overdue_amount"`    // bills past due date
	OverdueCount     int     `json:"overdue_count"`     // number of overdue bills
	AvgPaymentDays   float64 `json:"avg_payment_days"`  // average days to pay
}

// SupplierReportBill represents a single bill row in the supplier report
type SupplierReportBill struct {
	ID             int     `json:"id"`
	SequenceNumber int     `json:"sequence_number"`
	SSN            string  `json:"supplier_sequence_number"`
	Total          float64 `json:"total"`
	TotalBeforeVAT float64 `json:"total_before_vat"`
	TotalVAT       float64 `json:"total_vat"`
	Discount       float64 `json:"discount"`
	State          int     `json:"state"`
	EffectiveDate  string  `json:"effective_date"`
	PaymentDueDate string  `json:"payment_due_date"`
	DeliverDate    string  `json:"deliver_date"`
	PaymentMethod  int     `json:"payment_method"`
	ReceivedAt     string  `json:"received_at"`
	ReceivedBy     string  `json:"received_by"`
	ItemCount      int     `json:"item_count"`
	IsOverdue      bool    `json:"is_overdue"`
	DaysOverdue    int     `json:"days_overdue"`
}

// SupplierTopItem represents a top-purchased item from a supplier
type SupplierTopItem struct {
	Name      string  `json:"name"`
	TotalQty  int     `json:"total_qty"`
	TotalVal  float64 `json:"total_value"`
	AvgPrice  float64 `json:"avg_price"`
	BillCount int     `json:"bill_count"`
}

// LedgerEntry represents a single line in the account statement (كشف حساب)
type LedgerEntry struct {
	Date        string  `json:"date"`
	Type        string  `json:"type"`         // "bill", "payment", "opening"
	Reference   string  `json:"reference"`
	Description string  `json:"description"`
	Debit       float64 `json:"debit"`        // bills increase debt
	Credit      float64 `json:"credit"`       // payments reduce debt
	Balance     float64 `json:"balance"`      // running balance
	LinkURL     string  `json:"link_url"`
}

// AgingBucket represents an aging category for overdue analysis
type AgingBucket struct {
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

// PaymentMethodBreakdown shows spending by payment method
type PaymentMethodBreakdown struct {
	Method string  `json:"method"`
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

// MonthlySpend shows a single month's spending
type MonthlySpend struct {
	Month    string  `json:"month"`
	Amount   float64 `json:"amount"`
	Payments float64 `json:"payments"`
}

// SupplierReportPageData is the template data for the supplier report page
type SupplierReportPageData struct {
	PageData
	Supplier              Supplier                 `json:"supplier"`
	Summary               SupplierBillSummary      `json:"summary"`
	Bills                 []SupplierReportBill     `json:"bills"`
	TopItems              []SupplierTopItem        `json:"top_items"`
	Ledger                []LedgerEntry            `json:"ledger"`
	Aging                 []AgingBucket            `json:"aging"`
	PaymentMethods        []PaymentMethodBreakdown `json:"payment_methods"`
	MonthlySpending       []MonthlySpend           `json:"monthly_spending"`
	DateFrom              string                   `json:"date_from"`
	DateTo                string                   `json:"date_to"`
}

// ============================================================================
// Dashboard & Analytics
// ============================================================================

// DashboardStats represents key dashboard statistics
type DashboardStats struct {
	Invoices      int    `json:"invoices"`
	Products      int    `json:"products"`
	Clients       int    `json:"clients"`
	Revenue       string `json:"revenue"`
	Suppliers     int    `json:"suppliers"`
	PendingOrders string `json:"pending_orders"`
	PendingAmount string `json:"pending_amount"`
}

// DashboardPerformance represents performance metrics
type DashboardPerformance struct {
	CompletionRate     string `json:"completion_rate"`
	ActiveClients      string `json:"active_clients"`
	InventoryAvailable string `json:"inventory_available"`
	ForecastAccuracy   string `json:"forecast_accuracy"`
}

// RecentInvoice represents a recent invoice for dashboard display
type RecentInvoice struct {
	ID             int     `json:"id"`
	SequenceNumber string  `json:"sequence_number"`
	ClientName     string  `json:"client_name"`
	Total          string  `json:"total"`
	Date           string  `json:"date"`
	Status         string  `json:"status"`
	StatusClass    string  `json:"status_class"`
	TotalFormatted float64 `json:"total_formatted,omitempty"`
}

// TopClient represents a top client for dashboard display
type TopClient struct {
	Order int    `json:"order"`
	Name  string `json:"name"`
	Count int    `json:"count"`
	Total string `json:"total"`
	Last  string `json:"last"`
}

// StatusCounts represents invoice status counts for dashboard
type StatusCounts struct {
	Draft      int `json:"draft"`
	Processing int `json:"processing"`
	Processed  int `json:"processed"`
	Issued     int `json:"issued"`
}

// ============================================================================
// Page Data Structures (for template rendering)
// ============================================================================

// PageData represents common data for all pages
type PageData struct {
	Title   string `json:"title"`
	Version string `json:"version,omitempty"`
}

// InvoicesPageData represents data for invoices page
type InvoicesPageData struct {
	PageData
	Invoices   []InvoiceDisplay `json:"invoices"`
	Pagination interface{}      `json:"pagination"`
	PrevPage   int              `json:"prev_page"`
	NextPage   int              `json:"next_page"`
	Query      string           `json:"query"`
	State      string           `json:"state"`
	Clients    []Client         `json:"clients"`
	Stores     []Store          `json:"stores"`
}

// SuppliersPageData represents data for suppliers page
type SuppliersPageData struct {
	PageData
	Suppliers  []Supplier  `json:"suppliers"`
	Pagination interface{} `json:"pagination"`
	PrevPage   int         `json:"prev_page"`
	NextPage   int         `json:"next_page"`
	Query      string      `json:"query"`
}

// PurchaseBillsPageData represents data for purchase bills page
type PurchaseBillsPageData struct {
	PageData
	Bills      []PurchaseBillDisplay `json:"bills"`
	Pagination interface{}           `json:"pagination"`
	PrevPage   int                   `json:"prev_page"`
	NextPage   int                   `json:"next_page"`
	Query      string                `json:"query"`
}

// DashboardPageData represents data for dashboard page
type DashboardPageData struct {
	PageData
	Stats             DashboardStats       `json:"stats"`
	Performance       DashboardPerformance `json:"performance"`
	StatusCounts      StatusCounts         `json:"status_counts"`
	RecentInvoices    []RecentInvoice      `json:"recent_invoices"`
	TopClients        []TopClient          `json:"top_clients"`
	LabelsJSON        string               `json:"labels_json"`
	RevenueJSON       string               `json:"revenue_json"`
	StatusJSON        string               `json:"status_json"`
	TopProductsLabels string               `json:"top_products_labels"`
	TopProductsData   string               `json:"top_products_data"`
	ClientsLabelsJSON string               `json:"clients_labels_json"`
	ClientsDataJSON   string               `json:"clients_data_json"`
}

// ============================================================================
// Search & Filter
// ============================================================================

// SearchResult represents a generic search result
type SearchResult struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// CarSearchResult represents a car/VIN search result
type CarSearchResult struct {
	VIN          string `json:"vin"`
	Make         string `json:"make"`
	Model        string `json:"model"`
	Year         int    `json:"year"`
	Engine       string `json:"engine"`
	Transmission string `json:"transmission"`
}

// PartSearchResult represents a parts search result
type PartSearchResult struct {
	PartID      string  `json:"part_id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Available   bool    `json:"available"`
	Quantity    int     `json:"quantity"`
	Description string  `json:"description"`
}

// ============================================================================
// API Response Wrappers
// ============================================================================

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination struct {
		Page       int `json:"page"`
		PerPage    int `json:"per_page"`
		Total      int `json:"total"`
		TotalPages int `json:"total_pages"`
	} `json:"pagination"`
}
