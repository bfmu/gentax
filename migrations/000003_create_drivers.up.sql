CREATE TABLE drivers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id UUID NOT NULL REFERENCES owners(id),
  telegram_id BIGINT UNIQUE,
  full_name TEXT NOT NULL,
  phone TEXT,
  active BOOLEAN NOT NULL DEFAULT true,
  link_token TEXT,
  link_token_expires_at TIMESTAMPTZ,
  link_token_used BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_drivers_owner_id ON drivers(owner_id);
CREATE INDEX idx_drivers_telegram_id ON drivers(telegram_id) WHERE telegram_id IS NOT NULL;
