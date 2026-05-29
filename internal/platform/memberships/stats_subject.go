package memberships

import (
	"context"
	"fmt"
)

// PilotStatsSubject contains the resolved subject context for pilot stats requests.
type PilotStatsSubject struct {
	UserID            string  `gorm:"column:user_id"`
	DiscordID         string  `gorm:"column:discord_id"`
	IFCommunityID     string  `gorm:"column:if_community_id"`
	VAID              string  `gorm:"column:va_id"`
	VAName            string  `gorm:"column:va_name"`
	Role              string  `gorm:"column:role"`
	Callsign          string  `gorm:"column:callsign"`
	AirtablePilotID   *string `gorm:"column:airtable_pilot_id"`
	CareerModePilotID *string `gorm:"column:career_mode_pilot_id"`
}

// GetPilotStatsSubject resolves the pilot stats subject context for a Discord user in a VA.
func (r *Repository) GetPilotStatsSubject(ctx context.Context, discordUserID, vaID string) (*PilotStatsSubject, error) {
	const query = `
		SELECT
			u.id AS user_id,
			u.discord_id,
			u.if_community_id,
			vur.va_id,
			va.name AS va_name,
			vur.role,
			vur.callsign,
			vur.airtable_pilot_id,
			vur.career_mode_pilot_id
		FROM users u
		JOIN va_user_roles vur ON u.id = vur.user_id
		JOIN virtual_airlines va ON vur.va_id = va.id
		WHERE u.discord_id = ? AND vur.va_id = ? AND vur.is_active = true
		LIMIT 1
	`

	var subject PilotStatsSubject
	err := r.db.WithContext(ctx).Raw(query, discordUserID, vaID).Scan(&subject).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get pilot stats subject: %w", err)
	}
	if subject.UserID == "" {
		return nil, fmt.Errorf("pilot stats subject not found")
	}

	return &subject, nil
}

// GetPilotStatsSubject resolves the pilot stats subject through the platform service.
func (s *Service) GetPilotStatsSubject(ctx context.Context, discordUserID, vaID string) (*PilotStatsSubject, error) {
	return s.repo.GetPilotStatsSubject(ctx, discordUserID, vaID)
}
