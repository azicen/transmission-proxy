package service

import (
	"context"
	"strings"

	pb "transmission-proxy/api/v2"
	"transmission-proxy/internal/domain"
	"transmission-proxy/internal/errors"

	"github.com/go-kratos/kratos/v2/encoding"
	col "github.com/noxiouz/golang-generics-util/collection"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/types/known/emptypb"
)

type TorrentService struct {
	pb.UnimplementedTorrentServer

	uc *domain.TorrentUsecase
}

func NewTorrentService(uc *domain.TorrentUsecase) *TorrentService {
	return &TorrentService{
		uc: uc,
	}
}

// Add 添加种子
func (s *TorrentService) Add(ctx context.Context, req *pb.AddRequest) (res *emptypb.Empty, err error) {
	res = &emptypb.Empty{}
	urls := strings.Split(req.Urls, "\n")
	torrents := make([]*domain.Torrent, 0, len(urls))
	for _, url := range urls {
		torrent := &domain.Torrent{
			URL:    url,
			Path:   col.None[string](),
			Labels: col.None[[]string](),
			Cookie: col.None[string](),
			Paused: false,
		}
		if req.GetSavepath() != "" {
			torrent.Path = col.Some(req.GetSavepath())
		}
		if req.GetCookie() != "" {
			torrent.Cookie = col.Some(req.GetCookie())
		}
		var labels []string
		if req.GetTags() != "" {
			labels = strings.Split(req.GetTags(), ",")
		}
		if len(labels) > 0 {
			torrent.Labels = col.Some(labels)
		}
		if req.GetPaused() != "" {
			if strings.TrimSpace(req.GetPaused()) == "true" {
				torrent.Paused = true
			}
		}
		torrents = append(torrents, torrent)
	}

	err = s.uc.Add(ctx, torrents)
	return
}

// GetInfo 获取种子列表
func (s *TorrentService) GetInfo(ctx context.Context, req *pb.GetInfoRequest) (*httpbody.HttpBody, error) {
	filter := domain.TorrentFilter{
		Status: col.None[string](),
		Label:  col.None[string](),
		Hashes: col.None[[]string](),
	}
	if req.GetFilter() != "" {
		filter.Status = col.Some(req.GetFilter())
	}
	if req.GetTag() != "" {
		filter.Label = col.Some(req.GetTag())
	}
	if req.GetHashes() != "" {
		hashes := strings.Split(req.GetHashes(), "|")
		filter.Hashes = col.Some(hashes)
	}

	qbTorrents, err := s.uc.GetTorrentList(ctx, filter)
	if err != nil {
		return nil, err
	}

	if !qbTorrents.HasValue() {
		return &httpbody.HttpBody{Data: make([]byte, 0)}, nil
	}

	// 获取编解码器并编码json, qb需要返回一个纯数组`[{xxx},{xxx},...]`
	json, err := encoding.GetCodec("json").Marshal(qbTorrents.Value())
	if err != nil {
		return &httpbody.HttpBody{Data: make([]byte, 0)}, nil
	}
	return &httpbody.HttpBody{Data: json}, nil
}

// GetProperties 获取种子属性属性
func (s *TorrentService) GetProperties(ctx context.Context, req *pb.GetPropertiesRequest) (
	*pb.GetPropertiesResponse, error) {

	qbt, err := s.uc.GetTorrentProperties(ctx, req.GetHash())
	if err != nil {
		return nil, err
	}
	if !qbt.HasValue() {
		return nil, errors.ResourceNotExist("Torrent hash was not found")
	}

	return qbt.Value(), nil
}
