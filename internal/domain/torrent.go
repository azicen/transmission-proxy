package domain

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	pb "transmission-proxy/api/v2"
	"transmission-proxy/conf"
	"transmission-proxy/internal/errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/hekmon/transmissionrpc/v3"
	col "github.com/noxiouz/golang-generics-util/collection"
)

const torrentFileSuffix = ".torrent"

type PeerKey struct {
	hash string
	ip   string
	port int32
}

func (p PeerKey) String() string {
	return fmt.Sprintf("%s:%s:%d", p.hash, p.ip, p.port)
}

// Peer 种子节点列表中的节点元数据
type Peer struct {
	Ip           string // IP 地址
	Port         int32  // 端口号
	Connection   string // 连接类型
	PeerIdClient string // 客户端的 Peer ID
	ClientName   string // 客户端名称
	//Country     string  // 国家
	//CountryCode string  // 国家代码
	Progress      float64 // 进度（0-100%）
	DownloadSpeed int64   // 下载速度（字节/秒）
	Downloaded    int64   // 已下载数据量（字节） 代理计算
	UploadSpeed   int64   // 上传速度（字节/秒）
	Uploaded      int64   // 已上传数据量（字节） 代理计算
	Flags         string  // 标志信息
}

type Torrent struct {
	URL      string               // 种子url
	Path     col.Option[string]   // 种子保存路径
	Labels   col.Option[[]string] // 种子tag
	Cookie   col.Option[string]   // 发送 Cookie 以下载 .torrent 文件
	Paused   bool                 // 在暂停状态下添加种子
	Trackers []string             // 添加到种子的Tracker列表
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
	AddTorrent(ctx context.Context, torrents []*Torrent) error

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
	}

	torrentLabel := bootstrap.GetInfra().GetTr().GetAddTorrentLabel()
	if torrentLabel != "" {
		uc.torrentLabel = col.Some(torrentLabel)
	}

	return uc
}

// UpTrackerList 更新Tracker列表
func (uc *TorrentUsecase) UpTrackerList(ctx context.Context) (err error) {
	trackers := make(map[string]struct{}, len(uc.trackers))
	for _, tracker := range uc.defaultTrackers {
		trackers[tracker] = struct{}{}
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
			}
		}
	}

	uc.trackers = make([]string, 0, len(trackers))
	for tracker := range trackers {
		uc.trackers = append(uc.trackers, tracker)
	}
	return
}

