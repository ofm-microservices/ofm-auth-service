DROP TRIGGER IF EXISTS trg_auth_credentials_updated_at ON auth_credentials;
DROP FUNCTION IF EXISTS set_updated_at();
DROP TABLE IF EXISTS email_verification_codes;
DROP TABLE IF EXISTS auth_credentials;
