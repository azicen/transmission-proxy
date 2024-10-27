package domain

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	pb "transmission-proxy/api/v2"
	"transmission-proxy/conf"
	"transmission-proxy/internal/errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/hekmon/cunits/v2"
	"github.com/hekmon/transmissionrpc/v3"
	col "github.com/noxiouz/golang-generics-util/collection"
)

const torrentFileSuffix = ".torrent"
const trackerMaxSize = 64 // tracker 列表过长可能会更新失败

type PeerKey struct {
	Hash string
	IP   string
	Port int32
}

func (p PeerKey) String() string {
	return fmt.Sprintf("%s:%s:%d", p.Hash, p.IP, p.Port)
}

// Peer 种子节点列表中的节点元数据
type Peer struct {
	IP            string  // IP 地址
	Port          uint16  // 端口号
	Connection    string  // 连接类型
	PeerIdClient  string  // 客户端的 Peer ID
	ClientName    string  // 客户端名称
	Progress      float32 // 进度（0-100%）
	DownloadSpeed int64   // 下载速度（字节/秒）
	Downloaded    int64   // 已下载数据量（字节） 代理计算
	UploadSpeed   int64   // 上传速度（字节/秒）
	Uploaded      int64   // 已上传数据量（字节） 代理计算
	Flags         string  // 标志信息
	//Country     string  // 国家
	//CountryCode string  // 国家代码

	// 暂停跟踪计数器
	// 当BanIP时创建计数器，暂时停止展示该Peer。
	// 避免TR接口来不及停止导致PBH进行全量BanIPList更新。
	PauseTrackCounter col.Option[int8]
	// 活跃
	IsActive bool
}

type DownloadTorrent struct {
	URL      string               // 种子url
	Path     col.Option[string]   // 种子保存路径
	Labels   col.Option[[]string] // 种子tag
	Cookie   col.Option[string]   // 发送 Cookie 以下载 .torrent 文件
	Paused   bool                 // 在暂停状态下添加种子
	Trackers []string             // 添加到种子的Tracker列表
}

// Torrent 种子
type Torrent struct {
	Hash                   string                 // 种子的哈希值
	Name                   string                 // 种子名称
	URL                    string                 // 种子url
	Path                   string                 // 种子保存路径 （多文件种子为根目录路径，单文件种子为文件路径）
	TorrentPath            string                 // 种子数据存储的路径
	Labels                 col.Option[[]string]   // 种子tag
	CreationDate           *time.Time             // 种子创建的时间
	AddedDate              *time.Time             // 添加种子的时间
	DoneDate               col.Option[*time.Time] // 种子完成下载的时间
	Creator                string                 // 种子创建者
	Comment                string                 // 种子评论
	IsPrivate              bool                   // 如果种子来自私有 Tracker，则为 true
	SizeWhenDone           int64                  // 选择下载文件的总大小（Byte）
	TotalSize              int64                  // 种子的总大小（包括未选择的文件，Byte）
	HaveValidSize          int64                  // 有效的文件大小
	PieceSize              col.Option[int64]      // 种子片段大小（字节）
	TotalWasted            int64                  // 种子浪费的总数据量（字节）
	TotalDownloaded        int64                  // 种子下载的总数据量（字节）
	TotalDownloadedSession int64                  // 本次会话下载的总数据（字节） TR:noFunc
	TotalUploaded          int64                  // 种子上传的总数据量（字节）
	TotalUploadedSession   int64                  // 种子上传的总数据量（字节） TR:noFunc

	FileCount int32 // 拥有文件数 TR:noFunc

	RatioLimit    col.Option[float32] // 设置的分享比限制
	SeedTimeLimit col.Option[int64]   // 种子达到的最大做种时间限制（秒）

	Peers         map[PeerKey]struct{}
	PeerCount     int64 // 可用 Peers 的数量
	MaxPeerCount  int64 // 种子连接数限制
	PeerSendCount int64 // 连接到的种子数量

	Progress          float32           // 种子的下载进度
	LastActivity      *time.Time        // 最近一次上传或下载的时间
	Ratio             float32           // 种子的分享比。最大值为 9999
	LeftUntilDone     int64             // 还需下载的数据量（字节数）
	DownloadLimit     col.Option[int64] // 种子的下载速度限制（Byte/s）
	DownloadSpeed     int64             // 当前种子的下载速度（Byte/s）
	Downloaded        int64             // 已下载的数据量
	DownloadedSession int64             // 本次会话中已下载的数据量
	TimeDownloading   time.Duration     // 已经下载时长
	UploadLimit       col.Option[int64] // 当前种子的下载速度（Byte/s）
	UploadSpeed       int64             // 种子的上传速度（Byte/s）
	Uploaded          int64             // 已上传的数据量
	UploadedSession   int64             // 本次会话中已上传的数据量
	TimeUploading     time.Duration     // 已经做种时长
	UploadRatio       float32           // 种子的分享比
	Priority          int32             // 种子的优先级 若队列已禁用或处于做种模式，则返回 -1
	SeedingTime       time.Duration     // 种子完成后的做种时间（秒）
}

