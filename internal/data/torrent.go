package data

import (
	"bufio"
	"context"
	"io"
	"net/http"
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
	id    INTEGER PRIMARY KEY AUTOINCREMENT,
    key   VARCHAR UNIQUE NOT NULL,
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

// GetResponseLine 安行获取指定URL内容
func (d *torrentDao) GetResponseLine(_ context.Context, trackerListURL string) (lines []string, err error) {
	lines = make([]string, 0, 128)
	response, err := http.Get(trackerListURL)
	if err != nil {
		return
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(response.Body)

	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}

	err = scanner.Err()
	return
}

// UpTracker 更新Tracker
func (d *torrentDao) UpTracker(ctx context.Context, ids []int64, trackers []string) (err error) {
	// 添加tracker
	err = d.infra.TR.TorrentSet(ctx, transmissionrpc.TorrentSetPayload{
		IDs:         ids,
		TrackerList: trackers,
	})
	return
}

// AddTorrent 添加种子
func (d *torrentDao) AddTorrent(ctx context.Context, torrents []*domain.Torrent) (err error) {
	for _, torrent := range torrents {
		trt := transmissionrpc.TorrentAddPayload{
			Filename: &torrent.URL,
			Paused:   &torrent.Paused,
		}
		if torrent.Path.HasValue() {
			path := torrent.Path.Value()
			trt.DownloadDir = &path
		}
		if torrent.Labels.HasValue() {
			trt.Labels = torrent.Labels.Value()
		}
		if torrent.Cookie.HasValue() {
			cookies := torrent.Cookie.Value()
			trt.Cookies = &cookies
		}

		t, err := d.infra.TR.TorrentAdd(ctx, trt)
		if err != nil {
			d.log.Errorf("添加种子时出现错误 torrent=%s err=%v", torrent.URL, err)
		}

		// 添加tracker
		err = d.infra.TR.TorrentSet(ctx, transmissionrpc.TorrentSetPayload{
			IDs:         []int64{*t.ID},
			TrackerList: torrent.Trackers,
		})
		if err != nil {
			d.log.Errorf("更新种子Tracker时出现错误 torrent=%s err=%v", torrent.URL, err)
		}
	}
	return
}

// GetTorrent 获取种子
func (d *torrentDao) GetTorrent(ctx context.Context, hash string) (col.Option[transmissionrpc.Torrent], error) {
	torrents, err := d.infra.TR.TorrentGetAllForHashes(ctx, []string{hash})
	if err != nil {
		return nil, err
	}
	if len(torrents) == 0 {
		return nil, errors.ResourceNotExist("未找到Torrent")
	}
	trt := torrents[0]
	return col.Some(trt), nil
}

// GetTorrentAll 获取所有种子
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

// GetPeer 获取Peer
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

// SetPeer 设置Peer
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
