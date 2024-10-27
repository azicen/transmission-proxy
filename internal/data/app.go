package data

import (
	"context"

	"transmission-proxy/internal/domain"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/hekmon/transmissionrpc/v3"
)

var preferencesFields = []string{
	"start-added-torrents",
	"rename-partial-files",
	"download-dir",
	"incomplete-dir-enabled",
	"incomplete-dir",
	"script-torrent-done-enabled",
	"script-torrent-done-filename",
	"download-queue-enabled",
	"download-queue-size",
	"seed-queue-enabled",
	"seed-queue-size",
	"seedRatioLimited",
	"seedRatioLimit",
	"peer-port",
	"peer-port-random-on-start",
	"alt-speed-down",
	"alt-speed-enabled",
	"alt-speed-up",
	"peer-limit-global",
	"peer-limit-per-torrent",
	"version",
}

type appDao struct {
	infra *Infra
	log   *log.Helper
}

// NewAppDao .
func NewAppDao(infra *Infra, logger log.Logger) domain.AppRepo {
	return &appDao{
		infra: infra,
		log:   log.NewHelper(logger),
	}
}

// GetPreferences 获取首选项
func (d *appDao) GetPreferences(ctx context.Context) (transmissionrpc.SessionArguments, error) {
	pre, err := d.infra.TR.SessionArgumentsGet(ctx, preferencesFields)
	if err != nil {
		return transmissionrpc.SessionArguments{}, err
	}
	return pre, nil
}

// SetPreferences 设置首选项
func (d *appDao) SetPreferences(ctx context.Context, pre transmissionrpc.SessionArguments) error {
	err := d.infra.TR.SessionArgumentsSet(ctx, pre)
	if err != nil {
		return err
	}
	return nil
}