type TorrentFilter struct {
	Status col.Option[string]
	Label  col.Option[string]
	Hashes col.Option[[]string]
}

type Statistics struct {
	TotalDownloaded        int64 // 所有时间下载总量（字节）
	TotalUploaded          int64 // 所有时间上传总量（字节）
	TotalDownloadedSession int64 // 本次会话下载的总数据（字节）
	TotalUploadedSession   int64 // 本次会话上传的总数据量（字节）
	DownloadSpeed          int64 // 本次会话下载的速度（字节/秒）
	UploadSpeed            int64 // 本次会话上传的速度（字节/秒）
}

// HistoricalStatistics 历史统计数据（写盘统计）
type HistoricalStatistics struct {
	TotalDownloaded int64 // 所有时间下载总量（字节）
	TotalUploaded   int64 // 所有时间上传总量（字节）
}

// TorrentRepo .
type TorrentRepo interface {

	// GetResponseLine 安行获取指定URL内容
	GetResponseLine(_ context.Context, trackerListURL string) ([]string, error)

	// AddTorrent 添加种子
	AddTorrent(ctx context.Context, torrents []*DownloadTorrent) error

	// UpTracker 更新Tracker
	UpTracker(ctx context.Context, ids []int64, trackers []string) (err error)

	// GetTorrent 获取种子
	GetTorrent(ctx context.Context, hash string) (col.Option[transmissionrpc.Torrent], error)

	// GetTorrentAll 获取所有种子
	GetTorrentAll(ctx context.Context) (col.Option[[]transmissionrpc.Torrent], error)

	// GetPeer 获取Peer
	GetPeer(ctx context.Context, key PeerKey) (col.Option[*Peer], error)

	// SetPeer 设置Peer
	SetPeer(ctx context.Context, key PeerKey, peer *Peer) error

	// GetStateRefreshInterval 获取状态更新间隔(秒)
	GetStateRefreshInterval() int64

	// GetHistoricalStatistics 获取历史统计数据
	GetHistoricalStatistics() (HistoricalStatistics, error)

	// SaveHistoricalStatistics 保存历史统计
	SaveHistoricalStatistics(statistics HistoricalStatistics) error

	// CacheTmpTorrentFile 缓存临时种子文件
	CacheTmpTorrentFile(ctx context.Context, filename string, data []byte) error

	// GetTmpTorrentFile 获取缓存的临时种子文件
	GetTmpTorrentFile(ctx context.Context, filename string) (data col.Option[[]byte], err error)
}

// TorrentUsecase .
type TorrentUsecase struct {
	repo TorrentRepo
	log  *log.Helper

	statistics Statistics

	// torrentLabel 默认添加到的标签
	torrentLabel col.Option[string]
	// trackers 所有需要使用的Transfer列表
	trackers []string
	// defaultTrackers 配置文件中默认添加的Tracker
	defaultTrackers []string
	// subTransferURL 订阅的Transfer列表URL
	subTransferURL string

	rootURL string

	// torrents key: <Hash>
	torrents map[string]*Torrent
}

