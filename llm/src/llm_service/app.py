#工厂模式
import asyncio
from dataclasses import dataclass

from llm_service.config import AppConfig
from llm_service.server.consul_server import ConsulRegistry
from llm_service.server.grpc_server import GrpcServer
import logging as logger
import re
logger =logger.getLogger(__name__)

@dataclass
class Application:
    cfg:AppConfig
    grpc_server:GrpcServer
    consul_server:ConsulRegistry | None 
     

    async def run(self,shutdown_event:asyncio.Event)->None:
       registered =False
       server_task: asyncio.Task[None] | None = None
       shutdown_task: asyncio.Task[bool] | None = None 
       maintenance_task: asyncio.Task[None] | None = None
       try:
            await self.grpc_server.start()#启动grpc服务
            if self.consul_server is not None:
                #注册
                try:
                    #放入线程池
                    await asyncio.to_thread(self.consul_server.register)
                    registered = True
                except Exception:
                    if self.cfg.consul.registration_required:
                        raise
                    logger.exception("Failed to register with consul")
            #阻塞等待        
            #await self.grpc_server.wait()
            # 创建task
            server_task = asyncio.create_task(self.grpc_server.wait())
            shutdown_task = asyncio.create_task(shutdown_event.wait())
            if self.cfg.consul.registration_maintenance.enabled:
                maintenance_task = asyncio.create_task(
                    handle_service_exception(
                        self.consul_server,
                        shutdown_event,
                        interval=duration_seconds(
                            self.cfg.consul.registration_maintenance.interval
                        ),
                    )
                )
            # 等待任意一个任务完成
            done,_= await asyncio.wait({server_task, shutdown_task}, return_when=asyncio.FIRST_COMPLETED)
            if server_task in done:
                await server_task
       finally:
               if not shutdown_event.is_set():
                    shutdown_event.set()
               if maintenance_task is not None:
                   try:
                      await maintenance_task
                   except asyncio.CancelledError:
                       logger.exception("Maintenance task was cancelled")
               #取消任务
               for task in (server_task, shutdown_task):
                   if task is not None and task.done():
                        task.cancel()
               await self.grpc_server.set_health_stop() 

               try:   
                     #先从consul中注销
                    if (registered and self.cfg.consul.deregister_on_shutdown and self.consul_server is not None):
                        await asyncio.to_thread(self.consul_server.deregister)
               except Exception:
                    logger.exception("Failed to deregister with consul")
               finally:
                    await self.grpc_server.stop()#关闭

#构造函数，工厂模式
def create_app(cfg:AppConfig)->Application:
        #pass
        grpc_server = GrpcServer(cfg)
        consul_server:ConsulRegistry | None = None
        if cfg.consul.enabled:
            consul_server = ConsulRegistry(cfg)
        return Application(cfg,grpc_server,consul_server)

#处理服务异常后重新注册
async def handle_service_exception(
        consul_registry: ConsulRegistry |None,
        shutdown_event:asyncio.Event,
        interval: float,
) -> None:
      if consul_registry is None:
        return
      while not shutdown_event.is_set():
        try:
            #放入线程池中执行查询操作是否注册
            exists= await asyncio.to_thread(consul_registry.is_registered)
            if not exists:
                await asyncio.to_thread(consul_registry.register)
        except Exception as e:
            logger.error("Error registering service with Consul: %s", e)

        try:
            await asyncio.wait_for(shutdown_event.wait(), interval)
        except asyncio.TimeoutError:
            continue

def duration_seconds(s: str) -> float:
    """解析带单位时长字符串 10s / 1m / 2h 转为总秒数"""
    units = {
        "s": 1,
        "m": 60,
        "h": 3600,
        "d": 86400,
    }
    pattern = re.compile(r"(?P<val>[\d\.]+)(?P<unit>[smhd])")
    total = 0.0
    for match in pattern.finditer(s):
        val = float(match.group("val"))
        unit = match.group("unit")
        total += val * units[unit]
    return total