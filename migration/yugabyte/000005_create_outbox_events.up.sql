CREATE TABLE IF NOT EXISTS outbox_events (
    event_id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    operation TEXT NOT NULL,
    schema_version INT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_aggregate
    ON outbox_events (aggregate_type, aggregate_id, created_at);

CREATE OR REPLACE FUNCTION capture_auth_outbox_event() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    row_data JSONB;
    aggregate_id_value TEXT;
    operation_value TEXT := CASE TG_OP WHEN 'INSERT' THEN 'created' WHEN 'DELETE' THEN 'deactivated' ELSE 'updated' END;
BEGIN
    row_data := CASE WHEN TG_OP = 'DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END;
    aggregate_id_value := COALESCE(row_data->>'user_id', row_data->>'email_verification_code_id', row_data->>'refresh_token_id', row_data->>'id');
    INSERT INTO outbox_events(event_id, aggregate_type, aggregate_id, event_type, operation, payload)
    VALUES (
        md5(clock_timestamp()::TEXT || random()::TEXT)::UUID,
        CASE WHEN TG_TABLE_NAME = 'auth_credentials' THEN 'auth' ELSE TG_TABLE_NAME END,
        aggregate_id_value,
        CASE WHEN TG_TABLE_NAME = 'auth_credentials' THEN 'auth.credentials.' || operation_value ELSE 'auth.' || TG_TABLE_NAME || '.changed' END,
        operation_value,
        row_data
    );
    IF TG_OP = 'DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END;
$$;

DROP TRIGGER IF EXISTS auth_credentials_outbox ON auth_credentials;
CREATE TRIGGER auth_credentials_outbox AFTER INSERT OR UPDATE OR DELETE ON auth_credentials
FOR EACH ROW EXECUTE FUNCTION capture_auth_outbox_event();
DROP TRIGGER IF EXISTS email_verification_codes_outbox ON email_verification_codes;
CREATE TRIGGER email_verification_codes_outbox AFTER INSERT OR UPDATE OR DELETE ON email_verification_codes
FOR EACH ROW EXECUTE FUNCTION capture_auth_outbox_event();
DROP TRIGGER IF EXISTS auth_user_roles_outbox ON auth_user_roles;
CREATE TRIGGER auth_user_roles_outbox AFTER INSERT OR UPDATE OR DELETE ON auth_user_roles
FOR EACH ROW EXECUTE FUNCTION capture_auth_outbox_event();
DROP TRIGGER IF EXISTS refresh_tokens_outbox ON refresh_tokens;
CREATE TRIGGER refresh_tokens_outbox AFTER INSERT OR UPDATE OR DELETE ON refresh_tokens
FOR EACH ROW EXECUTE FUNCTION capture_auth_outbox_event();
