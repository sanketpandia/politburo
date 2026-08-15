package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const getByDiscordIDQuery = `
SELECT id, discord_id, if_community_id, if_api_id, is_active, username, created_at, updated_at, otp
FROM public.users
WHERE discord_id = $1
`

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetByDiscordID returns the user with the given Discord id.
// A missing row is (nil, nil); only database failures return an error.
func (r *Repository) GetByDiscordID(ctx context.Context, discordID string) (*User, error) {
	var user User
	var ifCommunityID, ifAPIID, username, otp sql.NullString
	err := r.db.QueryRowContext(ctx, getByDiscordIDQuery, discordID).Scan(
		&user.ID,
		&user.DiscordID,
		&ifCommunityID,
		&ifAPIID,
		&user.IsActive,
		&username,
		&user.CreatedAt,
		&user.UpdatedAt,
		&otp,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup user by discord_id: %w", err)
	}
	user.IFCommunityID = nullString(ifCommunityID)
	user.IFApiID = nullString(ifAPIID)
	user.Username = nullString(username)
	user.OTP = nullString(otp)
	return &user, nil
}

func nullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
