-- 009_supplier_report_indexes.sql
-- Indexes for GET /api/v2/supplier/:id/report.
--
-- These are the lookup patterns that remove the current frontend N+1 fallback:
--   * filter purchase bills by supplier and effective date
--   * join purchase_bill_product by bill_id
--   * filter supplier cash vouchers by recipient and effective date

CREATE INDEX idx_pb_supplier_eff_report ON purchase_bill (supplier_id, effective_date, id);

-- Skip this if 001_dashboard_indexes.sql has already added idx_pbp_bill_id.
CREATE INDEX idx_pbp_bill_id_report ON purchase_bill_product (bill_id);

-- Only apply after the cash_voucher table exists.
CREATE INDEX idx_cv_supplier_eff_report ON cash_voucher (recipient_type, recipient_id, effective_date, id);
