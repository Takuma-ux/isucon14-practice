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