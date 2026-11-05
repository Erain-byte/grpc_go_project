package consul

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func validateRegistration(registration *api.AgentServiceRegistration) error {
	if registration == nil || registration.ID == "" || registration.Name == "" {
		return fmt.Errorf("Consul registration identity is incomplete")
	}
	if registration.Check == nil || registration.Check.GRPC == "" {
		return fmt.Errorf("Consul gRPC health check is missing")
	}
	return nil
}

func (r *ConsulRegistry) registerAndRemember(registration *api.AgentServiceRegistration) error {
	if err := validateRegistration(registration); err != nil {
		return err
	}
	if err := r.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}
	r.registrationMu.Lock()
	r.registrations[registration.ID] = registration
	r.registrationMu.Unlock()
	return nil
}

func (r *ConsulRegistry) reconcileRegistrations(ctx context.Context) error {
	r.registrationMu.Lock()
	templates := make([]*api.AgentServiceRegistration, 0, len(r.registrations))
	for _, registration := range r.registrations {
		templates = append(templates, registration)
	}
	r.registrationMu.Unlock()

	for _, registration := range templates {
		if err := ctx.Err(); err != nil {
			return err
		}
		services, err := r.client.Agent().ServicesWithFilterOpts(
			fmt.Sprintf(`Service == %q`, registration.Name),
			(&api.QueryOptions{}).WithContext(ctx),
		)
		if err != nil {
			return err
		}
		if _, exists := services[registration.ID]; exists {
			continue
		}
		if err := r.client.Agent().ServiceRegisterOpts(registration, (api.ServiceRegisterOpts{}).WithContext(ctx)); err != nil {
			return err
		}
	}
	return nil
}

// StartRegistrationMaintenance 定期检查本实例是否仍在Agent中，缺失时自动补注册。
// 返回函数会取消后台任务并等待goroutine结束，注销服务前必须先调用它。
func (r *ConsulRegistry) StartRegistrationMaintenance(parent context.Context, logger *zap.Logger) func() {
	if !r.config.RegistrationMaintenance.Enabled {
		return func() {}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	interval, _ := time.ParseDuration(r.config.RegistrationMaintenance.Interval)
	maxBackoff, _ := time.ParseDuration(r.config.RegistrationMaintenance.MaxBackoff)
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if maxBackoff < interval {
		maxBackoff = 2 * time.Minute
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		delay := interval
		for {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			requestCtx, stopRequest := context.WithTimeout(ctx, r.registrationRequestTimeout())
			err := r.reconcileRegistrations(requestCtx)
			stopRequest()
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				logger.Warn("Consul registration maintenance failed", zap.Error(err))
				delay *= 2
				if delay > maxBackoff {
					delay = maxBackoff
				}
				continue
			}
			delay = interval
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}

func (r *ConsulRegistry) registrationRequestTimeout() time.Duration {
	timeout, _ := time.ParseDuration(r.config.CheckTimeout)
	if timeout < 5*time.Second {
		return 5 * time.Second
	}
	return timeout
}
