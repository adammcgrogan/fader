package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adammcgrogan/fader/internal/models"
)

type Queries struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Queries {
	return &Queries{pool: pool}
}

// ── Users ──────────────────────────────────────────────────────────────────

func (q *Queries) CreateUser(ctx context.Context, id uuid.UUID, email string) (*models.User, error) {
	u := &models.User{}
	err := q.pool.QueryRow(ctx,
		`INSERT INTO users (id, email) VALUES ($1, $2)
		 RETURNING id, email, tier, stripe_customer_id, is_admin, created_at`,
		id, email,
	).Scan(&u.ID, &u.Email, &u.Tier, &u.StripeCustomerID, &u.IsAdmin, &u.CreatedAt)
	return u, err
}

func (q *Queries) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u := &models.User{}
	err := q.pool.QueryRow(ctx,
		`SELECT id, email, tier, stripe_customer_id, is_admin, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.Tier, &u.StripeCustomerID, &u.IsAdmin, &u.CreatedAt)
	return u, err
}

func (q *Queries) SetUserTier(ctx context.Context, userID uuid.UUID, tier string) error {
	_, err := q.pool.Exec(ctx, `UPDATE users SET tier = $1 WHERE id = $2`, tier, userID)
	return err
}

func (q *Queries) SetStripeCustomerID(ctx context.Context, userID uuid.UUID, customerID string) error {
	_, err := q.pool.Exec(ctx, `UPDATE users SET stripe_customer_id = $1 WHERE id = $2`, customerID, userID)
	return err
}

func (q *Queries) ListAllUsers(ctx context.Context) ([]*models.User, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, email, tier, stripe_customer_id, is_admin, created_at FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(&u.ID, &u.Email, &u.Tier, &u.StripeCustomerID, &u.IsAdmin, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ── Profiles ───────────────────────────────────────────────────────────────

func (q *Queries) CreateProfile(ctx context.Context, userID uuid.UUID, handle, displayName string) (*models.Profile, error) {
	p := &models.Profile{}
	err := q.pool.QueryRow(ctx,
		`INSERT INTO profiles (user_id, handle, display_name) VALUES ($1, $2, $3)
		 RETURNING id, user_id, handle, display_name, avatar_url, template, bio, genres, accent_color, background_color, font_family, hide_footer, discover_hidden, created_at`,
		userID, handle, displayName,
	).Scan(&p.ID, &p.UserID, &p.Handle, &p.DisplayName, &p.AvatarURL, &p.Template, &p.Bio, &p.Genres, &p.AccentColor, &p.BackgroundColor, &p.FontFamily, &p.HideFooter, &p.DiscoverHidden, &p.CreatedAt)
	return p, err
}

func (q *Queries) GetProfileByHandle(ctx context.Context, handle string) (*models.Profile, error) {
	p := &models.Profile{}
	err := q.pool.QueryRow(ctx,
		`SELECT id, user_id, handle, display_name, avatar_url, template, bio, genres, accent_color, background_color, font_family, hide_footer, discover_hidden, created_at
		 FROM profiles WHERE handle = $1`,
		handle,
	).Scan(&p.ID, &p.UserID, &p.Handle, &p.DisplayName, &p.AvatarURL, &p.Template, &p.Bio, &p.Genres, &p.AccentColor, &p.BackgroundColor, &p.FontFamily, &p.HideFooter, &p.DiscoverHidden, &p.CreatedAt)
	return p, err
}

func (q *Queries) GetProfileByID(ctx context.Context, id uuid.UUID) (*models.Profile, error) {
	p := &models.Profile{}
	err := q.pool.QueryRow(ctx,
		`SELECT id, user_id, handle, display_name, avatar_url, template, bio, genres, accent_color, background_color, font_family, hide_footer, discover_hidden, created_at
		 FROM profiles WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.UserID, &p.Handle, &p.DisplayName, &p.AvatarURL, &p.Template, &p.Bio, &p.Genres, &p.AccentColor, &p.BackgroundColor, &p.FontFamily, &p.HideFooter, &p.DiscoverHidden, &p.CreatedAt)
	return p, err
}

