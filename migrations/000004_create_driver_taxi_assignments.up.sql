CREATE TABLE driver_taxi_assignments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  driver_id UUID NOT NULL REFERENCES drivers(id),
  taxi_id UUID NOT NULL REFERENCES taxis(id),
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  unassigned_at TIMESTAMPTZ,
  CONSTRAINT chk_unassigned_after_assigned CHECK (unassigned_at IS NULL OR unassigned_at > assigned_at)
);
CREATE INDEX idx_dta_driver_id ON driver_taxi_assignments(driver_id);
CREATE INDEX idx_dta_taxi_id ON driver_taxi_assignments(taxi_id);
CREATE UNIQUE INDEX idx_dta_active_assignment ON driver_taxi_assignments(driver_id) WHERE unassigned_at IS NULL;
