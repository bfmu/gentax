CREATE TABLE taxis (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id UUID NOT NULL REFERENCES owners(id),
  plate TEXT NOT NULL,
  model TEXT NOT NULL,
  year INT NOT NULL,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_taxi_year CHECK (year >= 1990 AND year <= EXTRACT(YEAR FROM now()) + 1),
  CONSTRAINT uq_owner_plate UNIQUE (owner_id, plate)
);
CREATE INDEX idx_taxis_owner_id ON taxis(owner_id);