func (q *Queries) GetProfilesByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Profile, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, user_id, handle, display_name, avatar_url, template, bio, genres, accent_color, background_color, font_family, hide_footer, discover_hidden, created_at
		 FROM profiles WHERE user_id = $1 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []*models.Profile
	for rows.Next() {
		p := &models.Profile{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.Handle, &p.DisplayName, &p.AvatarURL, &p.Template, &p.Bio, &p.Genres, &p.AccentColor, &p.BackgroundColor, &p.FontFamily, &p.HideFooter, &p.DiscoverHidden, &p.CreatedAt); err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (q *Queries) CountProfilesByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := q.pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

func (q *Queries) UpdateProfile(ctx context.Context, id uuid.UUID, displayName string, bio *string, avatarURL *string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE profiles SET display_name = $1, bio = $2, avatar_url = $3 WHERE id = $4`,
		displayName, bio, avatarURL, id,
	)
	return err
}

func (q *Queries) UpdateProfileTemplate(ctx context.Context, id uuid.UUID, template string) error {
	_, err := q.pool.Exec(ctx, `UPDATE profiles SET template = $1 WHERE id = $2`, template, id)
	return err
}

func (q *Queries) UpdateProfileHandle(ctx context.Context, id uuid.UUID, handle string) error {
	_, err := q.pool.Exec(ctx, `UPDATE profiles SET handle = $1 WHERE id = $2`, handle, id)
	return err
}

func (q *Queries) DeleteProfile(ctx context.Context, id uuid.UUID) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM profiles WHERE id = $1`, id)
	return err
}

func (q *Queries) HandleExists(ctx context.Context, handle string) (bool, error) {
	var exists bool
	err := q.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM profiles WHERE handle = $1)`, handle).Scan(&exists)
	return exists, err
}

func (q *Queries) ListAllProfiles(ctx context.Context) ([]*models.Profile, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, user_id, handle, display_name, avatar_url, template, bio, genres, accent_color, background_color, font_family, hide_footer, discover_hidden, created_at
		 FROM profiles ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []*models.Profile
	for rows.Next() {
		p := &models.Profile{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.Handle, &p.DisplayName, &p.AvatarURL, &p.Template, &p.Bio, &p.Genres, &p.AccentColor, &p.BackgroundColor, &p.FontFamily, &p.HideFooter, &p.DiscoverHidden, &p.CreatedAt); err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (q *Queries) ListDiscoverProfiles(ctx context.Context) ([]*models.Profile, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, user_id, handle, display_name, avatar_url, template, bio, genres, accent_color, background_color, font_family, hide_footer, discover_hidden, created_at
		 FROM profiles WHERE discover_hidden = FALSE ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []*models.Profile
	for rows.Next() {
		p := &models.Profile{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.Handle, &p.DisplayName, &p.AvatarURL, &p.Template, &p.Bio, &p.Genres, &p.AccentColor, &p.BackgroundColor, &p.FontFamily, &p.HideFooter, &p.DiscoverHidden, &p.CreatedAt); err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (q *Queries) UpdateDiscoverHidden(ctx context.Context, id uuid.UUID, hidden bool) error {
	_, err := q.pool.Exec(ctx, `UPDATE profiles SET discover_hidden = $1 WHERE id = $2`, hidden, id)
	return err
}

func (q *Queries) UpdateProfileHideFooter(ctx context.Context, id uuid.UUID, hideFooter bool) error {
	_, err := q.pool.Exec(ctx, `UPDATE profiles SET hide_footer = $1 WHERE id = $2`, hideFooter, id)
	return err
}

func (q *Queries) UpdateProfileBranding(ctx context.Context, id uuid.UUID, accent, background, font *string, hideFooter bool) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE profiles SET accent_color = $1, background_color = $2, font_family = $3, hide_footer = $4 WHERE id = $5`,
		accent, background, font, hideFooter, id,
	)
	return err
}

func (q *Queries) UpdateProfileGenres(ctx context.Context, id uuid.UUID, genres []string) error {
	if genres == nil {
		genres = []string{}
	}
	_, err := q.pool.Exec(ctx, `UPDATE profiles SET genres = $1 WHERE id = $2`, genres, id)
	return err
}

// ── Blocks ─────────────────────────────────────────────────────────────────

func (q *Queries) CreateBlock(ctx context.Context, profileID uuid.UUID, blockType string, data json.RawMessage) (*models.Block, error) {
	b := &models.Block{}
	err := q.pool.QueryRow(ctx,
		`INSERT INTO blocks (profile_id, type, position, data)
		 VALUES ($1, $2, (SELECT COALESCE(MAX(position)+1, 0) FROM blocks WHERE profile_id = $1), $3)
		 RETURNING id, profile_id, type, position, data, created_at`,
		profileID, blockType, data,
	).Scan(&b.ID, &b.ProfileID, &b.Type, &b.Position, &b.Data, &b.CreatedAt)
	return b, err
}

