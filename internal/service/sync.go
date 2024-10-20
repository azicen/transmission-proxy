package service

import (
	"context"
	col "github.com/noxiouz/golang-generics-util/collection"

	"transmission-proxy/internal/domain"

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
		Status: col.None[string](),
		Tag:    col.None[string](),
		Hashes: col.None[[]string](),
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
	*pb.GetTorrentPeersResponse, error) {

	peers, err := s.uc.GetTorrentPeers(ctx, req.GetHash())
	if err != nil {
		return nil, err
	}
	qbDate := &pb.GetTorrentPeersResponse{
		FullUpdate: true, // 是否为完整更新 TR:noFunc
		ShowFlags:  true, // 是否显示标志 TR:noFunc
		Rid:        req.GetRid(),
	}

	if peers.HasValue() {
		qbDate.Peers = peers.Value()
	}
	return qbDate, nil
}
