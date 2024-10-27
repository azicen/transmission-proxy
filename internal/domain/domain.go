package domain

import (
	// "github.com/go-kratos/kratos/v2/errors"
	"context"
	"github.com/google/wire"
	col "github.com/noxiouz/golang-generics-util/collection"
	"time"
)

var (
// ErrUserNotFound is user not found.
// ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewAppUsecase, NewTorrentUsecase)

// BanIPRepo .
type BanIPRepo interface {
	// GetBannedIPV4Status 获取封禁ipv4状态
	GetBannedIPV4Status(ctx context.Context, ips []string) (map[string]col.Option[*time.Time], error)

	// GetBannedIPV6Status 获取封禁ipv6状态
	GetBannedIPV6Status(ctx context.Context, ips []string) (map[string]col.Option[*time.Time], error)

	// BanIPV4 封禁ipv4
	BanIPV4(ctx context.Context, ips []string) error

	// BanIPV6 封禁ipv6
	BanIPV6(ctx context.Context, ips []string) error

	// UnbanIPV4 解禁ipv4
	UnbanIPV4(ctx context.Context, ips []string) error

	// UnbanIPV6 解禁ipv6
	UnbanIPV6(ctx context.Context, ips []string) error

	// UpBanIPV4List 更新ipv4封禁列表
	UpBanIPV4List(ctx context.Context, ips []string) error

	// UpBanIPV6List 更新ipv6封禁列表
	UpBanIPV6List(ctx context.Context, ips []string) error

	// ClearBanList 清空Ban列表
	ClearBanList(ctx context.Context) error
}
