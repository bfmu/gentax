-- name: CreateReceipt :one
INSERT INTO receipts (driver_id, taxi_id, storage_url, telegram_file_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetReceiptByID :one
SELECT * FROM receipts
WHERE id = $1;

-- name: ListPendingOCRReceipts :many
SELECT * FROM receipts
WHERE ocr_status = 'pending'
ORDER BY created_at ASC
FOR UPDATE SKIP LOCKED
LIMIT 5;

-- name: UpdateReceiptOCRFields :one
UPDATE receipts
SET ocr_status      = $1,
    ocr_raw         = $2,
    extracted_total = $3,
    extracted_date  = $4,
    extracted_vendor = $5,
    extracted_nit   = $6,
    extracted_cufe  = $7,
    extracted_concept = $8
WHERE id = $9
RETURNING *;

-- name: UpdateReceiptOCRStatus :one
UPDATE receipts
SET ocr_status = $1
WHERE id = $2
RETURNING *;

-- name: MarkReceiptOCRFailed :one
UPDATE receipts
SET ocr_status = 'failed'
WHERE id = $1
RETURNING *;
