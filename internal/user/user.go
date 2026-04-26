// Package user provides host user detection for convocate.
package user

import (
	"fmt"
	"os/user"
	"strconv"
)

// Info holds UID and GID information for a user.
type Info struct {
	UID      int
	GID      int
	Username string
	HomeDir  string
}

// LookupFunc abstracts user lookup for testing.
type LookupFunc func(username string) (*user.User, error)

// DefaultLookup is the default user lookup using os/user.
func DefaultLookup(username string) (*user.User, error) {
	return user.Lookup(username)
}

// Lookup retrieves user information by username.
func Lookup(username string) (Info, error) {
	return LookupWith(username, DefaultLookup)
}

// LookupWith retrieves user information using the provided lookup function.
func LookupWith(username string, lookup LookupFunc) (Info, error) {
	u, err := lookup(username)
	if err != nil {
		return Info{}, fmt.Errorf("user %q not found: %w", username, err)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return Info{}, fmt.Errorf("invalid UID %q for user %q: %w", u.Uid, username, err)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return Info{}, fmt.Errorf("invalid GID %q for user %q: %w", u.Gid, username, err)
	}

	return Info{
		UID:      uid,
		GID:      gid,
		Username: u.Username,
		HomeDir:  u.HomeDir,
	}, nil
}
