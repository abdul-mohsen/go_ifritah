-- queries/supplier_report.sql
-- Drop into pkg/db/queries/supplier_report.sql in the ifritah-go backend.
--
-- Goal: replace the frontend legacy `1 + N + 1` report fallback with one
-- aggregate endpoint. These queries filter by supplier_id and date inside the
-- database, then return every section required by the report page/downloads.
--
-- Verified table facts from the existing backend suggestions:
--   * purchase_bill has supplier_id, supplier_sequence_number, state,
--     effective_date, payment_due_date, discount.
--   * purchase_bill_product uses bill_id as the FK to purchase_bill.id and has
--     generated totals: total_before_vat, vat_total, total_including_vat.
--   * cash_voucher is the planned/current voucher API table shape used by the
--     frontend for supplier disbursements.
--
-- Param names are shared across queries for sqlc-generated params:
--   supplier_id, from_date, to_date

-- name: GetSupplierReportSummary :one
WITH bill_lines AS (
    SELECT
        pbp.bill_id,
        COUNT(*) AS item_count,
        ROUND(COALESCE(SUM(pbp.total_before_vat), 0), 2) AS total_before_vat,
        ROUND(COALESCE(SUM(pbp.vat_total), 0), 2) AS total_vat,
        ROUND(COALESCE(SUM(pbp.total_including_vat), 0), 2) AS total
    FROM purchase_bill_product pbp
    GROUP BY pbp.bill_id
), filtered_bills AS (
    SELECT
        pb.id,
        COALESCE(bl.total, 0) AS total,
        COALESCE(bl.total_before_vat, 0) AS total_before_vat,
        COALESCE(bl.total_vat, 0) AS total_vat,
        COALESCE(pb.discount, 0) AS discount,
        pb.state,
        pb.effective_date,
        pb.payment_due_date,
        pb.received_at
    FROM purchase_bill pb
    LEFT JOIN bill_lines bl ON bl.bill_id = pb.id
    WHERE pb.supplier_id = sqlc.arg('supplier_id')
      AND (sqlc.narg('from_date') IS NULL OR DATE(pb.effective_date) >= DATE(sqlc.narg('from_date')))
      AND (sqlc.narg('to_date') IS NULL OR DATE(pb.effective_date) <= DATE(sqlc.narg('to_date')))
), filtered_payments AS (
    SELECT cv.id, COALESCE(cv.amount, 0) AS amount
    FROM cash_voucher cv
    WHERE cv.recipient_type = 'supplier'
      AND cv.recipient_id = sqlc.arg('supplier_id')
      AND (sqlc.narg('from_date') IS NULL OR DATE(cv.effective_date) >= DATE(sqlc.narg('from_date')))
      AND (sqlc.narg('to_date') IS NULL OR DATE(cv.effective_date) <= DATE(sqlc.narg('to_date')))
)
SELECT
    COUNT(fb.id) AS bill_count,
    COALESCE(SUM(fb.total), 0) AS total_spent,
    COALESCE(SUM(fb.total_before_vat), 0) AS total_before_vat,
    COALESCE(SUM(fb.total_vat), 0) AS total_vat,
    COALESCE(SUM(fb.discount), 0) AS total_discount,
    (SELECT COALESCE(SUM(fp.amount), 0) FROM filtered_payments fp) AS total_payments,
    (SELECT COUNT(*) FROM filtered_payments fp) AS payment_count,
    LEAST((SELECT COALESCE(SUM(fp.amount), 0) FROM filtered_payments fp), COALESCE(SUM(fb.total), 0)) AS paid_total,
    GREATEST(COALESCE(SUM(fb.total), 0) - (SELECT COALESCE(SUM(fp.amount), 0) FROM filtered_payments fp), 0) AS unpaid_total,
    COALESCE(SUM(fb.total), 0) - (SELECT COALESCE(SUM(fp.amount), 0) FROM filtered_payments fp) AS closing_balance,
    CASE WHEN COUNT(fb.id) = 0 THEN 0 ELSE COALESCE(SUM(fb.total), 0) / COUNT(fb.id) END AS avg_bill,
    COUNT(CASE WHEN fb.received_at IS NOT NULL THEN 1 END) AS received_count
