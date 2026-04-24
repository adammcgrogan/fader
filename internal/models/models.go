package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID `db:"id"`
	Email            string    `db:"email"`
	Tier             string    `db:"tier"`
	StripeCustomerID *string   `db:"stripe_customer_id"`
	IsAdmin          bool      `db:"is_admin"`
	CreatedAt        time.Time `db:"created_at"`
}

func (u *User) IsPro() bool {
	return u.Tier == "pro"
}

type Profile struct {
	ID              uuid.UUID `db:"id"`
	UserID          uuid.UUID `db:"user_id"`
	Handle          string    `db:"handle"`
	DisplayName     string    `db:"display_name"`
	AvatarURL       *string   `db:"avatar_url"`
	Template        string    `db:"template"`
	Bio             *string   `db:"bio"`
	Genres          []string  `db:"genres"`
	AccentColor     *string   `db:"accent_color"`
	BackgroundColor *string   `db:"background_color"`
	FontFamily      *string   `db:"font_family"`
	HideFooter      bool      `db:"hide_footer"`
	DiscoverHidden  bool      `db:"discover_hidden"`
	CreatedAt       time.Time `db:"created_at"`
}

type Block struct {
	ID        uuid.UUID       `db:"id"`
	ProfileID uuid.UUID       `db:"profile_id"`
	Type      string          `db:"type"`
	Position  int             `db:"position"`
	Data      json.RawMessage `db:"data"`
	CreatedAt time.Time       `db:"created_at"`
}

type AnalyticsEvent struct {
	ID        int64      `db:"id"`
	ProfileID uuid.UUID  `db:"profile_id"`
	BlockID   *uuid.UUID `db:"block_id"`
	EventType string     `db:"event_type"`
	IPHash    *string    `db:"ip_hash"`
	Country   *string    `db:"country"`
	Referrer  *string    `db:"referrer"`
	UserAgent *string    `db:"user_agent"`
	CreatedAt time.Time  `db:"created_at"`
}

type Subscription struct {
	ID                   uuid.UUID  `db:"id"`
	UserID               uuid.UUID  `db:"user_id"`
	StripeSubscriptionID string     `db:"stripe_subscription_id"`
	Status               string     `db:"status"`
	CurrentPeriodEnd     *time.Time `db:"current_period_end"`
	CancelAtPeriodEnd    bool       `db:"cancel_at_period_end"`
	CreatedAt            time.Time  `db:"created_at"`
}

// Block data types (unmarshalled from JSONB)

type SocialData struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

type MusicLinkData struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Platform string `json:"platform"`
}

type GigData struct {
	Date      string `json:"date"`
	Venue     string `json:"venue"`
	Location  string `json:"location"`
	TicketURL string `json:"ticket_url,omitempty"`
}

type BioData struct {
	Text string `json:"text"`
}

type CustomLinkData struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type ImageData struct {
	URL     string `json:"url"`
	Caption string `json:"caption,omitempty"`
}

type VideoLinkData struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type AudioEmbedData struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

type RALinkData struct {
	Username string `json:"username"`
}

type ResidencyData struct {
	Venue     string `json:"venue"`
	Location  string `json:"location"`
	Frequency string `json:"frequency"`
	Since     string `json:"since,omitempty"`
}

type BookMeData struct {
	Label      string `json:"label"`
	IntroText  string `json:"intro_text"`
	SubmitText string `json:"submit_text"`
}

type Inquiry struct {
	ID        int64      `db:"id"`
	ProfileID uuid.UUID  `db:"profile_id"`
	Name      string     `db:"name"`
	Email     *string    `db:"email"`
	Phone     *string    `db:"phone"`
	Message   string     `db:"message"`
	ReadAt    *time.Time `db:"read_at"`
	CreatedAt time.Time  `db:"created_at"`
}

// AnalyticsSummary is used for the analytics dashboard
type AnalyticsSummary struct {
	TotalViews     int
	TotalClicks    int
	UniqueVisitors int
	Days           int
	ViewsByDay     []DailyStat
	ClicksByDay    []DailyStat
	ClicksByBlock  []BlockStat
	TopCountries   []CountryStat
	TopReferrers   []ReferrerStat
}

type DailyStat struct {
	Date  string
	Views int
}

type BlockStat struct {
	BlockID uuid.UUID
	Type    string
	Label   string
	Clicks  int
}

type CountryStat struct {
	Country string
	Count   int
}

type ReferrerStat struct {
	Referrer string
	Count    int
}