// NewTorrentUsecase .
func NewTorrentUsecase(bootstrap *conf.Bootstrap, dao TorrentRepo, logger log.Logger) *TorrentUsecase {
	// 初始化Transfer列表
	config := bootstrap.GetInfra().GetTr()
	subTransferURL := config.GetSubTransfer()
	defaultTrackers := strings.Split(config.GetTransfer(), "\n")
	trackers := make(map[string]struct{}, len(defaultTrackers))
	for _, tracker := range defaultTrackers {
		// 检查url
		urlStr := strings.TrimSpace(tracker)
		if urlStr != "" {
			trackerURL, err := url.ParseRequestURI(urlStr)
			if err == nil {
				trackers[trackerURL.String()] = struct{}{}
			}
		}
	}
	defaultTrackers = make([]string, 0, len(defaultTrackers))
	for urlStr := range trackers {
		defaultTrackers = append(defaultTrackers, urlStr)
	}

	statistics, err := dao.GetHistoricalStatistics()
	if err != nil {
		panic(err)
	}

	uc := &TorrentUsecase{
		repo: dao,
		log:  log.NewHelper(logger),

		statistics: Statistics{
			TotalDownloaded:        statistics.TotalDownloaded,
			TotalUploaded:          statistics.TotalUploaded,
			TotalDownloadedSession: 0,
			DownloadSpeed:          0,
			TotalUploadedSession:   0,
			UploadSpeed:            0,
		},
		torrentLabel:    col.None[string](),
		defaultTrackers: defaultTrackers,
		subTransferURL:  subTransferURL,
		rootURL:         bootstrap.GetTrigger().GetHttp().GetRootRul(),

		torrents: make(map[string]*Torrent, 128),
	}

	torrentLabel := bootstrap.GetInfra().GetTr().GetAddTorrentLabel()
	if torrentLabel != "" {
		uc.torrentLabel = col.Some(torrentLabel)
	}

	return uc
}

// UpTrackerList 更新Tracker列表
func (uc *TorrentUsecase) UpTrackerList(ctx context.Context) (err error) {
	// 完整的更新一次tracker列表
	i := 0

	trackers := make(map[string]struct{}, len(uc.trackers))
	for _, tracker := range uc.defaultTrackers {
		trackers[tracker] = struct{}{}
		i = i + 1
	}

	lines, err := uc.repo.GetResponseLine(ctx, uc.subTransferURL)
	if err != nil {
		return
	}

	for _, line := range lines {
		// 检查url
		urlStr := strings.TrimSpace(line)
		if urlStr != "" {
			trackerURL, err := url.ParseRequestURI(urlStr)
			if err == nil {
				trackers[trackerURL.String()] = struct{}{}
				i = i + 1
			}
		}
		if i >= trackerMaxSize {
			break
		}
	}

	// 缓存下来，当添加种子时使用
	uc.trackers = make([]string, 0, len(trackers))
	for tracker := range trackers {
		uc.trackers = append(uc.trackers, tracker)
	}
	return
}

// UpTorrentALLTrackerList 更新所有种子的Tracker
func (uc *TorrentUsecase) UpTorrentALLTrackerList(ctx context.Context) (err error) {
	torrentsOption, err := uc.repo.GetTorrentAll(ctx)
	if err != nil {
		return
	}
	if !torrentsOption.HasValue() {
		return
	}
	torrents := torrentsOption.Value()

	ids := make([]int64, 0, len(torrents))
	for _, trt := range torrents {
		ids = append(ids, *trt.ID)
	}

	err = uc.repo.UpTracker(ctx, ids, uc.trackers)
	return
}

// Add 添加种子
func (uc *TorrentUsecase) Add(ctx context.Context, torrents []*DownloadTorrent) (err error) {
	if uc.torrentLabel.HasValue() {
		for _, torrent := range torrents {
			var labels []string
			if torrent.Labels.HasValue() {
				labels = torrent.Labels.Value()
			} else {
				labels = make([]string, 0, 1)
			}
			labels = append(labels, uc.torrentLabel.Value())
			torrent.Labels = col.Some(labels)
			torrent.Trackers = uc.trackers
		}
	}
	err = uc.repo.AddTorrent(ctx, torrents)

	// TODO 立刻更新数据
	return
}

