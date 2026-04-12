BEGIN;

-- Drop existing CHECK constraint on status
ALTER TABLE expenses DROP CONSTRAINT IF EXISTS expenses_status_check;

-- Add new CHECK including needs_evidence
ALTER TABLE expenses ADD CONSTRAINT expenses_status_check
  CHECK (status IN ('pending', 'confirmed', 'needs_evidence', 'approved', 'rejected'));

COMMIT;
