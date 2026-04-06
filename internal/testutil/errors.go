package testutil

import "errors"

// ErrNotFound is returned by mock repositories when an entity is not found.
var ErrNotFound = errors.New("not found")