func (q *Queries) GetBlocksByProfileID(ctx context.Context, profileID uuid.UUID) ([]*models.Block, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, profile_id, type, position, data, created_at
		 FROM blocks WHERE profile_id = $1 ORDER BY position ASC`,
		profileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []*models.Block
	for rows.Next() {
		b := &models.Block{}
		if err := rows.Scan(&b.ID, &b.ProfileID, &b.Type, &b.Position, &b.Data, &b.CreatedAt); err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

func (q *Queries) GetBlockByID(ctx context.Context, id uuid.UUID) (*models.Block, error) {
	b := &models.Block{}
	err := q.pool.QueryRow(ctx,
		`SELECT id, profile_id, type, position, data, created_at FROM blocks WHERE id = $1`,
		id,
	).Scan(&b.ID, &b.ProfileID, &b.Type, &b.Position, &b.Data, &b.CreatedAt)
	return b, err
}

func (q *Queries) UpdateBlockData(ctx context.Context, id uuid.UUID, data json.RawMessage) error {
	_, err := q.pool.Exec(ctx, `UPDATE blocks SET data = $1 WHERE id = $2`, data, id)
	return err
}

func (q *Queries) DeleteBlock(ctx context.Context, id uuid.UUID) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM blocks WHERE id = $1`, id)
	return err
}

func (q *Queries) UpdateBlockPositions(ctx context.Context, positions map[uuid.UUID]int) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for id, pos := range positions {
		if _, err := tx.Exec(ctx, `UPDATE blocks SET position = $1 WHERE id = $2`, pos, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ── Inquiries ──────────────────────────────────────────────────────────────

func (q *Queries) CreateInquiry(ctx context.Context, profileID uuid.UUID, name string, email, phone *string, message, ipHash string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO inquiries (profile_id, name, email, phone, message, ip_hash)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		profileID, name, email, phone, message, nullStr(ipHash),
	)
	return err
}

func (q *Queries) ListInquiriesByProfileID(ctx context.Context, profileID uuid.UUID) ([]*models.Inquiry, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, profile_id, name, email, phone, message, read_at, created_at
		 FROM inquiries
		 WHERE profile_id = $1
		 ORDER BY created_at DESC`,
		profileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var inquiries []*models.Inquiry
	for rows.Next() {
		inquiry := &models.Inquiry{}
		if err := rows.Scan(&inquiry.ID, &inquiry.ProfileID, &inquiry.Name, &inquiry.Email, &inquiry.Phone, &inquiry.Message, &inquiry.ReadAt, &inquiry.CreatedAt); err != nil {
			return nil, err
		}
		inquiries = append(inquiries, inquiry)
	}
	return inquiries, rows.Err()
}

func (q *Queries) CountUnreadInquiriesByProfileID(ctx context.Context, profileID uuid.UUID) (int, error) {
	var count int
	err := q.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM inquiries WHERE profile_id = $1 AND read_at IS NULL`,
		profileID,
	).Scan(&count)
	return count, err
}

func (q *Queries) MarkInquiriesReadByProfileID(ctx context.Context, profileID uuid.UUID) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE inquiries
		 SET read_at = NOW()
		 WHERE profile_id = $1 AND read_at IS NULL`,
		profileID,
	)
	return err
}

func (q *Queries) CountRecentInquiriesByIP(ctx context.Context, profileID uuid.UUID, ipHash string, windowMinutes int) (int, error) {
	var count int
	err := q.pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM inquiries
		 WHERE profile_id = $1
		   AND ip_hash = $2
		   AND created_at > NOW() - ($3 * INTERVAL '1 minute')`,
		profileID, ipHash, windowMinutes,
	).Scan(&count)
	return count, err
}

// ── Analytics ──────────────────────────────────────────────────────────────

