CREATE TABLE expense_attachments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  expense_id UUID NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
  receipt_id UUID NOT NULL REFERENCES receipts(id),
  label TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_expense_attachments_expense_id ON expense_attachments(expense_id);
