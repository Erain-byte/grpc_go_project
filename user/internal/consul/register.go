package consul

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

// 本脚本完成服务异常后自动注册
// 第一次注册时需要保存的模板，后续只做补注册
// 严格验证
func validateRegistration(registration *api.AgentServiceRegistration) error {
	if registration == nil {
		return fmt.Errorf("registration is nil")
	}
	if registration.ID == "" {
		return fmt.Errorf("registration.ID is empty")
	}
	if registration.Name == "" {
		return fmt.Errorf("registration.Name is empty")
	}
	if registration.Check == nil {
		return fmt.Errorf("registration.Check is nil")
	}
	hashHttp := registration.Check.HTTP != ""
	hashGrpc := registration.Check.GRPC != ""
	if hashGrpc == hashHttp {
		return fmt.Errorf("registration.Check.HTTP and registration.Check.GRPC can not be both empty")
	}
	return nil
}

// registerAndRemember 保存成功注册的模板，补注册沿用原 TLS 和健康检查参数。
func (c *ConsulRegistry) registerAndRemember(registration *api.AgentServiceRegistration) error {
	if err := validateRegistration(registration); err != nil {
		return err
	}
	if err := c.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}
	c.registrationMu.Lock()
	defer c.registrationMu.Unlock()
	if c.registrations == nil {
		c.registrations = make(map[string]*api.AgentServiceRegistration)
	}
	c.registrations[registration.ID] = registration
	return nil
}

// reconcileRegistrations 查询失败不视为缺失，已有注册不重复写入
func (c *ConsulRegistry) reconcileRegistrations(ctx context.Context) error {
	c.registrationMu.Lock()
	defer c.registrationMu.Unlock()
	templates := c.registrations
	for _, registration := range templates {
		service, err := c.client.Agent().ServicesWithFilterOpts(
			fmt.Sprintf(`Service == %q`, registration.Name),
			//查询失败不失为缺失，已有注册不重复写入
			(&api.QueryOptions{}).WithContext(ctx),
		)
		if err != nil {
			return err
		}
		if _, ok := service[registration.ID]; ok {
			continue
		}
		//在consul已经消失的，直接使用本地模板重新写入有健康检查完成passing
		if err := c.client.Agent().ServiceRegisterOpts(registration, (api.ServiceRegisterOpts{}).WithContext(ctx)); err != nil {
			return err
		}
	}
	return nil
}

// StartRegistrationMaintenance定时补注册
func (c *ConsulRegistry) StartRegistrationMaintenance(parent context.Context, log *zap.Logger) func() {
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
			//开始补注册
			requestCtx, requestCancel := context.WithTimeout(ctx, 5*time.Second)
			requestCancel()
			if ctx.Err() != nil {
				return
			}
			err := c.reconcileRegistrations(requestCtx)
			if err != nil {
				log.Warn("Consul registration maintenance failed", zap.Error(err))
				//指数退避
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
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}