func (q *Queries) RecordEvent(ctx context.Context, profileID uuid.UUID, blockID *uuid.UUID, eventType, ipHash, country, referrer, userAgent string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO analytics_events (profile_id, block_id, event_type, ip_hash, country, referrer, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		profileID, blockID, eventType, nullStr(ipHash), nullStr(country), nullStr(referrer), nullStr(userAgent),
	)
	return err
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (q *Queries) GetAnalyticsSummary(ctx context.Context, profileID uuid.UUID, days int) (*models.AnalyticsSummary, error) {
	summary := &models.AnalyticsSummary{Days: days}

	// Total views, clicks, unique visitors (all-time)
	err := q.pool.QueryRow(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE event_type = 'view') AS views,
			COUNT(*) FILTER (WHERE event_type = 'click') AS clicks,
			COUNT(DISTINCT ip_hash) FILTER (WHERE event_type = 'view') AS unique_visitors
		 FROM analytics_events WHERE profile_id = $1`,
		profileID,
	).Scan(&summary.TotalViews, &summary.TotalClicks, &summary.UniqueVisitors)
	if err != nil {
		return nil, fmt.Errorf("totals: %w", err)
	}

	// Views by day
	rows, err := q.pool.Query(ctx,
		`SELECT DATE(created_at)::text AS day, COUNT(*) AS views
		 FROM analytics_events
		 WHERE profile_id = $1 AND event_type = 'view' AND created_at > NOW() - ($2 * INTERVAL '1 day')
		 GROUP BY day ORDER BY day ASC`,
		profileID, days,
	)
	if err != nil {
		return nil, fmt.Errorf("views by day: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s models.DailyStat
		if err := rows.Scan(&s.Date, &s.Views); err != nil {
			return nil, err
		}
		summary.ViewsByDay = append(summary.ViewsByDay, s)
	}

	// Clicks by day
	clickDayRows, err := q.pool.Query(ctx,
		`SELECT DATE(created_at)::text AS day, COUNT(*) AS views
		 FROM analytics_events
		 WHERE profile_id = $1 AND event_type = 'click' AND created_at > NOW() - ($2 * INTERVAL '1 day')
		 GROUP BY day ORDER BY day ASC`,
		profileID, days,
	)
	if err != nil {
		return nil, fmt.Errorf("clicks by day: %w", err)
	}
	defer clickDayRows.Close()
	for clickDayRows.Next() {
		var s models.DailyStat
		if err := clickDayRows.Scan(&s.Date, &s.Views); err != nil {
			return nil, err
		}
		summary.ClicksByDay = append(summary.ClicksByDay, s)
	}

	// Clicks by block
	blockRows, err := q.pool.Query(ctx,
		`SELECT ae.block_id, b.type,
		  COALESCE(b.data->>'title', b.data->>'label', b.data->>'platform', b.data->>'url', b.type) AS label,
		  COUNT(*) AS clicks
		 FROM analytics_events ae
		 JOIN blocks b ON b.id = ae.block_id
		 WHERE ae.profile_id = $1 AND ae.event_type = 'click' AND ae.block_id IS NOT NULL
		 GROUP BY ae.block_id, b.type, b.data ORDER BY clicks DESC LIMIT 20`,
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("clicks by block: %w", err)
	}
	defer blockRows.Close()
	for blockRows.Next() {
		var s models.BlockStat
		if err := blockRows.Scan(&s.BlockID, &s.Type, &s.Label, &s.Clicks); err != nil {
			return nil, err
		}
		summary.ClicksByBlock = append(summary.ClicksByBlock, s)
	}

	// Top countries
	countryRows, err := q.pool.Query(ctx,
		`SELECT COALESCE(country, 'Unknown'), COUNT(*) AS cnt
		 FROM analytics_events WHERE profile_id = $1 AND event_type = 'view'
		 GROUP BY country ORDER BY cnt DESC LIMIT 10`,
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("countries: %w", err)
	}
	defer countryRows.Close()
	for countryRows.Next() {
		var s models.CountryStat
		if err := countryRows.Scan(&s.Country, &s.Count); err != nil {
			return nil, err
		}
		summary.TopCountries = append(summary.TopCountries, s)
	}

	// Top referrers
	refRows, err := q.pool.Query(ctx,
		`SELECT COALESCE(referrer, 'Direct'), COUNT(*) AS cnt
		 FROM analytics_events WHERE profile_id = $1 AND event_type = 'view'
		 GROUP BY referrer ORDER BY cnt DESC LIMIT 10`,
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("referrers: %w", err)
	}
	defer refRows.Close()
	for refRows.Next() {
		var s models.ReferrerStat
		if err := refRows.Scan(&s.Referrer, &s.Count); err != nil {
			return nil, err
		}
		summary.TopReferrers = append(summary.TopReferrers, s)
	}

	return summary, nil
}

func (q *Queries) PurgeOldAnalyticsEvents(ctx context.Context) (int64, error) {
	tag, err := q.pool.Exec(ctx,
		`DELETE FROM analytics_events WHERE created_at < NOW() - INTERVAL '90 days'`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ── Subscriptions ──────────────────────────────────────────────────────────

func (q *Queries) UpsertSubscription(ctx context.Context, userID uuid.UUID, stripeSubID, status string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO subscriptions (user_id, stripe_subscription_id, status)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (stripe_subscription_id)
		 DO UPDATE SET status = EXCLUDED.status`,
		userID, stripeSubID, status,
	)
	return err
}

func (q *Queries) GetUserByStripeCustomerID(ctx context.Context, customerID string) (*models.User, error) {
	u := &models.User{}
	err := q.pool.QueryRow(ctx,
		`SELECT id, email, tier, stripe_customer_id, is_admin, created_at FROM users WHERE stripe_customer_id = $1`,
		customerID,
	).Scan(&u.ID, &u.Email, &u.Tier, &u.StripeCustomerID, &u.IsAdmin, &u.CreatedAt)
	return u, err
}
