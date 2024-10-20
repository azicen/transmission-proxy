package service

import (
	"context"
	"net"
	"strings"
	"transmission-proxy/internal/domain"

	pb "transmission-proxy/api/v2"

	"google.golang.org/protobuf/types/known/emptypb"
)

type TransferService struct {
	pb.UnimplementedTransferServer

	uc *domain.AppUsecase
}

func NewTransferService(uc *domain.AppUsecase) *TransferService {
	return &TransferService{
		uc: uc,
	}
}

// BanPeers Ban peers
func (s *TransferService) BanPeers(ctx context.Context, req *pb.BanPeersRequest) (*emptypb.Empty, error) {
	addresses := strings.Split(req.Peers, "|")
	ips := make([]string, len(addresses))
	for _, addr := range addresses {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			// 如果没有端口部分，则直接使用 addr 作为 host
			host = addr
		}
		ips = append(ips, host)
	}

	err := s.uc.BanIP(ctx, ips)
	if err != nil {
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}
