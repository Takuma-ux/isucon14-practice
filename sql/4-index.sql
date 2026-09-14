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
    ADD COLUMN total_rides_count INT NOT NULL DEFAULT 0,
    ADD COLUMN total_evaluation_sum BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN active_ride_id VARCHAR(26) NULL,
    ADD COLUMN active_ride_status VARCHAR(32) NULL,
    ADD COLUMN active_pickup_latitude INT NULL,
    ADD COLUMN active_pickup_longitude INT NULL,
    ADD COLUMN active_destination_latitude INT NULL,
    ADD COLUMN active_destination_longitude INT NULL,
    ADD COLUMN active_user_id VARCHAR(26) NULL,
    ADD COLUMN active_user_firstname VARCHAR(30) NULL,
    ADD COLUMN active_user_lastname VARCHAR(30) NULL,
    ADD INDEX idx_chairs_matching(is_active, is_free),
    ADD INDEX idx_chairs_active_ride_id(active_ride_id);

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

-- getChairStats 用: ARRIVED+CARRYING+COMPLETED 完走の件数・評価合計を chairs に載せる
UPDATE chairs c
INNER JOIN (
    SELECT r.chair_id,
           COUNT(*) AS cnt,
           SUM(r.evaluation) AS eval_sum
    FROM rides r
    WHERE r.chair_id IS NOT NULL
      AND r.evaluation IS NOT NULL
      AND EXISTS (SELECT 1 FROM ride_statuses rs WHERE rs.ride_id = r.id AND rs.status = 'ARRIVED')
      AND EXISTS (SELECT 1 FROM ride_statuses rs WHERE rs.ride_id = r.id AND rs.status = 'CARRYING')
      AND EXISTS (SELECT 1 FROM ride_statuses rs WHERE rs.ride_id = r.id AND rs.status = 'COMPLETED')
    GROUP BY r.chair_id
) s ON s.chair_id = c.id
SET c.total_rides_count = s.cnt,
    c.total_evaluation_sum = IFNULL(s.eval_sum, 0),
    c.updated_at = c.updated_at;

-- coordinate 用: 進行中ライドを chairs に載せる（is_free=FALSE の椅子）
UPDATE chairs c
INNER JOIN rides r ON r.id = (
    SELECT r2.id
    FROM rides r2
    WHERE r2.chair_id = c.id
    ORDER BY r2.updated_at DESC
    LIMIT 1
)
INNER JOIN users u ON u.id = r.user_id
SET c.active_ride_id = r.id,
    c.active_ride_status = r.latest_status,
    c.active_pickup_latitude = r.pickup_latitude,
    c.active_pickup_longitude = r.pickup_longitude,
    c.active_destination_latitude = r.destination_latitude,
    c.active_destination_longitude = r.destination_longitude,
    c.active_user_id = u.id,
    c.active_user_firstname = u.firstname,
    c.active_user_lastname = u.lastname,
    c.updated_at = c.updated_at
WHERE c.is_free = FALSE;