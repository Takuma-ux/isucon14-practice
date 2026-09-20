package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
)

func appAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, err := r.Cookie("app_session")
		if errors.Is(err, http.ErrNoCookie) || c.Value == "" {
			writeError(w, http.StatusUnauthorized, errors.New("app_session cookie is required"))
			return
		}
		accessToken := c.Value
		if cached, ok := loadUserByToken(accessToken); ok {
			ctx = context.WithValue(ctx, "user", cached)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		user := &User{}
		err = db.GetContext(ctx, user, "SELECT * FROM users WHERE access_token = ?", accessToken)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		rememberUser(user)

		ctx = context.WithValue(ctx, "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ownerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, err := r.Cookie("owner_session")
		if errors.Is(err, http.ErrNoCookie) || c.Value == "" {
			writeError(w, http.StatusUnauthorized, errors.New("owner_session cookie is required"))
			return
		}
		accessToken := c.Value
		owner := &Owner{}
		if err := db.GetContext(ctx, owner, "SELECT * FROM owners WHERE access_token = ?", accessToken); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		ctx = context.WithValue(ctx, "owner", owner)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func chairAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, err := r.Cookie("chair_session")
		if errors.Is(err, http.ErrNoCookie) || c.Value == "" {
			writeError(w, http.StatusUnauthorized, errors.New("chair_session cookie is required"))
			return
		}
		accessToken := c.Value
		if cached, ok := loadChairByToken(accessToken); ok {
			ctx = context.WithValue(ctx, "chair", cached)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		chair := &Chair{}
		// coordinate / notification で使う列だけ読む（SELECT * は行が肥大化している）
		err = db.GetContext(
			ctx,
			chair,
			`SELECT id,
			        latitude, longitude,
			        active_ride_id, active_ride_status,
			        active_pickup_latitude, active_pickup_longitude,
			        active_destination_latitude, active_destination_longitude,
			        active_user_id, active_user_firstname, active_user_lastname
			 FROM chairs WHERE access_token = ?`,
			accessToken,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		rememberChair(chair, accessToken)

		ctx = context.WithValue(ctx, "chair", chair)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
