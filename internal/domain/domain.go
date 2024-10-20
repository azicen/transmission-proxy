package domain

import (
	// "github.com/go-kratos/kratos/v2/errors"
	"github.com/google/wire"
)

var (
// ErrUserNotFound is user not found.
// ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewAppUsecase, NewTorrentUsecase)
