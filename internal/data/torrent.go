package data

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"transmission-proxy/conf"
	"transmission-proxy/internal/domain"
	"transmission-proxy/internal/errors"

	"github.com/eko/gocache/lib/v4/store"
	"github.com/go-kratos/kratos/v2/encoding"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/hekmon/transmissionrpc/v3"
	col "github.com/noxiouz/golang-generics-util/collection"
)

var (
	CacheNotFoundErr = store.NotFound{}
)

const (
	PropertiesFileName = "properties.json"
)

// HistoricalStatistics 历史统计数据（写盘统计）
type HistoricalStatistics struct {
	TotalDownloaded int64 `json:"total_downloaded"` // 所有时间下载总量（字节）
	TotalUploaded   int64 `json:"total_uploaded"`   // 所有时间上传总量（字节）
}

type torrentDao struct {
	infra *Infra
	log   *log.Helper
}

// NewTorrentDao .
func NewTorrentDao(infra *Infra, logger log.Logger) (domain.TorrentRepo, error) {

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

func (d *torrentDao) GetHistoricalStatistics() (statistics domain.HistoricalStatistics, err error) {
	path := filepath.Join(conf.FlagConf, PropertiesFileName)

	hs := HistoricalStatistics{
		TotalDownloaded: 0,
		TotalUploaded:   0,
	}
	// 检查文件是否存在
	if _, err = os.Stat(path); os.IsNotExist(err) {
		err = d.SaveHistoricalStatistics(statistics)
		return
	}

	// 读取文件内容到
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	err = encoding.GetCodec("json").Unmarshal(data, &hs)
	statistics.TotalDownloaded = hs.TotalUploaded
	statistics.TotalUploaded = hs.TotalUploaded
	return
}

// SaveHistoricalStatistics 保存历史统计
func (d *torrentDao) SaveHistoricalStatistics(statistics domain.HistoricalStatistics) (err error) {
	hs := HistoricalStatistics{
		TotalDownloaded: statistics.TotalDownloaded,
		TotalUploaded:   statistics.TotalUploaded,
	}

	path := filepath.Join(conf.FlagConf, PropertiesFileName)
	json, err := encoding.GetCodec("json").Marshal(&hs)
	if err != nil {
		return
	}
	// 打开文件以覆盖写入
	file, err := os.Create(path)
	if err != nil {
		return
	}
	defer func(file *os.File) {
		err = file.Close()
	}(file)
	_, err = file.Write(json)
	return
}
