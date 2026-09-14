package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
)

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 0.1s 間隔でも、待ちライドが無いときはトランザクションを開かない
	var waiting string
	if err := db.GetContext(ctx, &waiting, `SELECT id FROM rides WHERE chair_id IS NULL LIMIT 1`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	for {
		matched, err := matchOneRide(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if !matched {
			break
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func matchOneRide(ctx context.Context) (bool, error) {
	tx, err := db.Beginx()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	ride := &Ride{}
	if err := tx.GetContext(
		ctx,
		ride,
		`SELECT id, user_id, pickup_latitude, pickup_longitude,
		        destination_latitude, destination_longitude, latest_status
		 FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1`,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	var matchedID string
	if err := tx.GetContext(
		ctx,
		&matchedID,
		`SELECT id FROM chairs
		 WHERE is_active = TRUE
		   AND is_free = TRUE
		   AND latitude IS NOT NULL
		   AND longitude IS NOT NULL
		 ORDER BY (ABS(latitude - ?) + ABS(longitude - ?)) / IF(speed > 0, speed, 1) ASC
		 LIMIT 1`,
		ride.PickupLatitude, ride.PickupLongitude,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	user := &User{}
	if err := tx.GetContext(ctx, user, `SELECT id, firstname, lastname FROM users WHERE id = ?`, ride.UserID); err != nil {
		return false, err
	}

	status := ride.LatestStatus.String
	if status == "" {
		status = "MATCHING"
	}
	result, err := tx.ExecContext(
		ctx,
		`UPDATE chairs
		 SET is_free = FALSE,
		     active_ride_id = ?,
		     active_ride_status = ?,
		     active_pickup_latitude = ?,
		     active_pickup_longitude = ?,
		     active_destination_latitude = ?,
		     active_destination_longitude = ?,
		     active_user_id = ?,
		     active_user_firstname = ?,
		     active_user_lastname = ?
		 WHERE id = ? AND is_free = TRUE`,
		ride.ID, status,
		ride.PickupLatitude, ride.PickupLongitude,
		ride.DestinationLatitude, ride.DestinationLongitude,
		user.ID, user.Firstname, user.Lastname,
		matchedID,
	)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return true, nil
	}

	result, err = tx.ExecContext(ctx, `UPDATE rides SET chair_id = ? WHERE id = ? AND chair_id IS NULL`, matchedID, ride.ID)
	if err != nil {
		return false, err
	}
	n, err = result.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return true, nil
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
