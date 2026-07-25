DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'email_verification_codes'
          AND column_name = 'id'
    ) THEN
        EXECUTE 'ALTER TABLE email_verification_codes RENAME COLUMN id TO email_verification_code_id';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'refresh_tokens'
          AND column_name = 'id'
    ) THEN
        EXECUTE 'ALTER TABLE refresh_tokens RENAME COLUMN id TO refresh_token_id';
    END IF;
END
$$;
