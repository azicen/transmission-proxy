package main

import (
	"flag"
	"os"

	"transmission-proxy/conf"
	_ "transmission-proxy/encoding"
	"transmission-proxy/internal/trigger"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "github.com/azicen/kratos-extension/encoding"
	_ "github.com/joho/godotenv/autoload"
	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name 服务名
	Name = "transmission-proxy"
	// Version 编译时设置的版本号
	Version string
	// 配置文件目录
	flagConf string

	guid, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagConf, "conf", "./data/conf", "config path, eg: -conf config.toml")
}

func newApp(logger log.Logger, hs *http.Server, _ *trigger.ScheduledTask) *kratos.App {
	appInstance := kratos.New(
		kratos.ID(guid),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			hs,
		),
	)

	return appInstance
}

func main() {
	flag.Parse()

	bc, bcCleanup, err := conf.LoadConf(flagConf)
	if err != nil {
		panic(err)
	}
	defer bcCleanup()

	serviceConf := bc.GetService()
	logLevel := log.ParseLevel(serviceConf.GetLogLevel())
	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.Caller(5),
	)
	logger = log.NewFilter(logger, log.FilterLevel(logLevel))
	log.NewHelper(logger).Debugw("guid", guid, "version", Version)

	app, cleanup, err := initApp(bc, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}
