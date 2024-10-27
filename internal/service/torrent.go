package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"

	pb "transmission-proxy/api/v2"
	"transmission-proxy/internal/domain"
	"transmission-proxy/internal/errors"

	"github.com/go-kratos/kratos/v2/encoding"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http"
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
	urlsStr := strings.Split(req.Urls, "\n")
	urls := make([]string, 0, len(urlsStr))

	for _, str := range urlsStr {
		if str == "" {
			continue
		}
		parseURL, err := url.Parse(str)
		if err != nil {
			continue
		}
		urls = append(urls, parseURL.String())
	}

	if len(urls) == 0 {
		// 没有url，传输的种子文件
		httpCTX, ok := ctx.(http.Context)
		if !ok {
			return
		}
		httpReq := httpCTX.Request()

		const prefix = "torrent__"
		i := 0
		for {
			fileHeader, exist := httpReq.MultipartForm.File[fmt.Sprintf("%s%d", prefix, i)]
			if !exist {
				break
			}
			urlStr, err := s.cacheTorrent(ctx, fileHeader[0])
			if err != nil {
				return nil, err
			}
			urls = append(urls, urlStr)
			i = i + 1
		}
	}

	torrents := make([]*domain.DownloadTorrent, 0, len(urls))
	for _, urlStr := range urls {
		torrent := &domain.DownloadTorrent{
			URL:      urlStr,
			Path:     col.None[string](),
			Labels:   col.None[[]string](),
			Category: col.None[string](),
			Cookie:   col.None[string](),
			Paused:   false,
		}
		if req.GetSavepath() != "" {
			torrent.Path = col.Some(req.GetSavepath())
		}
		if req.GetCookie() != "" {
			torrent.Cookie = col.Some(req.GetCookie())
		}
		if req.GetCategory() != "" {
			torrent.Category = col.Some(req.GetCategory())
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

func (s *TorrentService) cacheTorrent(ctx context.Context, fileHeader *multipart.FileHeader) (
	filename string, err error) {

	file, err := fileHeader.Open()
	if err != nil {
		return
	}
	defer file.Close()
	fileData, err := io.ReadAll(file)
	if err != nil {
		return
	}
	filename, err = s.uc.CacheTmpTorrentFile(ctx, fileData)
	return
}

// GetInfo 获取种子列表
func (s *TorrentService) GetInfo(ctx context.Context, req *pb.GetInfoRequest) (res *httpbody.HttpBody, err error) {
	res = &httpbody.HttpBody{Data: make([]byte, 0)}
	filter := domain.TorrentFilter{
		Status:   col.None[string](),
		Category: col.None[string](),
		Label:    col.None[string](),
		Hashes:   col.None[[]string](),
	}
	if req.GetFilter() != "" {
		filter.Status = col.Some(req.GetFilter())
	}
	if req.GetCategory() != "" {
		filter.Category = col.Some(req.GetCategory())
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
		return
	}

	if !qbTorrents.HasValue() {
		return
	}

	// 获取编解码器并编码json, qb需要返回一个纯数组`[{xxx},{xxx},...]`
	// 直接编码[]any导致json默认值被省略，需要手动拼接
	data := make([]byte, 0, len(qbTorrents.Value())*2048)
	data = append(data, '[')
	codec := encoding.GetCodec("json")
	for _, torrent := range qbTorrents.Value() {
		var json []byte
		json, err = codec.Marshal(torrent)
		if err != nil {
			return
		}
		data = append(data, json...)
		data = append(data, ',')
	}
	if data[len(data)-1] == ',' {
		data[len(data)-1] = ']'
	} else {
		data = append(data, ']')
	}

	return &httpbody.HttpBody{Data: data}, nil
}

// GetProperties 获取种子属性属性
func (s *TorrentService) GetProperties(ctx context.Context, req *pb.GetPropertiesRequest) (
	*pb.GetPropertiesResponse, error) {

	qbt, err := s.uc.GetTorrentProperties(ctx, req.GetHash())
	if err != nil {
		return nil, err
	}
	if !qbt.HasValue() {
		return nil, errors.ResourceNotExist("DownloadTorrent hash was not found")
	}

	return qbt.Value(), nil
}

// Download 下载
// 用于给tr提供临时下载使用
func (s *TorrentService) Download(ctx context.Context, req *pb.DownloadRequest) (res *emptypb.Empty, err error) {
	res = &emptypb.Empty{}

	data, err := s.uc.GetTmpTorrentFile(ctx, req.GetFilename())
	if err != nil {
		return
	}

	disposition := fmt.Sprintf("attachment; filename=%s", req.GetFilename())

	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Content-Type", "application/x-bittorrent")
		tr.ReplyHeader().Set("Accept-Ranges", "bytes")
		tr.ReplyHeader().Set("Content-Disposition", disposition)
		tr.ReplyHeader().Set("Content-Length", strconv.Itoa(len(data)))
	}

	httpCTX, ok := ctx.(http.Context)
	if !ok {
		return
	}
	_, err = httpCTX.Response().Write(data)
	return
}