// GetTorrentList 获取种子列表
func (uc *TorrentUsecase) GetTorrentList(_ context.Context, filter TorrentFilter) (
	res col.Option[[]*pb.TorrentInfo], err error) {

	res = col.None[[]*pb.TorrentInfo]()

	torrents := make([]*Torrent, 0, len(uc.torrents))
	for _, torrent := range uc.torrents {
		torrents = append(torrents, torrent)
	}

	torrents = uc.filterTorrent(torrents, filter)

	qbTorrents := make([]*pb.TorrentInfo, 0, len(torrents))
	for _, trt := range torrents {
		qbt := torrentToQBTorrent(trt)
		qbTorrents = append(qbTorrents, qbt)
	}

	res = col.Some(qbTorrents)
	return
}

// GetTorrentProperties 获取种子属性
func (uc *TorrentUsecase) GetTorrentProperties(_ context.Context, hash string) (
	res col.Option[*pb.GetPropertiesResponse], err error) {

	res = col.None[*pb.GetPropertiesResponse]()

	torrent, ok := uc.torrents[hash]
	if !ok {
		return
	}

	qbt := &pb.GetPropertiesResponse{
		SavePath:               torrent.TorrentPath,            // 种子数据存储的路径
		CreationDate:           torrent.CreationDate.Unix(),    // 种子创建日期（Unix 时间戳）
		AdditionDate:           torrent.AddedDate.Unix(),       // 添加此 torrent 的时间（Unix 时间戳）
		Comment:                torrent.Comment,                // 种子评论
		CreatedBy:              torrent.Creator,                // 种子创建者
		TotalSize:              torrent.TotalSize,              // 种子总大小（字节）
		PieceSize:              0,                              // 种子片段大小（字节）
		TotalWasted:            torrent.TotalWasted,            // 种子浪费的总数据量（字节）
		TotalUploaded:          torrent.TotalUploaded,          // 种子上传的总数据量（字节）
		TotalUploadedSession:   torrent.TotalUploadedSession,   // 种子上传的总数据量（字节） TR:noFunc
		TotalDownloaded:        torrent.TotalDownloaded,        // 种子下载的总数据量（字节）
		TotalDownloadedSession: torrent.TotalDownloadedSession, // 本次会话下载的总数据（字节） TR:noFunc
		IsPrivate:              torrent.IsPrivate,              // 如果 torrent 来自私人追踪器，则为 True

		ShareRatio: torrent.Ratio,         // 种子分享比例
		DlSpeedAvg: torrent.DownloadSpeed, // 种子平均下载速度（字节/秒） TR:noFunc
		DlSpeed:    torrent.DownloadSpeed, // 种子下载速度（字节/秒）
		DlLimit:    -1,                    // 种子的下载速度限制
		UpSpeedAvg: torrent.UploadSpeed,   // 种子平均上传速度（字节/秒） TR:noFunc
		UpSpeed:    torrent.UploadSpeed,   // 种子上传速度（字节/秒）
		UpLimit:    -1,                    // 当前种子的下载速度

		NbConnections:      torrent.PeerCount,     // 种子连接数 TR:noFunc
		NbConnectionsLimit: torrent.MaxPeerCount,  // 种子连接数限制
		Peers:              torrent.PeerCount,     // 连接到的对等点数量
		PeersTotal:         torrent.PeerCount,     // 群体中的同伴数量
		PiecesHave:         torrent.FileCount,     // 拥有件数 TR:noFunc
		PiecesNum:          torrent.FileCount,     // 种子文件的数量
		Reannounce:         300,                   // 距离下一次广播的秒数 TR:noFunc
		Seeds:              torrent.PeerSendCount, // 连接到的种子数量
		SeedsTotal:         torrent.PeerCount,     // 群体中的种子数量 TR:noFunc

		// 种子运行时间（秒）
		TimeElapsed: int64(torrent.TimeDownloading.Seconds()) + int64(torrent.TimeUploading.Seconds()),

		// 种子的预计完成时间（秒）
		Eta:         0, // 种子的预计完成时间（秒）
		SeedingTime: 0, // 种子完成时所用的时间（秒） TR:noFunc

		CompletionDate: torrent.DoneDate.Value().Unix(), // 种子完成日期（Unix 时间戳）
		LastSeen:       torrent.DoneDate.Value().Unix(), // 最后看到的完整日期（Unix 时间戳） TR:noFunc
	}

	if torrent.DownloadLimit.HasValue() {
		qbt.DlLimit = torrent.DownloadLimit.Value() // 种子的下载速度限制
	}
	if torrent.UploadLimit.HasValue() {
		qbt.UpLimit = torrent.UploadLimit.Value() // 当前种子的下载速度
	}

	if torrent.DoneDate.HasValue() {
		qbt.CompletionDate = torrent.DoneDate.Value().Unix() // 种子完成日期（Unix 时间戳）
		qbt.LastSeen = torrent.DoneDate.Value().Unix()       // 最后看到的完整日期（Unix 时间戳） TR:noFunc
	}

	if torrent.PieceSize.HasValue() {
		qbt.PieceSize = torrent.PieceSize.Value() // 种子片段大小（字节）
	}

	res = col.Some(qbt)
	return
}

