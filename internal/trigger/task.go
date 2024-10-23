package trigger

import (
	"context"
	"time"

	"transmission-proxy/internal/domain"

	"github.com/go-kratos/kratos/v2/log"
)

type ScheduledTask struct {
	ctx context.Context
	uc  *domain.TorrentUsecase

	d   time.Duration
	log *log.Helper
}

func NewScheduledTask(uc *domain.TorrentUsecase, logger log.Logger) (
	*ScheduledTask, func()) {

	ctx, cancel := context.WithCancel(context.Background())

	task := &ScheduledTask{
		ctx: ctx,
		uc:  uc,
		d:   time.Duration(uc.GetStateRefreshInterval()) * time.Second,
		log: log.NewHelper(logger),
	}

	task.RunStatisticsTask()
	task.RunSaveHistoricalTask()
	task.RunUpTrackerTask()

	return task, cancel
}

// RunStatisticsTask 统计任务
func (t *ScheduledTask) RunStatisticsTask() {
	t.log.Debugf("启动更新状态任务")
	ctx, cancel := context.WithCancel(t.ctx)
	_ = cancel
	ticker := time.NewTicker(t.d)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.log.Debugf("执行更新状态任务")
				err := t.uc.UpPeerData(t.ctx)
				if err != nil {
					t.log.Errorw("err", err)
				}
				break

			case <-ctx.Done():
				t.log.Debugf("更新状态任务结束: %v", t.ctx.Err())
				return
			}
		}
	}()
}

// RunSaveHistoricalTask 保存历史统计任务
func (t *ScheduledTask) RunSaveHistoricalTask() {
	t.log.Debugf("启动定时保存任务")
	ctx, cancel := context.WithCancel(t.ctx)
	_ = cancel
	// 10分钟 写盘一次
	ticker := time.NewTicker(10 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.log.Debugf("执行定时保存任务")
				err := t.uc.SaveStatistics()
				if err != nil {
					t.log.Errorw("err", err)
				}
				break

			case <-ctx.Done():
				// 结束前再保存一次
				err := t.uc.SaveStatistics()
				if err != nil {
					t.log.Errorw("err", err)
				}
				t.log.Debugf("定时保存任务结束: %v", t.ctx.Err())
				return
			}
		}
	}()
}

// RunUpTrackerTask 更新Tracker任务
func (t *ScheduledTask) RunUpTrackerTask() {
	t.log.Debugf("启动更新Tracker任务")
	ctx, cancel := context.WithCancel(t.ctx)
	_ = cancel
	// 每日刷新一次
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				go func() {
					taskCtx, taskCancel := context.WithCancel(ctx)
					defer taskCancel()
					t.log.Debugf("执行更新Tracker任务")
					err := t.uc.UpTrackerList(taskCtx)
					if err != nil {
						t.log.Errorw("err", err)
					}
				}()
				break

			case <-ctx.Done():
				t.log.Debugf("更新Tracker任务结束: %v", t.ctx.Err())
				return
			}
		}
	}()
}
