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
	RegisterFormDataHTTPServer(server, torrentSrv)
	v2.RegisterAppHTTPServer(server, appSrv)
	v2.RegisterAuthHTTPServer(server, authSrv)
	v2.RegisterSyncHTTPServer(server, syncSrv)
	v2.RegisterTorrentHTTPServer(server, torrentSrv)
	v2.RegisterTransferHTTPServer(server, transferSrv)

	return server
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