FROM filtered_bills fb;

-- name: ListSupplierReportBills :many
WITH bill_lines AS (
    SELECT
        pbp.bill_id,
        COUNT(*) AS item_count,
        ROUND(COALESCE(SUM(pbp.total_before_vat), 0), 2) AS total_before_vat,
        ROUND(COALESCE(SUM(pbp.vat_total), 0), 2) AS total_vat,
        ROUND(COALESCE(SUM(pbp.total_including_vat), 0), 2) AS total
    FROM purchase_bill_product pbp
    GROUP BY pbp.bill_id
)
SELECT
    pb.id,
    pb.sequence_number,
    CAST(COALESCE(pb.supplier_sequence_number, '') AS CHAR) AS supplier_sequence_number,
    COALESCE(bl.total, 0) AS total,
    COALESCE(bl.total_before_vat, 0) AS total_before_vat,
    COALESCE(bl.total_vat, 0) AS total_vat,
    COALESCE(pb.discount, 0) AS discount,
    pb.state,
    DATE_FORMAT(pb.effective_date, '%Y-%m-%dT%H:%i:%sZ') AS effective_date,
    COALESCE(DATE_FORMAT(pb.payment_due_date, '%Y-%m-%dT%H:%i:%sZ'), '') AS payment_due_date,
    COALESCE(DATE_FORMAT(pb.received_at, '%Y-%m-%dT%H:%i:%sZ'), '') AS received_at,
    CAST(COALESCE(pb.received_by, '') AS CHAR) AS received_by,
    COALESCE(bl.item_count, 0) AS item_count
FROM purchase_bill pb
LEFT JOIN bill_lines bl ON bl.bill_id = pb.id
WHERE pb.supplier_id = sqlc.arg('supplier_id')
  AND (sqlc.narg('from_date') IS NULL OR DATE(pb.effective_date) >= DATE(sqlc.narg('from_date')))
  AND (sqlc.narg('to_date') IS NULL OR DATE(pb.effective_date) <= DATE(sqlc.narg('to_date')))
ORDER BY pb.effective_date ASC, pb.id ASC;

-- name: ListSupplierReportPayments :many
SELECT
    cv.id,
    cv.voucher_number,
    cv.voucher_type,
    DATE_FORMAT(cv.effective_date, '%Y-%m-%dT%H:%i:%sZ') AS effective_date,
    COALESCE(cv.amount, 0) AS amount,
    COALESCE(cv.payment_method, '') AS payment_method,
    COALESCE(cv.description, '') AS description
FROM cash_voucher cv
WHERE cv.recipient_type = 'supplier'
  AND cv.recipient_id = sqlc.arg('supplier_id')
  AND (sqlc.narg('from_date') IS NULL OR DATE(cv.effective_date) >= DATE(sqlc.narg('from_date')))
  AND (sqlc.narg('to_date') IS NULL OR DATE(cv.effective_date) <= DATE(sqlc.narg('to_date')))
ORDER BY cv.effective_date ASC, cv.id ASC;

-- name: ListSupplierReportTopItems :many
SELECT
    COALESCE(NULLIF(pbp.name, ''), CAST(pbp.product_id AS CHAR), 'Unknown') AS item_name,
    COALESCE(SUM(pbp.quantity), 0) AS total_qty,
    COALESCE(SUM(pbp.total_before_vat), 0) AS total_value,
    CASE WHEN COALESCE(SUM(pbp.quantity), 0) = 0 THEN 0 ELSE COALESCE(SUM(pbp.total_before_vat), 0) / SUM(pbp.quantity) END AS avg_price,
    COUNT(DISTINCT pb.id) AS bill_count
