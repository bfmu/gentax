CREATE TABLE receipts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  driver_id UUID NOT NULL REFERENCES drivers(id),
  taxi_id UUID NOT NULL REFERENCES taxis(id),
  storage_url TEXT NOT NULL,
  telegram_file_id TEXT,
  ocr_status TEXT NOT NULL DEFAULT 'pending'
    CHECK (ocr_status IN ('pending', 'processing', 'done', 'failed', 'skipped')),
  ocr_raw JSONB,
  extracted_total NUMERIC(12,2),
  extracted_date DATE,
  extracted_vendor TEXT,
  extracted_nit TEXT,
  extracted_cufe TEXT,
  extracted_concept TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_receipts_driver_id ON receipts(driver_id);
CREATE INDEX idx_receipts_ocr_status ON receipts(ocr_status) WHERE ocr_status = 'pending';