// GetPeers 获取种子 peer 数据
func (uc *TorrentUsecase) GetPeers(ctx context.Context, hash string) (res col.Option[map[PeerKey]*Peer], err error) {
	res = col.None[map[PeerKey]*Peer]()
	torrent, ok := uc.torrents[hash]
	if !ok {
		return
	}

	peers := make(map[PeerKey]*Peer, len(torrent.Peers))
	for key := range torrent.Peers {
		var peerOption col.Option[*Peer]
		peerOption, err = uc.repo.GetPeer(ctx, key)
		if err != nil {
			return
		}
		if !peerOption.HasValue() {
			continue
		}
		peer := peerOption.Value()

		if !peer.PauseTrackCounter.HasValue() {
			// 暂停跟踪计数器
			track := peer.PauseTrackCounter.Value() - 1
			if track > 0 {
				peer.PauseTrackCounter = col.Some(track)
				continue
			}
			peer.PauseTrackCounter = col.None[int8]()
		}
		if !peer.IsActive {
			continue
		}
		peers[key] = peer
	}
	res = col.Some(peers)
	return
}

// UpClientData 更新tr客户端数据
func (uc *TorrentUsecase) UpClientData(ctx context.Context) (err error) {
	torrentsOption, err := uc.repo.GetTorrentAll(ctx)
	if err != nil {
		return
	}
	if !torrentsOption.HasValue() {
		return
	}
	trTorrents := torrentsOption.Value()

	totalDownloadedIncrement := int64(0) // 本次数据刷新下载的总数据（字节）
	totalUploadedIncrement := int64(0)   // 本次数据刷新上传的总数据量（字节）

	downloadSpeed := int64(0)
	uploadSpeed := int64(0)

	tmpTorrents := make(map[string]*Torrent, len(torrentsOption.Value()))
	for _, trt := range trTorrents {
		torrent := trTorrentToTorrent(trt)
		torrent.Peers = make(map[PeerKey]struct{}, len(trt.Peers))

		for _, trPeer := range trt.Peers {
			key := PeerKey{*trt.HashString, trPeer.Address, int32(trPeer.Port)}
			peerInfoOption, err := uc.repo.GetPeer(ctx, key)
			if err != nil {
				return err
			}

			intervalDownloaded := uc.repo.GetStateRefreshInterval() * trPeer.RateToClient
			intervalUploaded := uc.repo.GetStateRefreshInterval() * trPeer.RateToPeer
			totalDownloadedIncrement = totalDownloadedIncrement + intervalDownloaded
			totalUploadedIncrement = totalUploadedIncrement + intervalUploaded
			downloadSpeed = downloadSpeed + trPeer.RateToClient
			uploadSpeed = uploadSpeed + trPeer.RateToPeer

			var peerInfo *Peer
			if peerInfoOption.HasValue() {
				peerInfo = peerInfoOption.Value()
			} else {
				connection := "BT"
				if trPeer.IsUTP {
					connection = "μTP"
				}
				peerInfo = &Peer{
					IP:                trPeer.Address,
					Port:              uint16(trPeer.Port),
					Connection:        connection,
					PeerIdClient:      trPeer.ClientName,
					ClientName:        trPeer.ClientName,
					Progress:          0,
					DownloadSpeed:     0, // B/s
					Downloaded:        0,
					UploadSpeed:       0, // B/s
					Uploaded:          0,
					Flags:             "",
					PauseTrackCounter: col.None[int8](),
					IsActive:          true,
				}
			}
			peerInfo.Progress = float32(trPeer.Progress)
			peerInfo.DownloadSpeed = trPeer.RateToClient
			peerInfo.UploadSpeed = trPeer.RateToPeer
			peerInfo.Downloaded = peerInfo.Downloaded + intervalDownloaded
			peerInfo.Uploaded = peerInfo.Uploaded + intervalUploaded
			peerInfo.Flags = trPeer.FlagStr

			err = uc.repo.SetPeer(ctx, key, peerInfo)
			if err != nil {
				return err
			}
			torrent.Peers[key] = struct{}{}
		}
		tmpTorrents[torrent.Hash] = torrent
	}

	// 更新统计量
	uc.statistics.TotalDownloadedSession = uc.statistics.TotalDownloadedSession + totalDownloadedIncrement
	uc.statistics.TotalUploadedSession = uc.statistics.TotalUploadedSession + totalUploadedIncrement
	uc.statistics.DownloadSpeed = downloadSpeed
	uc.statistics.UploadSpeed = uploadSpeed

	// 更新种子表
	uc.torrents = tmpTorrents

	return
}

