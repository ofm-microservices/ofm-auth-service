package repository

const (
	createCredentialQuery = `
		INSERT INTO auth_credentials (user_id, email, username, password_hash, email_verified, status)
		VALUES ($1, $2, $3, $4, FALSE, 'pending_registration')
		RETURNING user_id, email, username, password_hash, email_verified, status, created_at, updated_at
	`

	createVerificationCodeQuery = `
		INSERT INTO email_verification_codes (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`

	verifyRegistrationEmailQuery = `
		WITH code AS (
			UPDATE email_verification_codes
			SET used_at = NOW()
			WHERE user_id = $1
			  AND token_hash = $2
			  AND used_at IS NULL
			  AND expires_at > $3
			RETURNING user_id
		)
		UPDATE auth_credentials
		SET email_verified = TRUE,
		    status = 'email_verified'
		FROM code
		WHERE auth_credentials.user_id = code.user_id
		RETURNING auth_credentials.user_id,
		          auth_credentials.email,
		          auth_credentials.username,
		          auth_credentials.password_hash,
		          auth_credentials.email_verified,
		          auth_credentials.status,
		          auth_credentials.created_at,
		          auth_credentials.updated_at
	`

	createRefreshTokenQuery = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`

	getCredentialByUserIDQuery = `
		SELECT user_id, email, username, password_hash, email_verified, status, created_at, updated_at
		FROM auth_credentials
		WHERE user_id = $1
	`

	getCredentialByIdentifierQuery = `
		SELECT user_id, email, username, password_hash, email_verified, status, created_at, updated_at
		FROM auth_credentials
		WHERE username = $1 OR email = $1
		LIMIT 1
	`

	existsCredentialByEmailQuery = `
		SELECT EXISTS (
			SELECT 1
			FROM auth_credentials
			WHERE email = $1
		)
	`

	deleteCredentialByUserIDQuery = `
		DELETE FROM auth_credentials
		WHERE user_id = $1
	`

	deactivateRegistrationAuthQuery = `
		UPDATE auth_credentials
		SET email_verified = FALSE,
			status = 'registration_failed'
		WHERE user_id = $1
	`
)
