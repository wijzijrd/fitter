-- Create the restricted runtime role the service serves with. The infrastructure's
-- deploy workflow sets its password from SSM (app_db_password); migrations only
-- ensure the role exists and is granted exactly what the app needs. Never superuser.
--
-- The name is fixed rather than parameterised: it must match APP_DB_ROLE in the
-- infrastructure repo, which derives it from PROJECT_NAME (fitter -> fitter_app).
--
-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fitter_app') THEN
        CREATE ROLE fitter_app LOGIN;
    END IF;
END
$$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA public TO fitter_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO fitter_app;

-- Tables added by later migrations inherit the same grants. This applies to objects
-- created by the role running migrations, which is always the migration user.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fitter_app;

-- +goose Down
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM fitter_app;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM fitter_app;
REVOKE USAGE ON SCHEMA public FROM fitter_app;