// GetStateRefreshInterval 获取状态更新间隔
func (uc *TorrentUsecase) GetStateRefreshInterval() int64 {
	return uc.repo.GetStateRefreshInterval()
}

// GetStatistics 获取统计数据
func (uc *TorrentUsecase) GetStatistics() Statistics {
	return uc.statistics
}

// SaveStatistics 保存统计数据
func (uc *TorrentUsecase) SaveStatistics() (err error) {
	statistics := HistoricalStatistics{
		TotalDownloaded: uc.statistics.TotalDownloaded + uc.statistics.TotalDownloadedSession,
		TotalUploaded:   uc.statistics.TotalUploaded + uc.statistics.TotalUploadedSession,
	}
	err = uc.repo.SaveHistoricalStatistics(statistics)
	return
}

// CacheTmpTorrentFile 缓存临时种子文件
func (uc *TorrentUsecase) CacheTmpTorrentFile(ctx context.Context, data []byte) (fileURL string, err error) {
	if len(data) == 0 {
		return "", errors.ResourceNotExist("空的种子文件数据")
	}

	id := uuid.New().String()
	filename := fmt.Sprintf("%s%s", id, torrentFileSuffix)
	err = uc.repo.CacheTmpTorrentFile(ctx, filename, data)
	if err != nil {
		return
	}

	fileURL, err = url.JoinPath(uc.rootURL, "download", filename)
	return
}

// GetTmpTorrentFile 获取缓存的临时种子文件
func (uc *TorrentUsecase) GetTmpTorrentFile(ctx context.Context, filename string) (data []byte, err error) {
	dataOption, err := uc.repo.GetTmpTorrentFile(ctx, filename)
	if err != nil {
		return
	}
	if !dataOption.HasValue() {
		return nil, errors.ResourceNotExist("种子文件不存在")
	}
	data = dataOption.Value()
	return
}

func (uc *TorrentUsecase) filterTorrent(torrents []*Torrent, filter TorrentFilter) []*Torrent {
	if len(torrents) == 0 {
		return make([]*Torrent, 0)
	}

	// 根据种子哈希值过滤
	if filter.Hashes.HasValue() {
		tmpTorrents := make([]*Torrent, 0, len(torrents))
		for _, torrent := range torrents {
			if _, ok := uc.torrents[torrent.Hash]; ok {
				tmpTorrents = append(tmpTorrents, torrent)
			}
		}
		torrents = tmpTorrents
	}

	// TODO 过滤种子列表的状态

	// 标签筛选
	if filter.Label.HasValue() {
		tmpTorrents := make([]*Torrent, 0, len(torrents))
		for _, torrent := range torrents {
			if !torrent.Labels.HasValue() {
				continue
			}
			// 有标签
			if contains(torrent.Labels.Value(), filter.Label.Value()) {
				tmpTorrents = append(tmpTorrents, torrent)
			}
		}
		torrents = tmpTorrents
	}

	return torrents
}

