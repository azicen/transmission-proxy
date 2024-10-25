package trigger

import (
	"context"
	"fmt"

	v2 "transmission-proxy/api/v2"
	"transmission-proxy/conf"
	"transmission-proxy/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(
	bootstrap *conf.Bootstrap,
	appSrv *service.AppService,
	authSrv *service.AuthService,
	syncSrv *service.SyncService,
	torrentSrv *service.TorrentService,
	transferSrv *service.TransferService,
	logger log.Logger,
) *http.Server {
	config := bootstrap.GetTrigger()
	opts := []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
		),
	}
	opts = append(opts, http.Network("tcp"))
	if config.Http.Host != "" || config.Http.Port != 0 {
		opts = append(opts, http.Address(fmt.Sprintf("%s:%v", config.Http.Host, config.Http.Port)))
	}
	if config.Http.Timeout != nil {
		opts = append(opts, http.Timeout(config.Http.Timeout.AsDuration()))
	}

	server := http.NewServer(opts...)
	RegisterPingHTTPServer(server, appSrv)
	RegisterFormDataHTTPServer(server, torrentSrv)
	RegisterDeficienciesContentTypeHTTPServer(server, authSrv)
	v2.RegisterAppHTTPServer(server, appSrv)
	v2.RegisterAuthHTTPServer(server, authSrv)
	v2.RegisterSyncHTTPServer(server, syncSrv)
	v2.RegisterTorrentHTTPServer(server, torrentSrv)
	v2.RegisterTransferHTTPServer(server, transferSrv)

	return server
}

func RegisterPingHTTPServer(s *http.Server, srv v2.AppHTTPServer) {
	r := s.Route("/")
	r.HEAD("", func(ctx http.Context) error { return ctx.Result(200, nil) })
	r.GET("", AppPingHttpHandler(srv))
}

func AppPingHttpHandler(srv v2.AppHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in emptypb.Empty
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, v2.OperationAppPing)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.Ping(ctx, req.(*emptypb.Empty))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*wrapperspb.StringValue)
		return ctx.Result(200, reply)
	}
}

func RegisterDeficienciesContentTypeHTTPServer(s *http.Server, srv v2.AuthHTTPServer) {
	// 为什么会存在缺少ContentType头的HTTP请求呢？
	r := s.Route("/")
	r.POST("/api/v2/auth/logout", AuthLogoutHttpHandler(srv))
}

func AuthLogoutHttpHandler(srv v2.AuthHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		// 保证默认有ContentType
		ct := ctx.Request().Header.Get("Content-Type")
		if ct == "" {
			ctx.Request().Header.Set("Content-Type", "application/json")
		}

		var in emptypb.Empty
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, v2.OperationAuthLogout)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.Logout(ctx, req.(*emptypb.Empty))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*wrapperspb.StringValue)
		return ctx.Result(200, reply)
	}
}

func RegisterFormDataHTTPServer(s *http.Server, srv v2.TorrentHTTPServer) {
	r := s.Route("/")
	r.POST("/api/v2/torrents/add", TorrentAddHttpHandler(srv))
}

func TorrentAddHttpHandler(srv v2.TorrentHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		in := v2.AddRequest{}
		req := ctx.Request()
		in.Urls = req.FormValue("urls")
		path := req.FormValue("savepath")
		in.Savepath = &path
		cookie := req.FormValue("cookie")
		in.Cookie = &cookie
		tags := req.FormValue("tags")
		in.Tags = &tags
		paused := req.FormValue("paused")
		in.Paused = &paused
		http.SetOperation(ctx, v2.OperationTorrentAdd)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.Add(ctx, req.(*v2.AddRequest))
		})
		_, err := h(ctx, &in)
		if err != nil {
			return err
		}
		return ctx.Result(200, []byte{})
	}
}
