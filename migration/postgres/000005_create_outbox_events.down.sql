DROP TRIGGER IF EXISTS auth_credentials_outbox ON auth_credentials;
DROP TRIGGER IF EXISTS email_verification_codes_outbox ON email_verification_codes;
DROP TRIGGER IF EXISTS auth_user_roles_outbox ON auth_user_roles;
DROP TRIGGER IF EXISTS refresh_tokens_outbox ON refresh_tokens;
DROP FUNCTION IF EXISTS capture_auth_outbox_event();
DROP TABLE IF EXISTS outbox_events;
