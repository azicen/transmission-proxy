package service

import (
	"context"
	"fmt"
	"strings"

	"transmission-proxy/internal/domain"

	col "github.com/noxiouz/golang-generics-util/collection"
	pb "transmission-proxy/api/v2"
)

type SyncService struct {
	pb.UnimplementedSyncServer

	uc *domain.TorrentUsecase
}

func NewSyncService(uc *domain.TorrentUsecase) *SyncService {
	return &SyncService{
		uc: uc,
	}
}

// GetMainData 获取 Main Data
func (s *SyncService) GetMainData(ctx context.Context, req *pb.GetMainDataRequest) (*pb.GetMainDataResponse, error) {
	torrentOptions, err := s.uc.GetTorrentList(ctx, domain.TorrentFilter{
		Status:   col.None[string](),
		Category: col.None[string](),
		Label:    col.None[string](),
		Hashes:   col.None[[]string](),
	})
	if err != nil {
		return nil, err
	}
	torrents := make([]*pb.TorrentInfo, 0)
	if torrentOptions.HasValue() {
		torrents = torrentOptions.Value()
	}
	torrentMap := make(map[string]*pb.TorrentInfo, len(torrents))
	for _, torrent := range torrents {
		torrentMap[torrent.Hash] = torrent
	}

	statistics := s.uc.GetStatistics()

	return &pb.GetMainDataResponse{
		Rid:               req.Rid,
		FullUpdate:        true,
		Torrents:          torrentMap,
		TorrentsRemoved:   nil,
		Categories:        make(map[string]*pb.Category),
		CategoriesRemoved: make([]string, 0),
		Tags:              make([]string, 0),
		TagsRemoved:       make([]string, 0),
		ServerState: &pb.ServerState{
			AlltimeDl:            statistics.TotalDownloaded + statistics.TotalDownloadedSession,
			AlltimeUl:            statistics.TotalUploaded + statistics.TotalUploadedSession,
			AverageTimeQueue:     0,
			ConnectionStatus:     "",
			DhtNodes:             0,
			DlInfoData:           statistics.TotalDownloadedSession,
			DlInfoSpeed:          statistics.DownloadSpeed,
			DlRateLimit:          0,
			FreeSpaceOnDisk:      0,
			GlobalRatio:          "",
			QueuedIoJobs:         0,
			Queueing:             false,
			ReadCacheHits:        "",
			ReadCacheOverload:    "",
			RefreshInterval:      0,
			TotalBuffersSize:     0,
			TotalPeerConnections: 0,
			TotalQueuedSize:      0,
			TotalWastedSession:   0,
			UpInfoData:           statistics.TotalUploadedSession,
			UpInfoSpeed:          statistics.UploadSpeed,
			UpRateLimit:          0,
			UseAltSpeedLimits:    false,
			UseSubcategories:     false,
			WriteCacheOverload:   "",
		},
	}, nil
}

// GetTorrentPeers 获取种子 peer 数据
func (s *SyncService) GetTorrentPeers(ctx context.Context, req *pb.GetTorrentPeersRequest) (
	res *pb.GetTorrentPeersResponse, err error) {

	peers, err := s.uc.GetPeers(ctx, req.GetHash())
	if err != nil {
		return
	}
	res = &pb.GetTorrentPeersResponse{
		FullUpdate: true, // 是否为完整更新 TR:noFunc
		ShowFlags:  true, // 是否显示标志 TR:noFunc
		Rid:        req.GetRid(),
		Peers:      make(map[string]*pb.PeerInfo),
	}

	if !peers.HasValue() {
		return
	}

	res.Peers = make(map[string]*pb.PeerInfo, len(peers.Value()))
	for key, peer := range peers.Value() {
		res.Peers[genAddr(&key)] = peerToQBPeerInfo(peer)
	}

	return
}

// 构建key，key: <ip:port>
var genAddr = func(key *domain.PeerKey) string {
	return fmt.Sprintf("%s%d", key.IP, key.Port)
}

func peerToQBPeerInfo(peer *domain.Peer) *pb.PeerInfo {
	runes := []rune(peer.Flags)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteRune(r)
	}
	flags := b.String()

	res := &pb.PeerInfo{
		Client:       peer.ClientName,        // 客户端信息
		Connection:   peer.Connection,        // 连接类型
		Country:      "",                     // 国家 TR:noFunc
		CountryCode:  "",                     // 国家代码 TR:noFunc
		DlSpeed:      peer.DownloadSpeed,     // 下载速度（字节/秒）
		Downloaded:   peer.Downloaded,        // 已下载数据量（字节） TR:noFunc 代理统计
		Files:        "",                     // 文件信息 TR:noFunc
		Flags:        flags,                  // 标志信息
		FlagsDesc:    "",                     // 标志描述 TR:noFunc
		Ip:           peer.IP,                // IP 地址
		PeerIdClient: peer.ClientName,        // 客户端的 Peer ID TR:noFunc 代理统计
		Port:         int32(peer.Port),       // 端口号
		Progress:     float64(peer.Progress), // 进度（0-100%）
		Relevance:    0,                      // 相关性 TR:noFunc
		UpSpeed:      peer.UploadSpeed,       // 上传速度（字节/秒）
		Uploaded:     peer.Uploaded,          // 已上传数据量（字节） TR:noFunc 代理统计
		// 没有验证Uploaded是否可以计算 -> int64(float64(*trt.TotalSize) * peer.Progress)
	}

	return res
}