// 判断字符串数组中是否包含指定元素
func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func torrentToQBTorrent(torrent *Torrent) (qbt *pb.TorrentInfo) {
	qbt = &pb.TorrentInfo{
		Hash:        torrent.Hash,         // 种子的哈希值
		Name:        torrent.Name,         // 种子名称
		ContentPath: torrent.Path,         // 种子内容的绝对路径（多文件种子为根目录路径，单文件种子为文件路径）
		MagnetUri:   torrent.URL,          // 种子的磁力链接
		IsPrivate:   torrent.IsPrivate,    // 如果种子来自私有 Tracker，则为 true
		SavePath:    torrent.TorrentPath,  // 种子数据存储的路径
		Size:        torrent.SizeWhenDone, // 已选文件的总大小（字节数）
		TotalSize:   torrent.TotalSize,    // 种子的总大小（包括未选择的文件，单位：字节）

		AddedOn:           torrent.AddedDate.Unix(),             // 客户端添加该种子的时间（Unix 时间戳）
		LastActivity:      torrent.LastActivity.Unix(),          // 最近一次上传或下载的时间（Unix 时间戳）
		AmountLeft:        torrent.LeftUntilDone,                // 还需下载的数据量（字节数）
		Priority:          torrent.Priority,                     // 种子的优先级。若队列已禁用或处于做种模式，则返回 -1
		SeedingTime:       int64(torrent.SeedingTime.Seconds()), // 种子完成后的做种时间（秒）
		Ratio:             torrent.UploadRatio,                  // 种子的分享比。最大值为 9999
		Progress:          torrent.Progress,                     // 种子的下载进度（百分比）
		Dlspeed:           torrent.DownloadSpeed,                // 种子的下载速度（字节/秒）
		Downloaded:        torrent.Downloaded,                   // 已下载的数据量
		DownloadedSession: torrent.DownloadedSession,            // 本次会话中已下载的数据量 TR:noFunc
		Upspeed:           torrent.UploadSpeed,                  // 种子的上传速度（字节/秒）
		Uploaded:          torrent.Uploaded,                     // 已上传的数据量
		UploadedSession:   torrent.UploadedSession,              // 本次会话中已上传的数据量 TR:noFunc

		// 已完成的数据量（字节数） TR:总大小x已完成百分比
		//Completed: int64(float64(torrent.TotalSize) * float64(torrent.Progress)),
		Completed: torrent.HaveValidSize,
		// 种子的总活跃时间（秒） TR:下载时间+做种时间
		TimeActive: int64(torrent.TimeDownloading.Seconds()) + int64(torrent.TimeUploading.Seconds()),

		Eta:           0,     // 种子的预计完成时间（秒）
		FLPiecePrio:   false, // 如果首尾片段已优先下载，则为 true TR:noFunc
		ForceStart:    false, // 如果启用了强制启动，则为 true TR:noFunc
		AutoTmm:       false, // 是否由自动种子管理管理
		Availability:  0,     // 当前可用的文件片段百分比
		Category:      "",    // 种子的类别 TR:noFunc
		NumComplete:   0,     // 种群中的做种者数量
		NumIncomplete: 0,     // 种群中的下载者数量
		NumLeechs:     0,     // 已连接的下载者数量
		NumSeeds:      0,     // 已连接的做种者数量
		SeqDl:         false, // 如果启用了顺序下载，则为 true TR:noFunc
		State:         "",    // 种子的状态 TODO
		SuperSeeding:  false, // 如果启用了超级做种模式，则为 true TR:noFunc
		Tracker:       "",    // 第一个处于工作状态的 Tracker。如果没有工作中的 Tracker，则返回空字符串
	}

	tags := "" // 种子的标签列表，以逗号分隔
	if torrent.Labels.HasValue() {
		tags = strings.Join(torrent.Labels.Value(), ",")
	}
	qbt.Tags = tags

	if torrent.DownloadLimit.HasValue() {
		qbt.DlLimit = torrent.DownloadLimit.Value() // 种子的下载速度限制
	}
	if torrent.UploadLimit.HasValue() {
		qbt.UpLimit = torrent.UploadLimit.Value() // 当前种子的下载速度
	}

	if torrent.DoneDate.HasValue() {
		qbt.CompletionOn = torrent.DoneDate.Value().Unix() // 种子完成下载的时间（Unix 时间戳）
		qbt.SeenComplete = torrent.DoneDate.Value().Unix() // 种子上次完成的时间（Unix 时间戳）
	}

	if torrent.RatioLimit.HasValue() {
		qbt.RatioLimit = torrent.RatioLimit.Value() // 设置的分享比限制
		qbt.MaxRatio = torrent.RatioLimit.Value()   // 达到最大分享率后停止做种的最大分享比
	}
	if torrent.SeedTimeLimit.HasValue() {
		// 种子达到的最大做种时间限制（秒）。如果自动管理启用，则为 -2；未设置时默认为 -1
		qbt.SeedingTimeLimit = torrent.SeedTimeLimit.Value()
		qbt.MaxSeedingTime = torrent.SeedTimeLimit.Value() // 达到最大做种时间（秒）后停止做种
	} else {
		qbt.SeedingTimeLimit = -1
	}

	return qbt
}

