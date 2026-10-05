package domain

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

// UserID identifies an account.
type UserID int64

// User is an account. Market data is shared; a user owns only their profile,
// watchlists, LLM credentials and AI reports.
type User struct {
	ID        UserID
	Email     string
	CreatedAt time.Time
	// Profile is empty until the risk questionnaire is answered.
	Profile        RiskProfile
	ProfileAnswers SuitabilityAnswers
	ProfileAt      time.Time
}

// NormalizeEmail lowercases and validates an email address, so one person
// cannot hold two accounts that differ only in case.
func NormalizeEmail(s string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(s))
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e || len(e) > 254 {
		return "", fmt.Errorf("%w: email address is not valid", ErrInvalid)
	}
	return e, nil
}

// Password length limits, counted in characters. Long passphrases are
// welcome; the cap only bounds hashing work.
const (
	MinPasswordLen = 10
	MaxPasswordLen = 128
)

// ValidatePassword enforces the length rule. Composition rules are left
// out on purpose (NIST SP 800-63B): length is what makes a password strong.
func ValidatePassword(pw string) error {
	n := utf8.RuneCountInString(pw)
	if n < MinPasswordLen || n > MaxPasswordLen {
		return fmt.Errorf("%w: password must be %d to %d characters", ErrInvalid, MinPasswordLen, MaxPasswordLen)
	}
	return nil
}

// Session is a signed-in browser. Only a hash of its token is stored, so a
// leaked sessions table cannot be replayed.
type Session struct {
	TokenHash []byte
	UserID    UserID
	CreatedAt time.Time
	ExpiresAt time.Time
}

// WatchlistID identifies a watchlist.
type WatchlistID int64

// Watchlist is a user's named list of assets.
type Watchlist struct {
	ID     WatchlistID
	UserID UserID
	Name   string
	Assets []AssetKey
}

// MaxWatchlistName bounds a watchlist name, in characters.
const MaxWatchlistName = 60

// NormalizeWatchlistName trims and validates a watchlist name.
func NormalizeWatchlistName(s string) (string, error) {
	n := strings.TrimSpace(s)
	if n == "" || utf8.RuneCountInString(n) > MaxWatchlistName {
		return "", fmt.Errorf("%w: watchlist name must be 1 to %d characters", ErrInvalid, MaxWatchlistName)
	}
	return n, nil
}
