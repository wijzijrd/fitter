-- name: GetRegistrationMode :one
SELECT mode FROM registration_settings WHERE id = TRUE;

-- name: SetRegistrationMode :exec
UPDATE registration_settings SET mode = $1 WHERE id = TRUE;

-- name: ListRegistrationAllowlist :many
SELECT * FROM registration_allowlist ORDER BY added_at ASC;

-- name: IsAthleteAllowlisted :one
SELECT EXISTS (SELECT 1 FROM registration_allowlist WHERE strava_athlete_id = $1);

-- name: AddToRegistrationAllowlist :exec
INSERT INTO registration_allowlist (strava_athlete_id, added_by, added_at) VALUES ($1, $2, $3)
ON CONFLICT (strava_athlete_id) DO NOTHING;

-- name: RemoveFromRegistrationAllowlist :exec
DELETE FROM registration_allowlist WHERE strava_athlete_id = $1;
