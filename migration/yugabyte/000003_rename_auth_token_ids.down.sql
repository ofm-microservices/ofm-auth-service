ALTER TABLE refresh_tokens
    RENAME COLUMN refresh_token_id TO id;

ALTER TABLE email_verification_codes
    RENAME COLUMN email_verification_code_id TO id;
