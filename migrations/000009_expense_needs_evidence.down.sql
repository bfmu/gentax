BEGIN;

-- First update any needs_evidence rows to confirmed (safety)
UPDATE expenses SET status = 'confirmed' WHERE status = 'needs_evidence';

-- Drop new constraint and restore old one
ALTER TABLE expenses DROP CONSTRAINT IF EXISTS expenses_status_check;
ALTER TABLE expenses ADD CONSTRAINT expenses_status_check
  CHECK (status IN ('pending', 'confirmed', 'approved', 'rejected'));

COMMIT;
