package repository

const (
	createCredentialQuery = `
		INSERT INTO auth_credentials (user_id, email, password_hash, email_verified)
		VALUES ($1, $2, $3, FALSE)
		RETURNING user_id, email, password_hash, email_verified, created_at, updated_at
	`

	createVerificationCodeQuery = `
		INSERT INTO email_verification_codes (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`

	getCredentialByUserIDQuery = `
		SELECT user_id, email, password_hash, email_verified, created_at, updated_at
		FROM auth_credentials
		WHERE user_id = $1
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
)