// UpTorrentTrackerList 更新种子的Tracker
func (uc *TorrentUsecase) UpTorrentTrackerList(ctx context.Context) (err error) {
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
func (uc *TorrentUsecase) Add(ctx context.Context, torrents []*Torrent) (err error) {
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
	return
}

// GetTorrentList 获取种子列表
func (uc *TorrentUsecase) GetTorrentList(ctx context.Context, filter TorrentFilter) (
	col.Option[[]*pb.TorrentInfo], error) {

	torrentsOption, err := uc.repo.GetTorrentAll(ctx)
	if err != nil {
		return col.None[[]*pb.TorrentInfo](), err
	}
	if !torrentsOption.HasValue() {
		return col.None[[]*pb.TorrentInfo](), nil
	}
	torrents := filterTorrent(torrentsOption.Value(), filter)

	qbTorrents := make([]*pb.TorrentInfo, 0, len(torrents))
	for _, trt := range torrents {
		qbt := trTorrentToQBTorrent(trt)
		qbTorrents = append(qbTorrents, qbt)
	}

	return col.Some(qbTorrents), err
}

// GetTorrentProperties 获取种子属性
func (uc *TorrentUsecase) GetTorrentProperties(ctx context.Context, hash string) (
	col.Option[*pb.GetPropertiesResponse], error) {

	torrentOption, err := uc.repo.GetTorrent(ctx, hash)
	if err != nil {
		return col.None[*pb.GetPropertiesResponse](), err
	}
	if !torrentOption.HasValue() {
		return col.None[*pb.GetPropertiesResponse](), nil
	}
	trt := torrentOption.Value()

	// 种子的下载速度限制（字节/秒），-1 表示无限制
	downloadLimit := int64(-1)
	if trt.DownloadLimited != nil && *trt.DownloadLimited {
		downloadLimit = *trt.DownloadLimit
	}
	// 当前种子的下载速度（字节/秒）
	uploadLimit := int64(-1)
	if trt.UploadLimited != nil && *trt.UploadLimited {
		downloadLimit = *trt.UploadLimit
	}
	// 种子的预计完成时间（秒）
	eta := int64(0)
	if trt.ETAIdle != nil {
		eta = *trt.ETAIdle
	}
	if trt.ETA != nil {
		eta = *trt.ETA
	}

	pieceSize := int64(-1)
	if trt.PieceSize != nil {
		pieceSize = int64(*trt.PieceSize)
	}

	qbt := &pb.GetPropertiesResponse{
		SavePath:               *trt.TorrentFile,       // 种子数据存储的路径
		CreationDate:           trt.DateCreated.Unix(), // 种子创建日期（Unix 时间戳）
		PieceSize:              pieceSize,              // 种子片段大小（字节）
		Comment:                *trt.Comment,           // 种子评论
		TotalWasted:            *trt.CorruptEver,       // 种子浪费的总数据量（字节）
		TotalUploaded:          *trt.UploadedEver,      // 种子上传的总数据量（字节）
		TotalUploadedSession:   0,                      // 种子上传的总数据量（字节） TR:noFunc
		TotalDownloaded:        *trt.DownloadedEver,    // 种子下载的总数据量（字节）
		TotalDownloadedSession: 0,                      // 本次会话下载的总数据（字节） TR:noFunc
		UpLimit:                uploadLimit,
		DlLimit:                downloadLimit,
		// 种子运行时间（秒）
		TimeElapsed:        int64(trt.TimeDownloading.Seconds()) + int64(trt.TimeSeeding.Seconds()),
		SeedingTime:        0,                         // 种子完成时所用的时间（秒） TR:noFunc
		NbConnections:      *trt.PeersConnected,       // 种子连接数 TR:noFunc
		NbConnectionsLimit: *trt.MaxConnectedPeers,    // 种子连接数限制
		ShareRatio:         float32(*trt.UploadRatio), // 种子分享比例
		AdditionDate:       trt.AddedDate.Unix(),      // 添加此 torrent 的时间（Unix 时间戳）
		CompletionDate:     trt.DoneDate.Unix(),       // 种子完成日期（Unix 时间戳）
		CreatedBy:          *trt.Creator,              // 种子创建者
		DlSpeedAvg:         *trt.RateDownload,         // 种子平均下载速度（字节/秒） TR:noFunc
		DlSpeed:            *trt.RateDownload,         // 种子下载速度（字节/秒）
		// 种子的预计完成时间（秒）
		Eta:        eta,
		LastSeen:   trt.DoneDate.Unix(),   // 最后看到的完整日期（Unix 时间戳） TR:noFunc
		Peers:      *trt.PeersConnected,   // 连接到的对等点数量
		PeersTotal: int64(len(trt.Peers)), // 群体中的同伴数量
		PiecesHave: int32(len(trt.Files)), // 拥有件数 TR:noFunc
		PiecesNum:  int32(len(trt.Files)), // 种子文件的数量
		Reannounce: 300,                   // 距离下一次广播的秒数 TR:noFunc
		Seeds:      *trt.PeersSendingToUs, // 连接到的种子数量
		SeedsTotal: int64(len(trt.Peers)), // 群体中的种子数量 TR:noFunc
		TotalSize:  int64(*trt.TotalSize), // 种子总大小（字节）
		UpSpeedAvg: *trt.RateUpload,       // 种子平均上传速度（字节/秒） TR:noFunc
		UpSpeed:    *trt.RateUpload,       // 种子上传速度（字节/秒）
		IsPrivate:  *trt.IsPrivate,        // 如果 torrent 来自私人追踪器，则为 True
	}

	return col.Some(qbt), nil
}

// GetTorrentPeers 获取种子 peer 数据
func (uc *TorrentUsecase) GetTorrentPeers(ctx context.Context, hash string) (
	col.Option[map[string]*pb.PeerInfo], error) {

	torrentOption, err := uc.repo.GetTorrent(ctx, hash)
	if err != nil {
		return col.None[map[string]*pb.PeerInfo](), err
	}
	if !torrentOption.HasValue() {
		return col.None[map[string]*pb.PeerInfo](), nil
	}
	trt := torrentOption.Value()

	trPeers := make([]transmissionrpc.Peer, 0, 10)
	if len(trt.Peers) != 0 {
		trPeers = trt.Peers
	}

	qbPeers := make(map[string]*pb.PeerInfo, len(trPeers))
	for _, peer := range trPeers {
		addr := fmt.Sprintf("%s:%d", peer.Address, peer.Port)

		key := PeerKey{hash, peer.Address, int32(peer.Port)}
		peerInfoOption, err := uc.repo.GetPeer(ctx, key)
		if err != nil {
			return col.None[map[string]*pb.PeerInfo](), err
		}
		peerInfo := &Peer{}
		if peerInfoOption.HasValue() {
			peerInfo = peerInfoOption.Value()
		}

		runes := []rune(peerInfo.Flags)
		var b strings.Builder
		for i, r := range runes {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteRune(r)
		}
		flags := b.String()

		qbPeers[addr] = &pb.PeerInfo{
			Client:       peerInfo.ClientName,    // 客户端信息
			Connection:   peerInfo.Connection,    // 连接类型
			Country:      "",                     // 国家 TR:noFunc
			CountryCode:  "",                     // 国家代码 TR:noFunc
			DlSpeed:      peerInfo.DownloadSpeed, // 下载速度（字节/秒）
			Downloaded:   peerInfo.Downloaded,    // 已下载数据量（字节） TR:noFunc 代理统计
			Files:        "",                     // 文件信息 TR:noFunc
			Flags:        flags,                  // 标志信息
			FlagsDesc:    "",                     // 标志描述 TR:noFunc
			Ip:           peerInfo.Ip,            // IP 地址
			PeerIdClient: peerInfo.ClientName,    // 客户端的 Peer ID TR:noFunc 代理统计
			Port:         peerInfo.Port,          // 端口号
			Progress:     peerInfo.Progress,      // 进度（0-100%）
			Relevance:    0,                      // 相关性 TR:noFunc
			Shadowbanned: false,                  // 是否被影子禁用 TR:noFunc
			UpSpeed:      peerInfo.UploadSpeed,   // 上传速度（字节/秒）
			Uploaded:     peerInfo.Uploaded,      // 已上传数据量（字节） TR:noFunc 代理统计
			// 没有验证Uploaded是否可以计算 -> int64(float64(*trt.TotalSize) * peer.Progress)
		}
	}

	return col.Some(qbPeers), err
}

// UpPeerData 更新peer数据
func (uc *TorrentUsecase) UpPeerData(ctx context.Context) (err error) {
	torrentsOption, err := uc.repo.GetTorrentAll(ctx)
	if err != nil {
		return
	}
	if !torrentsOption.HasValue() {
		return
	}
	torrents := torrentsOption.Value()

	totalDownloadedIncrement := int64(0) // 本次数据刷新下载的总数据（字节）
	totalUploadedIncrement := int64(0)   // 本次数据刷新上传的总数据量（字节）

	downloadSpeed := int64(0)
	uploadSpeed := int64(0)

	for _, torrent := range torrents {
		for _, peer := range torrent.Peers {
			key := PeerKey{*torrent.HashString, peer.Address, int32(peer.Port)}
			peerInfoOption, err := uc.repo.GetPeer(ctx, key)
			if err != nil {
				return err
			}

			intervalDownloaded := uc.repo.GetStateRefreshInterval() * peer.RateToClient
			intervalUploaded := uc.repo.GetStateRefreshInterval() * peer.RateToPeer
			totalDownloadedIncrement = totalDownloadedIncrement + intervalDownloaded
			totalUploadedIncrement = totalUploadedIncrement + intervalUploaded
			downloadSpeed = downloadSpeed + peer.RateToClient
			uploadSpeed = uploadSpeed + peer.RateToPeer

			var peerInfo *Peer
			if peerInfoOption.HasValue() {
				peerInfo = peerInfoOption.Value()
			} else {
				connection := "BT"
				if peer.IsUTP {
					connection = "μTP"
				}
				peerInfo = &Peer{
					Ip:            peer.Address,
					Port:          int32(peer.Port),
					Connection:    connection,
					PeerIdClient:  peer.ClientName,
					ClientName:    peer.ClientName,
					Progress:      0,
					DownloadSpeed: 0, // B/s
					Downloaded:    0,
					UploadSpeed:   0, // B/s
					Uploaded:      0,
				}
			}
			peerInfo.Progress = peer.Progress
			peerInfo.DownloadSpeed = peer.RateToClient
			peerInfo.UploadSpeed = peer.RateToPeer
			peerInfo.Downloaded = peerInfo.Downloaded + intervalDownloaded
			peerInfo.Uploaded = peerInfo.Uploaded + intervalUploaded
			peerInfo.Flags = peer.FlagStr

			err = uc.repo.SetPeer(ctx, key, peerInfo)
			if err != nil {
				return err
			}
		}
	}

	// 更新统计量
	uc.statistics.TotalDownloadedSession = uc.statistics.TotalDownloadedSession + totalDownloadedIncrement
	uc.statistics.TotalUploadedSession = uc.statistics.TotalUploadedSession + totalUploadedIncrement
	uc.statistics.DownloadSpeed = downloadSpeed
	uc.statistics.UploadSpeed = uploadSpeed

	return
}

func trTorrentToQBTorrent(trt transmissionrpc.Torrent) *pb.TorrentInfo {
	totalSize := int64(*trt.TotalSize)

	// 种子的下载速度限制（字节/秒），-1 表示无限制
	downloadLimit := int64(-1)
	if trt.DownloadLimited != nil && *trt.DownloadLimited {
		downloadLimit = *trt.DownloadLimit
	}
	// 当前种子的下载速度（字节/秒）
	uploadLimit := int64(-1)
	if trt.UploadLimited != nil && *trt.UploadLimited {
		downloadLimit = *trt.UploadLimit
	}
	// 种子的预计完成时间（秒）
	eta := int64(0)
	if trt.ETAIdle != nil {
		eta = *trt.ETAIdle
	}
	if trt.ETA != nil {
		eta = *trt.ETA
	}

	// 种子的标签列表，以逗号分隔
	tags := ""
	if len(trt.Labels) > 0 {
		tags = strings.Join(trt.Labels, ",")
	}

	// 第一个处于工作状态的 Tracker。如果没有工作中的 Tracker，则返回空字符串
	oneTracker := ""
	if len(trt.TrackerStats) > 0 {
		for _, tracker := range trt.TrackerStats {
			if tracker.LastAnnounceResult == "Success" {
				oneTracker = tracker.Announce
				break
			}
		}
	}

	qbt := &pb.TorrentInfo{
		AddedOn:      trt.AddedDate.Unix(), // 客户端添加该种子的时间（Unix 时间戳）
		AmountLeft:   *trt.LeftUntilDone,   // 还需下载的数据量（字节数）
		AutoTmm:      false,                // 是否由自动种子管理管理
		Availability: 0,                    // 当前可用的文件片段百分比
		Category:     "",                   // 种子的类别 TR:noFunc
		// 已完成的数据量（字节数） TR:总大小x已完成百分比
		Completed:    int64(float64(totalSize) * (*trt.PercentDone)),
		CompletionOn: trt.DoneDate.Unix(), // 种子完成下载的时间（Unix 时间戳）
		ContentPath:  *trt.DownloadDir,    // 种子内容的绝对路径（多文件种子为根目录路径，单文件种子为文件路径）

		DlLimit:           downloadLimit,
		Dlspeed:           *trt.RateDownload,   // 当前种子的下载速度（字节/秒）
		Downloaded:        *trt.DownloadedEver, // 已下载的数据量
		DownloadedSession: 0,                   // 本次会话中已下载的数据量 TR:noFunc

		Eta:          eta,
		FLPiecePrio:  false,                // 如果首尾片段已优先下载，则为 true TR:noFunc
		ForceStart:   false,                // 如果启用了强制启动，则为 true TR:noFunc
		Hash:         *trt.HashString,      // 种子的哈希值
		IsPrivate:    *trt.IsPrivate,       // 如果种子来自私有 Tracker，则为 true
		LastActivity: trt.StartDate.Unix(), // 最近一次上传或下载的时间（Unix 时间戳）
		MagnetUri:    *trt.MagnetLink,      // 种子的磁力链接
		Name:         *trt.Name,            // 种子名称

		MaxRatio:       float32(*trt.SeedRatioLimit),       // 达到最大分享率后停止做种的最大分享比
		MaxSeedingTime: int64(trt.SeedIdleLimit.Seconds()), // 达到最大做种时间（秒）后停止做种

		NumComplete:   0, // 种群中的做种者数量
		NumIncomplete: 0, // 种群中的下载者数量
		NumLeechs:     0, // 已连接的下载者数量
		NumSeeds:      0, // 已连接的做种者数量

		Priority: int32(*trt.BandwidthPriority), // 种子的优先级。若队列已禁用或处于做种模式，则返回 -1
		// 种子的下载进度（百分比/100）
		// TR:(总大小-剩余下载字节数)/总大小/100
		Progress:    float32((float64(totalSize) - float64(*trt.LeftUntilDone)) / float64(totalSize) / 100),
		Ratio:       float32(*trt.UploadRatio),        // 种子的分享比。最大值为 9999
		RatioLimit:  float32(*trt.SeedRatioLimit),     // 设置的分享比限制
		SavePath:    *trt.TorrentFile,                 // 种子数据存储的路径
		SeedingTime: int64(trt.TimeSeeding.Seconds()), // 种子完成后的做种时间（秒）
		// 种子达到的最大做种时间限制（秒）。如果自动管理启用，则为 -2；未设置时默认为 -1
		SeedingTimeLimit: int64(trt.SeedIdleLimit.Seconds()),
		SeenComplete:     trt.DoneDate.Unix(),      // 种子上次完成的时间（Unix 时间戳）
		SeqDl:            false,                    // 如果启用了顺序下载，则为 true TR:noFunc
		Size:             int64(*trt.SizeWhenDone), // 已选文件的总大小（字节数）
		State:            "",                       // 种子的状态 TODO
		SuperSeeding:     false,                    // 如果启用了超级做种模式，则为 true TR:noFunc
		Tags:             tags,
		// 种子的总活跃时间（秒） TR:下载时间+做种时间
		TimeActive: int64(trt.TimeDownloading.Seconds()) + int64(trt.TimeSeeding.Seconds()),
		TotalSize:  totalSize, // 种子的总大小（包括未选择的文件，单位：字节）
		Tracker:    oneTracker,

		UpLimit:         uploadLimit,
		Upspeed:         *trt.RateUpload,   // 种子的上传速度（字节/秒）
		Uploaded:        *trt.UploadedEver, // 已上传的数据量
		UploadedSession: 0,                 // 本次会话中已上传的数据量 TR:noFunc
	}

	return qbt
}

// filterTorrent 过滤种子列表的状态。可选的状态包括：
// "all"（全部）、"downloading"（正在下载）、"seeding"（做种中）、
// "completed"（已完成）、"paused"（已暂停）、"active"（活跃中）、
// "inactive"（空闲）*、"resumed"（恢复）*、"stalled"（停滞中）、
// "stalled_uploading"（上传已停滞）、"stalled_downloading"（下载已停滞）、"errored"（错误）。
func filterTorrent(torrents []transmissionrpc.Torrent, filter TorrentFilter) []transmissionrpc.Torrent {
	if len(torrents) == 0 {
		return make([]transmissionrpc.Torrent, 0)
	}

	// 根据种子哈希值过滤
	if filter.Hashes.HasValue() {
		hashSet := make(map[string]struct{}, len(filter.Hashes.Value()))
		for _, hash := range filter.Hashes.Value() {
			hashSet[hash] = struct{}{}
		}
		tmpTorrents := make([]transmissionrpc.Torrent, 0, len(torrents))
		for _, torrent := range torrents {
			if torrent.HashString == nil {
				continue
			}
			if _, ok := hashSet[*torrent.HashString]; ok {
				tmpTorrents = append(tmpTorrents, torrent)
			}
		}
		torrents = tmpTorrents
	}

	// 过滤种子列表的状态
	if filter.Status.HasValue() {
		status := filter.Status.Value()
		if status == "all" {
			// 什么都不做
		}
		if status == "resumed" {
			// 什么都做不了, tr缺少这个状态
			return make([]transmissionrpc.Torrent, 0)
		}

		torrentStatusFilter := make(map[transmissionrpc.TorrentStatus]struct{}, 10)
		switch status {
		case "downloading":
			// 下载
			// TorrentStatusDownload 表示正在下载的种子
			torrentStatusFilter[transmissionrpc.TorrentStatusDownload] = struct{}{}

		case "seeding":
			// 做种
			// TorrentStatusSeed 表示正在做种的种子
			torrentStatusFilter[transmissionrpc.TorrentStatusSeed] = struct{}{}

		case "active":
			// 活跃
			torrentStatusFilter[transmissionrpc.TorrentStatusDownload] = struct{}{}
			torrentStatusFilter[transmissionrpc.TorrentStatusSeed] = struct{}{}

		case "paused":
			// 停止
			// TorrentStatusStopped 表示已停止的种子
			torrentStatusFilter[transmissionrpc.TorrentStatusStopped] = struct{}{}

		case "stalled_uploading":
			// TorrentStatusSeedWait 表示排队等待做种的种子
			// 上传已暂停
			torrentStatusFilter[transmissionrpc.TorrentStatusSeedWait] = struct{}{}

		case "stalled_downloading":
			// 下载已暂停
			// TorrentStatusDownloadWait 表示排队等待下载的种子
			torrentStatusFilter[transmissionrpc.TorrentStatusDownloadWait] = struct{}{}

		case "checking":
			// 正在检查
			// TorrentStatusCheckWait 表示排队等待校验文件的种子
			// TorrentStatusCheck 表示正在校验文件的种子
			torrentStatusFilter[transmissionrpc.TorrentStatusCheckWait] = struct{}{}
			torrentStatusFilter[transmissionrpc.TorrentStatusCheck] = struct{}{}
		}

		if len(torrentStatusFilter) > 0 {
			tmpTorrents := make([]transmissionrpc.Torrent, 0, len(torrents))
			for _, torrent := range torrents {
				if torrent.Status == nil {
					continue
				}
				if _, ok := torrentStatusFilter[*torrent.Status]; ok {
					tmpTorrents = append(tmpTorrents, torrent)
				}
			}
			torrents = tmpTorrents
		}

		if status == "completed" {
			tmpTorrents := make([]transmissionrpc.Torrent, 0, len(torrents))
			for _, torrent := range torrents {
				// 是否完成
				if torrent.IsFinished == nil {
					continue
				}
				if *torrent.IsFinished {
					tmpTorrents = append(tmpTorrents, torrent)
				}
			}
			torrents = tmpTorrents
		}

		if status == "stalled" || status == "inactive" {
			tmpTorrents := make([]transmissionrpc.Torrent, 0, len(torrents))
			for _, torrent := range torrents {
				// 是否完成
				if torrent.IsStalled == nil {
					continue
				}
				if *torrent.IsStalled {
					tmpTorrents = append(tmpTorrents, torrent)
				}
			}
			torrents = tmpTorrents
		}

		if status == "errored" {
			tmpTorrents := make([]transmissionrpc.Torrent, 0, len(torrents))
			for _, torrent := range torrents {
				// 有错误
				if torrent.Error == nil {
					continue
				}
				tmpTorrents = append(tmpTorrents, torrent)
			}
			torrents = tmpTorrents
		}
	}

	// 标签筛选
	if filter.Label.HasValue() {
		tmpTorrents := make([]transmissionrpc.Torrent, 0, len(torrents))
		for _, torrent := range torrents {
			// 有标签
			if len(torrent.Labels) == 0 {
				continue
			}
			if contains(torrent.Labels, filter.Label.Value()) {
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
