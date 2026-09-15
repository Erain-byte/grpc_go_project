package consul

import (
	"context"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
	"sync"
	"time"
)

// registerAndRemember 保存成功注册的模板，补注册沿用原 TLS 和健康检查参数。
func (r *ConsulRegistry) registerAndRemember(registration *api.AgentServiceRegistration) error {
	if err := r.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}
	r.registrationMu.Lock()
	defer r.registrationMu.Unlock()
	if r.registrations == nil {
		r.registrations = make(map[string]*api.AgentServiceRegistration)
	}
	r.registrations[registration.ID] = registration
	return nil
}

// reconcileRegistrations 查询失败不视为缺失，已有注册不重复写入。
func (r *ConsulRegistry) reconcileRegistrations(ctx context.Context) error {
	services, err := r.client.Agent().ServicesWithFilterOpts("", (&api.QueryOptions{}).WithContext(ctx))
	if err != nil {
		return err
	}
	r.registrationMu.Lock()
	templates := make([]*api.AgentServiceRegistration, 0, len(r.registrations))
	for _, registration := range r.registrations {
		templates = append(templates, registration)
	}
	r.registrationMu.Unlock()
	for _, registration := range templates {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, exists := services[registration.ID]; exists {
			continue
		}
		// 新注册仍由 Agent 检查，不强行标记 passing。
		if err := r.client.Agent().ServiceRegisterOpts(registration, (api.ServiceRegisterOpts{}).WithContext(ctx)); err != nil {
			return err
		}
	}
	return nil
}

// StartRegistrationMaintenance 返回取消并等待退出的函数，必须先停止维护再注销。
func (r *ConsulRegistry) StartRegistrationMaintenance(parent context.Context, log *zap.Logger) func() {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		delay := 30 * time.Second
		for {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			requestCtx, stopRequest := context.WithTimeout(ctx, 5*time.Second)
			err := r.reconcileRegistrations(requestCtx)
			stopRequest()
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				log.Warn("Consul registration maintenance failed", zap.Error(err))
				// 指数退避，最长两分钟，避免依赖故障时频繁请求。
				delay *= 2
				if delay > 2*time.Minute {
					delay = 2 * time.Minute
				}
			} else {
				delay = 30 * time.Second
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { cancel(); <-done }) }
}
