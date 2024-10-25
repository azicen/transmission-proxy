package service

import (
	"context"
	"strings"

	pb "transmission-proxy/api/v2"
	"transmission-proxy/internal/domain"

	"github.com/go-kratos/kratos/v2/encoding"
	"github.com/go-kratos/kratos/v2/transport"
	col "github.com/noxiouz/golang-generics-util/collection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const html = `<!DOCTYPE html>
<html lang="zh">

<head>
    <meta charset="UTF-8" />
    <meta name="color-scheme" content="light dark" />
    <meta name="description" content="qBittorrent WebUI">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">

    <title>qBittorrent WebUI</title>

    <link rel="icon" type="image/png" href="images/qbittorrent32.png" />
    <link rel="icon" type="image/svg+xml" href="images/qbittorrent-tray.svg" />
    <link rel="stylesheet" type="text/css" href="css/login.css?v=83s5ed" />
    <noscript>
        <link rel="stylesheet" type="text/css" href="css/noscript.css?v=83s5ed" />
    </noscript>

    <script defer src="scripts/login.js?locale=zh&v=83s5ed"></script>
</head>

<body>
    <noscript id="noscript">
        <h1>JavaScript 是必需的！要让 WebUI 正确工作，你必须启用 JavaScript</h1>
    </noscript>
    <div id="main">
        <h1>qBittorrent WebUI</h1>
        <div id="logo" class="col">
            <img src="images/qbittorrent-tray.svg" alt="qBittorrent logo" />
        </div>
        <div id="formplace" class="col">
            <form id="loginform">
                <div class="row">
                    <label for="username">用户名</label><br />
                    <input type="text" id="username" name="username" autocomplete="username" autofocus required />
                </div>
                <div class="row">
                    <label for="password">密码</label><br />
                    <input type="password" id="password" name="password" autocomplete="current-password" required />
                </div>
                <div class="row">
                    <input type="submit" id="loginButton" value="登录" />
                </div>
            </form>
        </div>
        <div id="error_msg"></div>
    </div>
</body>

</html>
`

type AppService struct {
	pb.UnimplementedAppServer

	uc *domain.AppUsecase
}

func NewAppService(uc *domain.AppUsecase) *AppService {
	return &AppService{
		uc: uc,
	}
}

func (s *AppService) Ping(_ context.Context, _ *emptypb.Empty) (*wrapperspb.StringValue, error) {
	return &wrapperspb.StringValue{Value: html}, nil
}

// GetVersion 获取应用程序版本
func (s *AppService) GetVersion(ctx context.Context, _ *emptypb.Empty) (*wrapperspb.StringValue, error) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Content-Type", "text/plain")
	}
	return &wrapperspb.StringValue{Value: "v4.6.6.10"}, nil
}

// GetWebAPIVersion 获取WebAPI版本
func (s *AppService) GetWebAPIVersion(ctx context.Context, _ *emptypb.Empty) (*wrapperspb.StringValue, error) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Content-Type", "text/plain")
	}
	return &wrapperspb.StringValue{Value: "2.8.3"}, nil
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

	pre := domain.Preferences{
		ListenPort: col.None[int32](),
		BanList:    col.None[[]string](),
	}

	json := req.GetJson()
	codec := encoding.GetCodec("json")
	var v pb.SetPreferencesRequest_Json
	err := codec.Unmarshal([]byte(json), &v)
	if err != nil {
		return &emptypb.Empty{}, err
	}

	if v.GetListenPort() != 0 {
		pre.ListenPort = col.Some(v.GetListenPort())
	}

	ips := strings.Split(v.GetBanned_IPs(), "\n")
	if len(ips) > 0 {
		pre.BanList = col.Some(ips)
	}

	err = s.uc.SetPreferences(ctx, &pre)
	if err != nil {
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}
