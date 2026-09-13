ALTER TABLE chairs
    ADD COLUMN total_distance INT NOT NULL DEFAULT 0,
    ADD COLUMN total_distance_updated_at DATETIME(6) NULL,
    ADD INDEX idx_chairs_owner_id (owner_id);

ALTER TABLE chair_locations
    ADD INDEX idx_chair_locations_chair_id_created_at (chair_id, created_at);

UPDATE chairs
    LEFT JOIN (
        SELECT chair_id,
            SUM(IFNULL(distance, 0))AS total_distance,
            MAX(created_at) AS total_distance_updated_at
        FROM (
            SELECT chair_id,
                   created_at,
                   ABS(latitude - LAG(latitude) OVER (PARTITION BY chair_id ORDER BY created_at)) +
                   ABS(longitude - LAG(longitude) OVER (PARTITION BY chair_id ORDER BY created_at)) AS distance
            FROM chair_locations
        ) tmp
        GROUP BY chair_id
    ) d ON d.chair_id = chairs.id
SET chairs.total_distance = IFNULL(d.total_distance, 0),
    chairs.total_distance_updated_at = d.total_distance_updated_at;

ALTER TABLE ride_statuses
    ADD INDEX idx_ride_statuses_ride_id_created_at (ride_id, created_at);

ALTER TABLE rides
    ADD COLUMN latest_status VARCHAR(32) NULL,
    ADD INDEX idx_rides_chair_id(chair_id),
    ADD INDEX idx_rides_user_id(user_id),
    ADD INDEX idx_rides_chair_id_created_at(chair_id, created_at);

ALTER TABLE chairs
    ADD COLUMN latitude INT NULL,
    ADD COLUMN longitude INT NULL,
    ADD COLUMN speed INT NOT NULL DEFAULT 1,
    ADD COLUMN is_free TINYINT(1) NOT NULL DEFAULT 1,
    ADD INDEX idx_chairs_matching(is_active, is_free);

UPDATE rides r
    INNER JOIN (
        SELECT rs.ride_id, rs.status
        FROM ride_statuses rs
        INNER JOIN (
            SELECT ride_id, MAX(created_at) AS max_at
            FROM ride_statuses
            GROUP BY ride_id
        ) t ON t.ride_id = rs.ride_id AND t.max_at = rs.created_at
    ) x ON x.ride_id = r.id
SET r.latest_status = x.status,
    r.updated_at = r.updated_at;

UPDATE chairs c
    INNER JOIN chair_models m ON m.name = c.model
SET c.speed = m.speed,
    c.updated_at = c.updated_at;

UPDATE chairs c
    INNER JOIN (
        SELECT cl.chair_id, cl.latitude, cl.longitude
        FROM chair_locations cl
        INNER JOIN (
            SELECT chair_id, MAX(created_at) AS max_at
            FROM chair_locations
            GROUP BY chair_id
        ) t ON t.chair_id = cl.chair_id AND t.max_at = cl.created_at
    ) x ON x.chair_id = c.id
SET c.latitude = x.latitude,
    c.longitude = x.longitude,
    c.updated_at = c.updated_at;

UPDATE chairs c
LEFT JOIN (
    SELECT r.chair_id
    FROM rides r
    INNER JOIN ride_statuses rs ON rs.ride_id = r.id
    GROUP BY r.id, r.chair_id
    HAVING COUNT(rs.chair_sent_at) < 6
) busy ON busy.chair_id = c.id
SET c.is_free = (busy.chair_id IS NULL),
    c.updated_at = c.updated_at;