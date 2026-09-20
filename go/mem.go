package main

import (
	"database/sql"
	"sync"
)

type memChair struct {
	mu sync.RWMutex
	c  Chair
}

type memUser struct {
	mu sync.RWMutex
	u  User
}

var (
	chairMemByToken sync.Map // string -> *memChair
	chairMemByID    sync.Map // string -> *memChair
	userMemByToken  sync.Map // string -> *memUser
	userMemByID     sync.Map // string -> *memUser
)

func resetMemCaches() {
	chairMemByToken = sync.Map{}
	chairMemByID = sync.Map{}
	userMemByToken = sync.Map{}
	userMemByID = sync.Map{}
}

func rememberChair(c *Chair, token string) {
	cp := *c
	if token != "" {
		cp.AccessToken = token
	}
	m := &memChair{c: cp}
	if token != "" {
		chairMemByToken.Store(token, m)
	}
	if cp.ID != "" {
		chairMemByID.Store(cp.ID, m)
	}
}

func loadChairByToken(token string) (*Chair, bool) {
	v, ok := chairMemByToken.Load(token)
	if !ok {
		return nil, false
	}
	m := v.(*memChair)
	m.mu.RLock()
	cp := m.c
	m.mu.RUnlock()
	return &cp, true
}

func patchChairByID(id string, fn func(*Chair)) {
	if id == "" {
		return
	}
	v, ok := chairMemByID.Load(id)
	if !ok {
		return
	}
	m := v.(*memChair)
	m.mu.Lock()
	fn(&m.c)
	m.mu.Unlock()
}

func patchChairByRideID(rideID, status string) {
	if rideID == "" {
		return
	}
	chairMemByID.Range(func(_, v any) bool {
		m := v.(*memChair)
		m.mu.Lock()
		if m.c.ActiveRideID.Valid && m.c.ActiveRideID.String == rideID {
			m.c.ActiveRideStatus = sql.NullString{String: status, Valid: true}
		}
		m.mu.Unlock()
		return true
	})
}

func memAssignChair(chairID string, ride *Ride, user *User, status string) {
	patchChairByID(chairID, func(c *Chair) {
		c.IsFree = false
		c.ActiveRideID = sql.NullString{String: ride.ID, Valid: true}
		c.ActiveRideStatus = sql.NullString{String: status, Valid: true}
		c.ActivePickupLatitude = sql.NullInt64{Int64: int64(ride.PickupLatitude), Valid: true}
		c.ActivePickupLongitude = sql.NullInt64{Int64: int64(ride.PickupLongitude), Valid: true}
		c.ActiveDestinationLatitude = sql.NullInt64{Int64: int64(ride.DestinationLatitude), Valid: true}
		c.ActiveDestinationLongitude = sql.NullInt64{Int64: int64(ride.DestinationLongitude), Valid: true}
		if user != nil {
			c.ActiveUserID = sql.NullString{String: user.ID, Valid: true}
			c.ActiveUserFirstname = sql.NullString{String: user.Firstname, Valid: true}
			c.ActiveUserLastname = sql.NullString{String: user.Lastname, Valid: true}
		}
	})
}

func memFreeChair(chairID string) {
	patchChairByID(chairID, func(c *Chair) {
		c.IsFree = true
		c.ActiveRideID = sql.NullString{}
		c.ActiveRideStatus = sql.NullString{}
		c.ActivePickupLatitude = sql.NullInt64{}
		c.ActivePickupLongitude = sql.NullInt64{}
		c.ActiveDestinationLatitude = sql.NullInt64{}
		c.ActiveDestinationLongitude = sql.NullInt64{}
		c.ActiveUserID = sql.NullString{}
		c.ActiveUserFirstname = sql.NullString{}
		c.ActiveUserLastname = sql.NullString{}
	})
}

func rememberUser(u *User) {
	if u == nil || u.ID == "" {
		return
	}
	m := &memUser{u: *u}
	if u.AccessToken != "" {
		userMemByToken.Store(u.AccessToken, m)
	}
	userMemByID.Store(u.ID, m)
}

func loadUserByToken(token string) (*User, bool) {
	v, ok := userMemByToken.Load(token)
	if !ok {
		return nil, false
	}
	m := v.(*memUser)
	m.mu.RLock()
	cp := m.u
	m.mu.RUnlock()
	return &cp, true
}

func patchUserByID(id string, fn func(*User)) {
	if id == "" {
		return
	}
	v, ok := userMemByID.Load(id)
	if !ok {
		return
	}
	m := v.(*memUser)
	m.mu.Lock()
	fn(&m.u)
	m.mu.Unlock()
}
