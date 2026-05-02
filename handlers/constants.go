package handlers

// Shared HTTP header / MIME constants. Centralizing these avoids the
// SonarCloud S1192 "string literal duplicated" warnings and prevents typos.
const (
	headerContentType       = "Content-Type"
	headerContentDisp       = "Content-Disposition"
	mimeJSON                = "application/json"
	msgSupplierNotFound     = "المورد غير موجود"
	msgSupplierReportFailed = "تعذر تحميل تقرير المورد"
)
