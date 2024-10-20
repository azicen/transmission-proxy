package service

import (
	"context"
	pb "transmission-proxy/api/v2"
	"transmission-proxy/internal/domain"

	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/protobuf/types/known/emptypb"
)

type AuthService struct {
	pb.UnimplementedAuthServer

	uc *domain.TorrentUsecase
}

func NewAuthService(uc *domain.TorrentUsecase) *AuthService {
	return &AuthService{
		uc: uc,
	}
}

// Login 登陆
func (s *AuthService) Login(ctx context.Context, req *pb.AuthRequest) (*emptypb.Empty, error) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Set-Cookie", "SID=uTgftNGsVl4afcI4ev7riOJavOyNKZnb; HttpOnly; path=/")
	}
	return &emptypb.Empty{}, nil
}
