CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone      VARCHAR(20) UNIQUE NOT NULL,
    name       VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE drivers (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone                 VARCHAR(20) UNIQUE NOT NULL,
    name                  VARCHAR(100) NOT NULL,
    location              GEOMETRY(Point, 4326),
    available             BOOLEAN DEFAULT false,
    membresia_active_until TIMESTAMPTZ,
    created_at            TIMESTAMPTZ DEFAULT NOW(),
    updated_at            TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE rides (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    passenger_id     UUID NOT NULL REFERENCES users(id),
    driver_id        UUID REFERENCES drivers(id),
    status           VARCHAR(20) NOT NULL DEFAULT 'REQUESTED',
    pickup_location  GEOMETRY(Point, 4326) NOT NULL,
    dropoff_location GEOMETRY(Point, 4326) NOT NULL,
    pickup_address   TEXT NOT NULL,
    dropoff_address  TEXT,
    fare             NUMERIC(10,2),
    cancelled_by     VARCHAR(20),
    cancelled_at     TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    updated_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_drivers_location ON drivers USING GIST (location);
CREATE INDEX idx_rides_pickup_location ON rides USING GIST (pickup_location);
CREATE INDEX idx_rides_dropoff_location ON rides USING GIST (dropoff_location);
