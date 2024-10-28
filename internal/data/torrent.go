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
	// 很怪，如果没有这个空行，tr将永远不会刷新tracker服务器
	tmpTrackers := make([]string, 0, len(trackers)*2)
	for _, tracker := range trackers {
		tmpTrackers = append(tmpTrackers, tracker, "")
	}
	// 添加tracker
	data := transmissionrpc.TorrentSetPayload{
		IDs:         ids,
		TrackerList: tmpTrackers,
	}
	err = d.infra.TR.TorrentSet(ctx, data)
	return
}

// AddTorrent 添加种子
func (d *torrentDao) AddTorrent(ctx context.Context, torrents []*domain.DownloadTorrent, trackers []string) (err error) {
	ids := make([]int64, len(torrents))
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

		ids = append(ids, *t.ID)
	}
	// 添加tracker
	if len(ids) > 0 {
		err = d.UpTracker(ctx, ids, trackers)
		if err != nil {
			d.log.Errorf("更新种子Tracker时出现错误 err=%v", err)
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
	peerInfo, err := d.infra.PeerCache.Get(ctx, key)
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
	err := d.infra.PeerCache.Set(ctx, key, peer)
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

// CacheTmpTorrentFile 缓存临时种子文件
func (d *torrentDao) CacheTmpTorrentFile(ctx context.Context, filename string, data []byte) (err error) {
	err = d.infra.TmpTorrentFileData.Set(ctx, filename, data)
	return
}

// GetTmpTorrentFile 获取缓存的临时种子文件
func (d *torrentDao) GetTmpTorrentFile(ctx context.Context, filename string) (col.Option[[]byte], error) {
	data, err := d.infra.TmpTorrentFileData.Get(ctx, filename)
	if err != nil && CacheNotFoundErr.Is(err) {
		return col.None[[]byte](), nil
	}
	if err != nil {
		return col.None[[]byte](), err
	}
	return col.Some(data), nil
}
