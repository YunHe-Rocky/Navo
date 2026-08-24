package main

import (
	"fmt"
	"strings"
)

type routeBenchmarkApplication struct {
	snapshot  func() (Dashboard, error)
	selectOne func(string) error
	benchmark func() (ProxyBenchmark, error)
}

func (application routeBenchmarkApplication) Run(outboundID string) (result ProxyBenchmark, err error) {
	requested := strings.TrimSpace(outboundID)
	if requested == "" {
		return ProxyBenchmark{}, fmt.Errorf("测速线路不能为空")
	}
	snapshot, err := application.snapshot()
	if err != nil {
		return ProxyBenchmark{}, err
	}
	previous := strings.TrimSpace(snapshot.Runtime.SelectedID)
	if previous == "" {
		previous = strings.TrimSpace(snapshot.Runtime.ActiveID)
	}
	targetChanged := requested != previous
	if targetChanged && snapshot.Capture.CommittedMode != "off" {
		return ProxyBenchmark{}, fmt.Errorf("系统代理或 TUN 接管期间只能测速当前线路；请先断开连接，避免临时切线影响正在进行的流量")
	}
	if targetChanged {
		if err := application.selectOne(requested); err != nil {
			return ProxyBenchmark{}, err
		}
		if previous != "" {
			defer func() {
				if restoreErr := application.selectOne(previous); restoreErr != nil {
					if err == nil {
						err = fmt.Errorf("测速后恢复线路失败: %w", restoreErr)
					} else {
						err = fmt.Errorf("%v; 测速后恢复线路失败: %w", err, restoreErr)
					}
				}
			}()
		}
	}
	return application.benchmark()
}

func (a *App) RunRouteBenchmark(outboundID string) (ProxyBenchmark, error) {
	application := routeBenchmarkApplication{
		snapshot:  a.GetDashboard,
		selectOne: a.SelectRoute,
		benchmark: a.RunProxyBenchmark,
	}
	return application.Run(outboundID)
}
