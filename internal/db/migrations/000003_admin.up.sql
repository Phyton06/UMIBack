CREATE TABLE IF NOT EXISTS admins (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone      VARCHAR(20) UNIQUE NOT NULL,
    name       VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS suspended_until TIMESTAMPTZ;
ALTER TABLE drivers ADD COLUMN IF NOT EXISTS suspended_until TIMESTAMPTZ;

INSERT INTO admins (phone, name) VALUES ('+525500000001', 'Admin UMI');
INSERT INTO drivers (phone, name) VALUES ('+525500000002', 'Conductor Test');
INSERT INTO users (phone, name) VALUES ('+525500000003', 'Pasajero Test');
