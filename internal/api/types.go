package api

// Error is a failed API response.
type Error struct {
	Status  int    `json:"-"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Code != "" {
		return e.Message + " (" + e.Code + ")"
	}
	if e.Message != "" {
		return e.Message
	}
	return "request failed"
}

// User is a public profile.
type User struct {
	ID             string            `json:"id"`
	Username       string            `json:"username"`
	Name           string            `json:"name"`
	AvatarURL      string            `json:"avatar_url"`
	Bio            string            `json:"bio"`
	Company        string            `json:"company"`
	Location       string            `json:"location"`
	Website        string            `json:"website"`
	GitHubURL      string            `json:"github_url"`
	HTMLURL        string            `json:"html_url"`
	CreatedAt      string            `json:"created_at"`
	CommitsCount   int               `json:"commits_count"`
	FollowersCount int               `json:"followers_count"`
	FollowingCount int               `json:"following_count"`
	Lane           any               `json:"lane"`
	IsFollowed     *bool             `json:"is_followed"`
	FollowsYou     *bool             `json:"follows_you"`
	IsMuting       *bool             `json:"is_muting"`
	IsBlocking     *bool             `json:"is_blocking"`
	Activity       *UserActivity     `json:"activity"`
	Streaks        *UserStreaks      `json:"streaks"`
	Achievements   *UserAchievements `json:"achievements"`
}

// UserActivity is the profile heatmap (usually the last 7 days).
type UserActivity struct {
	Scale   string               `json:"scale"`
	Buckets []UserActivityBucket `json:"buckets"`
}

// UserActivityBucket is one day (or week) of commit counts.
type UserActivityBucket struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// UserStreaks is consecutive days with a commit.
type UserStreaks struct {
	Longest int `json:"longest"`
	Current int `json:"current"`
}

// UserAchievements is the Unix-rank progression the web shows on a profile.
type UserAchievements struct {
	Level         int               `json:"level"`
	Rank          string            `json:"rank"`
	XP            int               `json:"xp"`
	NextLevelXP   int               `json:"next_level_xp"`
	LevelProgress int               `json:"level_progress"`
	XPBreakdown   map[string]XPPart `json:"xp_breakdown"`
	Ranks         []UserRank        `json:"ranks"`
	Badges        []UserBadge       `json:"badges"`
}

// XPPart is one XP source (commits, streak, stars, …).
type XPPart struct {
	Count      int `json:"count"`
	Multiplier int `json:"multiplier"`
	XP         int `json:"xp"`
}

// UserRank is one Unix rank on the XP ladder.
type UserRank struct {
	Rank        string `json:"rank"`
	Level       int    `json:"level"`
	MinXP       int    `json:"min_xp"`
	Description string `json:"description"`
	Achieved    bool   `json:"achieved"`
}

// UserBadge is a named achievement, locked or unlocked.
type UserBadge struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Unlocked    bool   `json:"unlocked"`
	Progress    int    `json:"progress"`
	Target      int    `json:"target"`
}

// Me is the signed-in user plus private fields.
type Me struct {
	User
	PasteisMode      bool   `json:"pasteis_mode"`
	PasteisRemaining int    `json:"pasteis_remaining"`
	PasteisResetsAt  string `json:"pasteis_resets_at"`
	PasteisReceived  int    `json:"pasteis_received"`
	PreferredLane    any    `json:"preferred_lane"`
	AutomaticLane    any    `json:"automatic_lane"`
	SponsorsURL      string `json:"sponsors_url"`
	GitHubSyncedAt   string `json:"github_synced_at"`
}

// MeResponse is GET /api/v1/me.
type MeResponse struct {
	Data                     Me  `json:"data"`
	UnreadNotificationsCount int `json:"unread_notifications_count"`
}

// Commit is a post (or reply) on the log.
type Commit struct {
	ID             string        `json:"id"`
	ShortHash      string        `json:"short_hash"`
	Body           string        `json:"body"`
	ParentID       *string       `json:"parent_id"`
	HTMLURL        string        `json:"html_url"`
	CreatedAt      string        `json:"created_at"`
	StarsCount     int           `json:"stars_count"`
	RepliesCount   int           `json:"replies_count"`
	Author         *User         `json:"author"`
	Status         string        `json:"status"`
	Starred        bool          `json:"starred"`
	IsSaved        bool          `json:"is_saved"`
	PasteisCount   int           `json:"pasteis_count"`
	DeletedAt      *string       `json:"deleted_at"`
	QuotedCommitID *string       `json:"quoted_commit_id"`
	QuotedCommit   *Commit       `json:"quoted_commit"`
	ReplyPreview   *ReplyPreview `json:"reply_preview"`
	Images         []CommitImage `json:"images"`
}

// CommitImage is a picture attached to a commit. The TUI only shows a count.
type CommitImage struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ReplyPreview is a nested reply page on a feed item.
type ReplyPreview struct {
	Data       []Commit `json:"data"`
	NextCursor *string  `json:"next_cursor"`
}

// Page is a cursor-paginated list. NextCursor is opaque; pass it back as-is.
type Page[T any] struct {
	Data  []T `json:"data"`
	Links struct {
		Next *string `json:"next"`
		Prev *string `json:"prev"`
	} `json:"links"`
	Meta struct {
		NextCursor *string `json:"next_cursor"`
		PrevCursor *string `json:"prev_cursor"`
	} `json:"meta"`
}

// LogPage is a feed plus "new since" metadata.
type LogPage struct {
	Page[Commit]
	Log             string   `json:"log"`
	IsRanked        bool     `json:"is_ranked"`
	NewCommitsCount int      `json:"new_commits_count"`
	Suggestions     []Commit `json:"suggestions"`
}

// CommitPage is one commit with its ancestor chain.
type CommitPage struct {
	Data      Commit   `json:"data"`
	Ancestors []Commit `json:"ancestors"`
}

// Notification is one inbox row.
type Notification struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	SubjectID   string  `json:"subject_id"`
	Reason      *string `json:"reason"`
	ReadAt      *string `json:"read_at"`
	CreatedAt   string  `json:"created_at"`
	Actor       *User   `json:"actor"`
	GroupID     string  `json:"group_id"`
	EventCount  int     `json:"event_count"`
	ActorsCount int     `json:"actors_count"`
	UnreadCount int     `json:"unread_count"`
	Commit      *Commit `json:"commit"`
}

// NotificationPage is the inbox.
type NotificationPage struct {
	Page[Notification]
	ReadAt string `json:"read_at"`
}

// CreateCommit is the POST /api/v1/commits body.
type CreateCommit struct {
	Body           string  `json:"body"`
	ParentID       *string `json:"parent_id,omitempty"`
	QuotedCommitID *string `json:"quoted_commit_id,omitempty"`
}
