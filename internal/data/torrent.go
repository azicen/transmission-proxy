package data

import (
	"context"
	"strconv"

	"transmission-proxy/internal/domain"
	"transmission-proxy/internal/errors"

	"github.com/eko/gocache/lib/v4/store"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/hekmon/transmissionrpc/v3"
	col "github.com/noxiouz/golang-generics-util/collection"
)

var (
	CacheNotFoundErr = store.NotFound{}
)

var schema = `
CREATE TABLE IF NOT EXISTS properties (
	id INTEGER PRIMARY KEY AUTOINCREMENT, -- 自增主键
    key VARCHAR(255) NOT NULL,            -- 字符串，最大长度255
    value TEXT
);
INSERT OR IGNORE INTO properties (key, value) VALUES ('TotalDownloaded', '0');
INSERT OR IGNORE INTO properties (key, value) VALUES ('TotalUploaded', '0');
`

const (
	TotalDownloadedKey = "TotalDownloaded"
	TotalUploadedKey   = "TotalUploaded"
	QueryProperties    = `SELECT value FROM properties WHERE key=$1`
	UpdateProperties   = `UPDATE properties SET value=$2 WHERE key=$1`
)

type torrentDao struct {
	infra *Infra
	log   *log.Helper
}

// NewTorrentDao .
func NewTorrentDao(infra *Infra, logger log.Logger) (domain.TorrentRepo, error) {

	_, err := infra.DB.Exec(schema)
	if err != nil {
		return nil, err
	}

	return &torrentDao{
		infra: infra,
		log:   log.NewHelper(logger),
	}, nil
}

func (d *torrentDao) GetTorrent(ctx context.Context, hash string) (col.Option[transmissionrpc.Torrent], error) {
	torrents, err := d.infra.TR.TorrentGetAllForHashes(ctx, []string{hash})
	if err != nil {
		return nil, err
	}
	if len(torrents) == 0 {
		return nil, errors.ResourceNotExist("Torrent hash was not found")
	}
	trt := torrents[0]
	return col.Some(trt), nil
}

func (d *torrentDao) GetTorrentAll(ctx context.Context) (col.Option[[]transmissionrpc.Torrent], error) {
	torrents, err := d.infra.TR.TorrentGetAll(ctx)
	if err != nil {
		return nil, err
	}
	if len(torrents) == 0 {
		return col.None[[]transmissionrpc.Torrent](), nil
	}
	return col.Some(torrents), nil
}

func (d *torrentDao) GetPeer(ctx context.Context, key domain.PeerKey) (col.Option[*domain.Peer], error) {
	peerInfo, err := d.infra.Cache.Get(ctx, key)
	if err != nil && CacheNotFoundErr.Is(err) {
		return col.None[*domain.Peer](), nil
	}
	if err != nil {
		return col.None[*domain.Peer](), err
	}
	return col.Some(peerInfo), nil
}

func (d *torrentDao) SetPeer(ctx context.Context, key domain.PeerKey, peer *domain.Peer) error {
	err := d.infra.Cache.Set(ctx, key, peer)
	if err != nil {
		return err
	}
	return nil
}

func (d *torrentDao) GetStateRefreshInterval() int64 {
	return d.infra.stateRefreshInterval
}

func (d *torrentDao) GetHistoricalStatistics() (domain.HistoricalStatistics, error) {

	var totalDownloadedValue string
	err := d.infra.DB.Get(&totalDownloadedValue, QueryProperties, TotalDownloadedKey)
	if err != nil {
		return domain.HistoricalStatistics{}, err
	}
	totalDownloaded, err := strconv.Atoi(totalDownloadedValue)
	if err != nil {
		return domain.HistoricalStatistics{}, err
	}
	var totalUploadedValue string
	err = d.infra.DB.Get(&totalUploadedValue, QueryProperties, TotalUploadedKey)
	if err != nil {
		return domain.HistoricalStatistics{}, err
	}
	totalUploaded, err := strconv.Atoi(totalUploadedValue)
	if err != nil {
		return domain.HistoricalStatistics{}, err
	}

	return domain.HistoricalStatistics{
		TotalDownloaded: int64(totalDownloaded),
		TotalUploaded:   int64(totalUploaded),
	}, nil
}

// SaveHistoricalStatistics 保存历史统计
func (d *torrentDao) SaveHistoricalStatistics(statistics domain.HistoricalStatistics) (err error) {
	_, err = d.infra.DB.Exec(UpdateProperties, statistics.TotalDownloaded, TotalDownloadedKey)
	if err != nil {
		return
	}
	_, err = d.infra.DB.Exec(UpdateProperties, statistics.TotalUploaded, TotalUploadedKey)
	if err != nil {
		return
	}

	return
}
