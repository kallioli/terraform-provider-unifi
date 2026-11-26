package unifi

import (
	"errors"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

// alreadyDeleted reports whether a failed delete can count as a success
// because the object is no longer on the controller, so that a destroy does
// not fail (and need a manual `terraform state rm`) when the object was
// removed out of band after the last refresh, or with -refresh=false.
//
// go-unifi turns an HTTP 404, which the v2 endpoints answer for an unknown
// ID, into a *unifi.NotFoundError. What the legacy REST endpoints and the
// sitemgr command answer for an unknown ID is not documented, and any non-404
// error is an APIError. Rather than guess at error codes, a failure that is
// not a NotFoundError is checked with lookup, which fetches the object again:
// the delete counts as done only if that lookup itself returns a
// NotFoundError. Any other outcome keeps the original error.
func alreadyDeleted(err error, lookup func() error) bool {
	var notFound *unifi.NotFoundError
	if errors.As(err, &notFound) {
		return true
	}
	return errors.As(lookup(), &notFound)
}
