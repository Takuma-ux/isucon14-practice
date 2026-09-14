package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"
	"github.com/oklog/ulid/v2"
)

type chairPostChairsRequest struct {
	Name               string `json:"name"`
	Model              string `json:"model"`
	ChairRegisterToken string `json:"chair_register_token"`
}

type chairPostChairsResponse struct {
	ID      string `json:"id"`
	OwnerID string `json:"owner_id"`
}

func chairPostChairs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &chairPostChairsRequest{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Model == "" || req.ChairRegisterToken == "" {
		writeError(w, http.StatusBadRequest, errors.New("some of required fields(name, model, chair_register_token) are empty"))
		return
	}

	owner := &Owner{}
	if err := db.GetContext(ctx, owner, "SELECT * FROM owners WHERE chair_register_token = ?", req.ChairRegisterToken); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, errors.New("invalid chair_register_token"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	chairID := ulid.Make().String()
	accessToken := secureRandomStr(32)

	speed := 1
	if err := db.GetContext(ctx, &speed, `SELECT speed FROM chair_models WHERE name = ?`, req.Model); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		speed = 1
	}

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO chairs (id, owner_id, name, model, is_active, access_token, speed, is_free) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		chairID, owner.ID, req.Name, req.Model, false, accessToken, speed, true,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Path:  "/",
		Name:  "chair_session",
		Value: accessToken,
	})

	writeJSON(w, http.StatusCreated, &chairPostChairsResponse{
		ID:      chairID,
		OwnerID: owner.ID,
	})
}

type postChairActivityRequest struct {
	IsActive bool `json:"is_active"`
}

func chairPostActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	chair := ctx.Value("chair").(*Chair)

	req := &postChairActivityRequest{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err := db.ExecContext(ctx, "UPDATE chairs SET is_active = ? WHERE id = ?", req.IsActive, chair.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type chairPostCoordinateResponse struct {
	RecordedAt int64 `json:"recorded_at"`
}

func chairPostCoordinate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &Coordinate{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	chair := ctx.Value("chair").(*Chair)
	recordedAt := time.Now()

	distance := 0
	if chair.Latitude.Valid && chair.Longitude.Valid {
		distance = calculateDistance(
			int(chair.Latitude.Int64),
			int(chair.Longitude.Int64),
			req.Latitude,
			req.Longitude,
		)
	}

	// chair_locations への履歴 INSERT はしない（Go 経路は chairs の lat/lng / total_distance で足りる）
	if _, err := db.ExecContext(
		ctx,
		`UPDATE chairs
		SET total_distance = total_distance + ?,
			total_distance_updated_at = ?,
			latitude = ?,
			longitude = ?
		WHERE id = ?`,
		distance, recordedAt, req.Latitude, req.Longitude, chair.ID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// middleware で読んだ chairs.active_ride_* を使う（rides の ORDER BY SELECT をしない）
	if chair.ActiveRideID.Valid {
		status := chair.ActiveRideStatus.String
		if status != "COMPLETED" && status != "CANCELED" {
			if req.Latitude == int(chair.ActivePickupLatitude.Int64) &&
				req.Longitude == int(chair.ActivePickupLongitude.Int64) &&
				status == "ENROUTE" {
				if err := insertRideStatus(ctx, db, chair.ActiveRideID.String, "PICKUP"); err != nil {
					writeError(w, http.StatusInternalServerError, err)
					return
				}
			}

			if req.Latitude == int(chair.ActiveDestinationLatitude.Int64) &&
				req.Longitude == int(chair.ActiveDestinationLongitude.Int64) &&
				status == "CARRYING" {
				if err := insertRideStatus(ctx, db, chair.ActiveRideID.String, "ARRIVED"); err != nil {
					writeError(w, http.StatusInternalServerError, err)
					return
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, &chairPostCoordinateResponse{
		RecordedAt: recordedAt.UnixMilli(),
	})
}

type simpleUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type chairGetNotificationResponse struct {
	Data         *chairGetNotificationResponseData `json:"data"`
	RetryAfterMs int                               `json:"retry_after_ms"`
}

type chairGetNotificationResponseData struct {
	RideID                string     `json:"ride_id"`
	User                  simpleUser `json:"user"`
	PickupCoordinate      Coordinate `json:"pickup_coordinate"`
	DestinationCoordinate Coordinate `json:"destination_coordinate"`
	Status                string     `json:"status"`
}

func chairGetNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	chair := ctx.Value("chair").(*Chair)

	if !chair.ActiveRideID.Valid {
		writeJSON(w, http.StatusOK, &chairGetNotificationResponse{
			RetryAfterMs: 30,
		})
		return
	}

	rideID := chair.ActiveRideID.String
	yetSentRideStatus := RideStatus{}
	status := ""
	if err := db.GetContext(ctx, &yetSentRideStatus, `SELECT * FROM ride_statuses WHERE ride_id = ? AND chair_sent_at IS NULL ORDER BY created_at ASC LIMIT 1`, rideID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			status = chair.ActiveRideStatus.String
		} else {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		status = yetSentRideStatus.Status
	}

	if yetSentRideStatus.ID != "" {
		if _, err := db.ExecContext(ctx, `UPDATE ride_statuses SET chair_sent_at = CURRENT_TIMESTAMP(6) WHERE id = ?`, yetSentRideStatus.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		// 評価完了の INSERT 時点では空けない。椅子が COMPLETED を受け取るまで次の配車を付けない
		if yetSentRideStatus.Status == "COMPLETED" {
			if _, err := db.ExecContext(
				ctx,
				`UPDATE chairs
				 SET is_free = TRUE,
				     active_ride_id = NULL,
				     active_ride_status = NULL,
				     active_pickup_latitude = NULL,
				     active_pickup_longitude = NULL,
				     active_destination_latitude = NULL,
				     active_destination_longitude = NULL,
				     active_user_id = NULL,
				     active_user_firstname = NULL,
				     active_user_lastname = NULL
				 WHERE id = ?`,
				chair.ID,
			); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, &chairGetNotificationResponse{
		Data: &chairGetNotificationResponseData{
			RideID: rideID,
			User: simpleUser{
				ID:   chair.ActiveUserID.String,
				Name: fmt.Sprintf("%s %s", chair.ActiveUserFirstname.String, chair.ActiveUserLastname.String),
			},
			PickupCoordinate: Coordinate{
				Latitude:  int(chair.ActivePickupLatitude.Int64),
				Longitude: int(chair.ActivePickupLongitude.Int64),
			},
			DestinationCoordinate: Coordinate{
				Latitude:  int(chair.ActiveDestinationLatitude.Int64),
				Longitude: int(chair.ActiveDestinationLongitude.Int64),
			},
			Status: status,
		},
		RetryAfterMs: 30,
	})
}

type postChairRidesRideIDStatusRequest struct {
	Status string `json:"status"`
}

func chairPostRideStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rideID := r.PathValue("ride_id")

	chair := ctx.Value("chair").(*Chair)

	req := &postChairRidesRideIDStatusRequest{}
	if err := bindJSON(r, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	tx, err := db.Beginx()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()

	ride := &Ride{}
	if err := tx.GetContext(ctx, ride, "SELECT * FROM rides WHERE id = ? FOR UPDATE", rideID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("ride not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if ride.ChairID.String != chair.ID {
		writeError(w, http.StatusBadRequest, errors.New("not assigned to this ride"))
		return
	}

	switch req.Status {
	// Acknowledge the ride
	case "ENROUTE":
		if err := insertRideStatus(ctx, tx, ride.ID, "ENROUTE"); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	// After Picking up user
	case "CARRYING":
		status, err := getLatestRideStatus(ctx, tx, ride.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if status != "PICKUP" {
			writeError(w, http.StatusBadRequest, errors.New("chair has not arrived yet"))
			return
		}
		if err := insertRideStatus(ctx, tx, ride.ID, "CARRYING"); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, errors.New("invalid status"))
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