func trTorrentToTorrent(trt transmissionrpc.Torrent) *Torrent {
	torrent := &Torrent{
		Hash:                   *trt.HashString,
		Name:                   *trt.Name,
		URL:                    *trt.MagnetLink,
		Path:                   *trt.DownloadDir,
		TorrentPath:            *trt.TorrentFile,
		Labels:                 col.None[[]string](),
		CreationDate:           trt.DateCreated,
		AddedDate:              trt.AddedDate,
		DoneDate:               col.Some(trt.DoneDate),
		Creator:                "",
		Comment:                *trt.Comment,
		IsPrivate:              *trt.IsPrivate,
		SizeWhenDone:           BitsToBytes(trt.SizeWhenDone),
		TotalSize:              BitsToBytes(trt.TotalSize),
		HaveValidSize:          *trt.HaveValid,
		PieceSize:              col.None[int64](),
		TotalWasted:            *trt.CorruptEver,
		TotalDownloaded:        *trt.DownloadedEver,
		TotalDownloadedSession: 0,
		TotalUploaded:          *trt.UploadedEver,
		TotalUploadedSession:   0,
		FileCount:              int32(len(trt.Files)),
		RatioLimit:             col.Some(float32(*trt.SeedRatioLimit)),
		SeedTimeLimit:          col.Some(int64(trt.SeedIdleLimit.Seconds())),
		Peers:                  make(map[PeerKey]struct{}),
		PeerCount:              *trt.PeersConnected,
		MaxPeerCount:           *trt.MaxConnectedPeers,
		PeerSendCount:          *trt.PeersSendingToUs,
		Progress:               0,
		LastActivity:           trt.StartDate,
		Ratio:                  float32(*trt.UploadRatio),
		LeftUntilDone:          *trt.LeftUntilDone,
		DownloadLimit:          col.None[int64](),
		DownloadSpeed:          *trt.RateDownload,
		Downloaded:             *trt.DownloadedEver,
		DownloadedSession:      0,
		TimeDownloading:        *trt.TimeDownloading,
		UploadLimit:            col.None[int64](),
		UploadSpeed:            *trt.RateUpload,
		Uploaded:               *trt.UploadedEver,
		UploadedSession:        0,
		TimeUploading:          *trt.TimeSeeding,
		UploadRatio:            float32(*trt.UploadRatio),
		Priority:               int32(*trt.BandwidthPriority),
		SeedingTime:            *trt.TimeSeeding,
	}

	torrent.Progress = float32((float64(torrent.HaveValidSize) - float64(torrent.LeftUntilDone)) / float64(torrent.HaveValidSize))

	if len(trt.Labels) > 0 {
		torrent.Labels = col.Some(trt.Labels)
	}

	if trt.DownloadLimited != nil && *trt.DownloadLimited {
		torrent.DownloadLimit = col.Some(*trt.DownloadLimit)
	}
	if trt.UploadLimited != nil && *trt.UploadLimited {
		torrent.UploadLimit = col.Some(*trt.UploadLimit)
	}

	if trt.PieceSize != nil {
		torrent.PieceSize = col.Some(BitsToBytes(trt.PieceSize))
	}

	return torrent
}

func BitsToBytes(bits *cunits.Bits) int64 {
	if bits == nil {
		return 0
	}
	return int64(*bits) / 8
}
