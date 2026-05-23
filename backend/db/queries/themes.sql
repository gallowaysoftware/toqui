-- name: ListThemes :many
SELECT slug, display_name, description, icon, created_at
FROM themes
ORDER BY display_name;

-- name: GetTheme :one
SELECT slug, display_name, description, icon, created_at
FROM themes
WHERE slug = $1;

-- name: CreateTheme :one
INSERT INTO themes (slug, display_name, description, icon)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTripThemes :many
SELECT t.slug, t.display_name, t.description, t.icon, tt.confidence, tt.source
FROM trip_themes tt
JOIN themes t ON t.slug = tt.theme_slug
WHERE tt.trip_id = $1
ORDER BY tt.confidence DESC;

-- name: SetTripTheme :exec
INSERT INTO trip_themes (trip_id, theme_slug, confidence, source)
VALUES ($1, $2, $3, $4)
ON CONFLICT (trip_id, theme_slug) DO UPDATE
SET confidence = EXCLUDED.confidence, source = EXCLUDED.source;

-- name: RemoveTripTheme :exec
DELETE FROM trip_themes
WHERE trip_id = $1 AND theme_slug = $2;

-- name: ClearTripThemes :exec
DELETE FROM trip_themes
WHERE trip_id = $1 AND source = 'ai';
