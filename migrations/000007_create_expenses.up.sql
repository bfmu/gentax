CREATE TABLE expenses (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id UUID NOT NULL REFERENCES owners(id),
  driver_id UUID NOT NULL REFERENCES drivers(id),
  taxi_id UUID NOT NULL REFERENCES taxis(id),
  category_id UUID NOT NULL REFERENCES expense_categories(id),
  receipt_id UUID NOT NULL REFERENCES receipts(id),
  amount NUMERIC(12,2),
  expense_date DATE,
  notes TEXT,
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'confirmed', 'approved', 'rejected')),
  rejection_reason TEXT,
  reviewed_by UUID REFERENCES owners(id),
  reviewed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_review_consistency CHECK (
    (status IN ('approved', 'rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
    OR (status NOT IN ('approved', 'rejected'))
  )
);
CREATE INDEX idx_expenses_owner_id ON expenses(owner_id);
CREATE INDEX idx_expenses_driver_id ON expenses(driver_id);
CREATE INDEX idx_expenses_taxi_id ON expenses(taxi_id);
CREATE INDEX idx_expenses_status ON expenses(status);
CREATE INDEX idx_expenses_expense_date ON expenses(expense_date);
