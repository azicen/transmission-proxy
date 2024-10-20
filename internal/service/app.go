package service

import (
	"context"

	pb "transmission-proxy/api/v2"
	"transmission-proxy/internal/domain"

	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type AppService struct {
	pb.UnimplementedAppServer

	uc *domain.AppUsecase
}

func NewAppService(uc *domain.AppUsecase) *AppService {
	return &AppService{
		uc: uc,
	}
}

// GetVersion 获取应用程序版本
func (s *AppService) GetVersion(ctx context.Context, _ *emptypb.Empty) (*wrapperspb.StringValue, error) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Content-Type", "text/plain")
	}
	return &wrapperspb.StringValue{Value: "v4.6.6.10"}, nil
}

// GetPreferences 获取应用程序首选项
func (s *AppService) GetPreferences(ctx context.Context, _ *emptypb.Empty) (*pb.GetPreferencesResponse, error) {
	qbd, err := s.uc.GetPreferences(ctx)
	if err != nil {
		return nil, err
	}
	return qbd, nil
}

// SetPreferences 设置应用程序首选项
func (s *AppService) SetPreferences(ctx context.Context, req *pb.SetPreferencesRequest) (
	*emptypb.Empty, error) {

	err := s.uc.SetPreferences(ctx, req)
	if err != nil {
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}