FROM purchase_bill pb
INNER JOIN purchase_bill_product pbp ON pbp.bill_id = pb.id
WHERE pb.supplier_id = sqlc.arg('supplier_id')
  AND (sqlc.narg('from_date') IS NULL OR DATE(pb.effective_date) >= DATE(sqlc.narg('from_date')))
  AND (sqlc.narg('to_date') IS NULL OR DATE(pb.effective_date) <= DATE(sqlc.narg('to_date')))
GROUP BY item_name
ORDER BY total_value DESC, total_qty DESC
LIMIT 20;

-- name: ListSupplierReportAging :many
WITH bill_lines AS (
    SELECT
        pbp.bill_id,
        ROUND(COALESCE(SUM(pbp.total_including_vat), 0), 2) AS total
    FROM purchase_bill_product pbp
    GROUP BY pbp.bill_id
), aging_source AS (
    SELECT
        CASE
            WHEN pb.payment_due_date IS NULL OR DATE(pb.payment_due_date) >= CURRENT_DATE THEN 'current'
            WHEN DATEDIFF(CURRENT_DATE, DATE(pb.payment_due_date)) BETWEEN 1 AND 30 THEN '1-30'
            WHEN DATEDIFF(CURRENT_DATE, DATE(pb.payment_due_date)) BETWEEN 31 AND 60 THEN '31-60'
            WHEN DATEDIFF(CURRENT_DATE, DATE(pb.payment_due_date)) BETWEEN 61 AND 90 THEN '61-90'
            ELSE '90+'
        END AS bucket,
        COALESCE(bl.total, 0) AS total
    FROM purchase_bill pb
    LEFT JOIN bill_lines bl ON bl.bill_id = pb.id
    WHERE pb.supplier_id = sqlc.arg('supplier_id')
      AND (sqlc.narg('from_date') IS NULL OR DATE(pb.effective_date) >= DATE(sqlc.narg('from_date')))
      AND (sqlc.narg('to_date') IS NULL OR DATE(pb.effective_date) <= DATE(sqlc.narg('to_date')))
)
SELECT bucket, COUNT(*) AS bill_count, COALESCE(SUM(total), 0) AS bucket_total
FROM aging_source
GROUP BY bucket;

-- name: ListSupplierReportMonthlySpending :many
WITH bill_lines AS (
    SELECT
        pbp.bill_id,
        ROUND(COALESCE(SUM(pbp.total_including_vat), 0), 2) AS total
    FROM purchase_bill_product pbp
    GROUP BY pbp.bill_id
)
SELECT
    DATE_FORMAT(pb.effective_date, '%Y-%m') AS month,
    COALESCE(SUM(bl.total), 0) AS total_spent
FROM purchase_bill pb
LEFT JOIN bill_lines bl ON bl.bill_id = pb.id
WHERE pb.supplier_id = sqlc.arg('supplier_id')
  AND (sqlc.narg('from_date') IS NULL OR DATE(pb.effective_date) >= DATE(sqlc.narg('from_date')))
  AND (sqlc.narg('to_date') IS NULL OR DATE(pb.effective_date) <= DATE(sqlc.narg('to_date')))
GROUP BY month
ORDER BY month ASC;

-- name: GetSupplierReportPaymentBreakdown :one
SELECT
    COALESCE(SUM(CASE WHEN cv.payment_method = 'cash' THEN cv.amount ELSE 0 END), 0) AS cash_total,
    COALESCE(SUM(CASE WHEN cv.payment_method = 'bank_transfer' THEN cv.amount ELSE 0 END), 0) AS bank_transfer_total
FROM cash_voucher cv
WHERE cv.recipient_type = 'supplier'
  AND cv.recipient_id = sqlc.arg('supplier_id')
  AND (sqlc.narg('from_date') IS NULL OR DATE(cv.effective_date) >= DATE(sqlc.narg('from_date')))
  AND (sqlc.narg('to_date') IS NULL OR DATE(cv.effective_date) <= DATE(sqlc.narg('to_date')));