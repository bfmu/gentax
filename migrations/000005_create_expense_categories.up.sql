CREATE TABLE expense_categories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id UUID NOT NULL REFERENCES owners(id),
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_owner_category UNIQUE (owner_id, name)
);
CREATE INDEX idx_expense_categories_owner_id ON expense_categories(owner_id);
