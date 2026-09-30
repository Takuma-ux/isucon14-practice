package main

import (
	"database/sql"
	"errors"
	"net/http"
)

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// MEMO: 一旦最も待たせているリクエストに適当な空いている椅子マッチさせる実装とする。おそらくもっといい方法があるはず…
	ride := &Ride{}
	if err := db.GetContext(ctx, ride, `SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	matched := &Chair{}

	if err := db.GetContext(ctx, matched,
		'SELECT c.*
		FROM chairs c
		JOIN chair_locations loc ON loc.chair_id = c.id
		WHERE c.is_active = TRUE
			AND loc.created_at = (
				SELECT MAX(created_at)
				FROM chair_locations
				WHERE chair_id = c.id
			)
			AND NOT EXISTS(
				SELECT 1 
				FROM rides r
				WHERE r.chair_id = c.id
					AND r.status <> 'COMPLETED'
			)
		ORDER BY ABS(loc.latitude - ?) + ABS(loc.longitude - ?)
		LIMIT 1',
		ride.PickupLatitude, ride.PickupLongitude); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
		}

	if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matched.ID, ride.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
