-- Indexes for admin dashboard stats and real-time map queries.
CREATE INDEX IF NOT EXISTS idx_rides_status_created ON rides(status, created_at);
CREATE INDEX IF NOT EXISTS idx_drivers_available ON drivers(available);
CREATE INDEX IF NOT EXISTS idx_rides_driver_status ON rides(driver_id, status);
