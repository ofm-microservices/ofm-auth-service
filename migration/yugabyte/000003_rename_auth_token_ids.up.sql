ALTER TABLE email_verification_codes
    RENAME COLUMN id TO email_verification_code_id;

ALTER TABLE refresh_tokens
    RENAME COLUMN id TO refresh_token_id;
