// Package parser provides directory listing parsers for remote HTTP file servers.
package parser

import (
	"context"
	"errors"
	"net/http"

	"github.com/steadyfall/fspeek/cache"
)

// ErrNoMatch is returned when a lister does not recognize the server's response format.
var ErrNoMatch = errors.New("lister does not recognize this format")

// DirectoryLister lists the contents of a remote directory URL.
type DirectoryLister interface {
	List(ctx context.Context, url string, client *http.Client) ([]cache.Entry, error)
}

// cascadeLister implements DirectoryLister by trying multiple listers in order.
type cascadeLister struct {
	listers []DirectoryLister
}

// NewCascade returns a DirectoryLister that tries each provided lister in order,
// returning the result of the first one that succeeds. If a lister returns
// ErrNoMatch it is skipped. If all fail, the last non-ErrNoMatch error (or
// ErrNoMatch if all returned ErrNoMatch) is returned.
func NewCascade(listers ...DirectoryLister) DirectoryLister {
	return cascadeLister{listers: listers}
}

func (c cascadeLister) List(ctx context.Context, url string, client *http.Client) ([]cache.Entry, error) {
	return Cascade(ctx, url, client, c.listers)
}

// Cascade tries each lister in order and returns the result of the first one
// that succeeds (returns nil error). If a lister returns ErrNoMatch it is
// skipped. If all listers fail, the last non-ErrNoMatch error (or ErrNoMatch
// if all returned ErrNoMatch) is returned.
func Cascade(ctx context.Context, url string, client *http.Client, listers []DirectoryLister) ([]cache.Entry, error) {
	var lastErr error
	for _, l := range listers {
		entries, err := l.List(ctx, url, client)
		if err == nil {
			return entries, nil
		}
		if errors.Is(err, ErrNoMatch) {
			continue
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNoMatch
}
