package handlers

import (
	"sort"
	"sync"
	"time"

	"afrita/helpers"
	"afrita/models"
)

// ============================================================================
// In-Memory Mock Notification Store
// ============================================================================
// Backend does NOT have notification endpoints yet (B.7 in DATABASE_AND_API.md).
// This store holds realistic Arabic notifications so the notification bell and
// list page work end-to-end with in-memory persistence.
// When the backend adds /api/v2/notifications, swap for real API calls.
// Documented in api_docs/missing_apis.md.

// MockNotificationStore is a thread-safe in-memory notification store.
type MockNotificationStore struct {
	mu            sync.RWMutex
	notifications map[int]*models.Notification
	nextID        int
}

// NotificationStore is the global singleton for all notification handlers.
var NotificationStore *MockNotificationStore

// MockNotifConfigStore holds per-user notification preferences.
type MockNotifConfigStore struct {
	mu      sync.RWMutex
	configs map[int]*models.NotificationConfig // keyed by user_id
}

// NotifConfigStore is the global singleton for notification config.
var NotifConfigStore *MockNotifConfigStore

func init() {
	NotificationStore = NewMockNotificationStore()
	NotifConfigStore = NewMockNotifConfigStore()

	// Wire the notification count function so helpers.Render can inject it
	// into every template without a circular import.
	// Prefers cached backend data; falls back to mock store count.
	helpers.NotificationCountFunc = func() int {
		if cached, found := helpers.APICache.Get("notifications"); found {
			notifs := cached.([]models.Notification)
			count := 0
			for _, n := range notifs {
				if !n.IsRead {
					count++
				}
			}
			return count
		}
		return NotificationStore.UnreadCount()
	}
}

// NewMockNotificationStore creates a store seeded with sample Arabic notifications.
func NewMockNotificationStore() *MockNotificationStore {
	now := time.Now()

	store := &MockNotificationStore{
		notifications: make(map[int]*models.Notification),
		nextID:        9,
	}

	seed := []*models.Notification{
		{
			ID: 1, UserID: 1, Type: "low_stock",
			Title:   "تنبيه مخزون منخفض",
			Message: "المنتج 'فلتر زيت تويوتا' وصل إلى 3 وحدات فقط",
			IsRead:  false, Resource: "product", ResourceID: "12",
			CreatedAt: now.Add(-1 * time.Hour),
		},
		{
			ID: 2, UserID: 1, Type: "payment_due",
			Title:   "فاتورة مستحقة الدفع",
			Message: "الفاتورة INV-2024-0156 مستحقة خلال 3 أيام — العميل: محمد الأحمد",
			IsRead:  false, Resource: "invoice", ResourceID: "156",
			CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			ID: 3, UserID: 1, Type: "new_order",
			Title:   "طلب جديد",
			Message: "تم استلام طلب جديد ORD-2024-0089 من فرع جدة",
			IsRead:  false, Resource: "order", ResourceID: "89",
			CreatedAt: now.Add(-5 * time.Hour),
		},
		{
			ID: 4, UserID: 1, Type: "system",
			Title:   "تحديث النظام",
			Message: "تم تحديث النظام إلى الإصدار 0.5.0 بنجاح — تحسينات الأداء والتخزين المؤقت",
			IsRead:  true, Resource: "", ResourceID: "",
			CreatedAt: now.Add(-24 * time.Hour),
		},
		{
			ID: 5, UserID: 1, Type: "low_stock",
			Title:   "تنبيه مخزون منخفض",
			Message: "المنتج 'بطارية بوش 70 أمبير' وصل إلى وحدة واحدة فقط",
			IsRead:  false, Resource: "product", ResourceID: "25",
			CreatedAt: now.Add(-6 * time.Hour),
		},
		{
			ID: 6, UserID: 1, Type: "info",
			Title:   "مرحباً بك في عفريته",
			Message: "شكراً لاستخدامك نظام عفريته لإدارة قطع الغيار. يمكنك البدء بإضافة الفواتير والمنتجات.",
			IsRead:  true, Resource: "", ResourceID: "",
			CreatedAt: now.Add(-48 * time.Hour),
		},
		{
			ID: 7, UserID: 1, Type: "payment_due",
			Title:   "دفعة متأخرة",
			Message: "الفاتورة INV-2024-0143 متأخرة 5 أيام — العميل: سالم الدوسري — المبلغ: 12,500 ر.س",
			IsRead:  false, Resource: "invoice", ResourceID: "143",
			CreatedAt: now.Add(-3 * time.Hour),
		},
		{
			ID: 8, UserID: 1, Type: "new_order",
			Title:   "طلب جديد من عميل VIP",
			Message: "تم استلام طلب كبير ORD-2024-0092 بقيمة 45,000 ر.س من الشركة السعودية للسيارات",
			IsRead:  false, Resource: "order", ResourceID: "92",
			CreatedAt: now.Add(-8 * time.Hour),
		},
	}

	for _, n := range seed {
		store.notifications[n.ID] = n
	}

	return store
}

// List returns all notifications sorted by created_at desc.
func (s *MockNotificationStore) List() []models.Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]models.Notification, 0, len(s.notifications))
	for _, n := range s.notifications {
		list = append(list, *n)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list
}

// UnreadCount returns the number of unread notifications.
func (s *MockNotificationStore) UnreadCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, n := range s.notifications {
		if !n.IsRead {
			count++
		}
	}
	return count
}

// GetByID returns a copy of the notification or nil.
func (s *MockNotificationStore) GetByID(id int) *models.Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n, ok := s.notifications[id]; ok {
		copy := *n
		return &copy
	}
	return nil
}

// MarkRead marks a single notification as read. Returns false if not found.
func (s *MockNotificationStore) MarkRead(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.notifications[id]; ok {
		n.IsRead = true
		return true
	}
	return false
}

// MarkAllRead marks all notifications as read. Returns the count marked.
func (s *MockNotificationStore) MarkAllRead() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, n := range s.notifications {
		if !n.IsRead {
			n.IsRead = true
			count++
		}
	}
	return count
}

// Create adds a new notification and returns its ID.
func (s *MockNotificationStore) Create(n *models.Notification) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n.ID = s.nextID
	s.nextID++
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	stored := *n
	s.notifications[n.ID] = &stored
	return n.ID
}

// ============================================================================
// Notification Config Store
// ============================================================================

// NewMockNotifConfigStore creates a config store with sensible defaults.
func NewMockNotifConfigStore() *MockNotifConfigStore {
	return &MockNotifConfigStore{
		configs: map[int]*models.NotificationConfig{
			1: { // Default config for user 1 (admin)
				UserID:             1,
				LowStockAlert:      true,
				LowStockThreshold:  5,
				PendingInvoiceDays: 7,
				NewOrderAlert:      true,
				PaymentDueAlert:    true,
				DailySummary:       false,
				EmailEnabled:       false,
			},
		},
	}
}

// Get returns the notification config for a user, or defaults if not set.
func (s *MockNotifConfigStore) Get(userID int) models.NotificationConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if cfg, ok := s.configs[userID]; ok {
		return *cfg
	}
	// Return sensible defaults
	return models.NotificationConfig{
		UserID:             userID,
		LowStockAlert:      true,
		LowStockThreshold:  5,
		PendingInvoiceDays: 7,
		NewOrderAlert:      true,
		PaymentDueAlert:    true,
		DailySummary:       false,
		EmailEnabled:       false,
	}
}

// Save persists notification config for a user.
func (s *MockNotifConfigStore) Save(cfg models.NotificationConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := cfg
	s.configs[cfg.UserID] = &stored
}
