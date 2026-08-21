package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// wktToLatLng parsea WKT "POINT(lon lat)" a (lat, lng); ok=false si el string es vacío o inválido.
func wktToLatLng(wkt string) (lat, lng float64, ok bool) {
	if wkt == "" {
		return 0, 0, false
	}
	lon, lat, err := parsePoint(wkt)
	if err != nil {
		return 0, 0, false
	}
	return lat, lon, true
}

// ActiveRides retorna los viajes en curso con ubicaciones de conductor para el mapa admin.
//
// Request:  GET /admin/rides/active
// Response: 200 [{ride_id, status, passenger_name, driver_id, driver_name,
//
//	driver_lat, driver_lng, pickup_lat, pickup_lng, dropoff_lat, dropoff_lng, started_at}]
//
// Errors:   500 si la consulta falla
func ActiveRides(pool Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(),
			`SELECT
				r.id AS ride_id,
				r.status,
				u.name AS passenger_name,
				d.id AS driver_id,
				d.name AS driver_name,
				ST_AsText(d.location) AS driver_loc,
				ST_AsText(r.pickup_location) AS pickup_loc,
				ST_AsText(r.dropoff_location) AS dropoff_loc,
				r.created_at,
				r.updated_at
			FROM rides r
			JOIN users u ON u.id = r.passenger_id
			JOIN drivers d ON d.id = r.driver_id
			WHERE r.status IN ('ACCEPTED','EN_ROUTE','IN_PROGRESS')
			  AND d.location IS NOT NULL
			ORDER BY r.created_at DESC`)
		if err != nil {
			slog.Error("active rides: query", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		defer rows.Close()

		type activeRide struct {
			RideID        string    `json:"ride_id"`
			Status        string    `json:"status"`
			PassengerName string    `json:"passenger_name"`
			DriverID      string    `json:"driver_id"`
			DriverName    string    `json:"driver_name"`
			DriverLat     float64   `json:"driver_lat"`
			DriverLng     float64   `json:"driver_lng"`
			PickupLat     *float64  `json:"pickup_lat"`
			PickupLng     *float64  `json:"pickup_lng"`
			DropoffLat    *float64  `json:"dropoff_lat"`
			DropoffLng    *float64  `json:"dropoff_lng"`
			StartedAt     time.Time `json:"started_at"`
		}

		rides := make([]activeRide, 0)
		for rows.Next() {
			var rideID, driverID uuid.UUID
			var status, passengerName, driverName string
			var driverLoc, pickupLoc, dropoffLoc string
			var createdAt, updatedAt time.Time
			if err := rows.Scan(&rideID, &status, &passengerName, &driverID, &driverName,
				&driverLoc, &pickupLoc, &dropoffLoc, &createdAt, &updatedAt); err != nil {
				slog.Error("active rides: scan", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}

			ride := activeRide{
				RideID:        rideID.String(),
				Status:        status,
				PassengerName: passengerName,
				DriverID:      driverID.String(),
				DriverName:    driverName,
				StartedAt:     createdAt,
			}
			if lat, lng, ok := wktToLatLng(driverLoc); ok {
				ride.DriverLat, ride.DriverLng = lat, lng
			}
			if lat, lng, ok := wktToLatLng(pickupLoc); ok {
				ride.PickupLat, ride.PickupLng = &lat, &lng
			}
			if lat, lng, ok := wktToLatLng(dropoffLoc); ok {
				ride.DropoffLat, ride.DropoffLng = &lat, &lng
			}
			rides = append(rides, ride)
		}
		if rows.Err() != nil {
			slog.Error("active rides: rows iteration", "error", rows.Err())
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, rides)
	}
}
