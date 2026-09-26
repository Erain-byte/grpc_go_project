# LLM 服务

`llm` 是项目中的 Python 内部服务，通过 gRPC 接收 Gateway 转发的请求。第一阶段先完成普通对话和流式对话闭环，之后再扩展会话存储、Tool Calling、MCP 与 RAG。

## 目录结构

详细的文件放置、当前实现状态和调用流程见 [目录结构与文件职责说明](目录结构与文件职责说明.md)。
中间件目录是 `src/llm_service/middleware/`，目前只有 `__init__.py`，尚未实现 gRPC Interceptor。

```text
llm/
|-- config/
|   `-- llm.yaml                 非敏感启动配置
|-- scripts/
|   `-- generate_proto.ps1       生成 Python gRPC 代码
|-- src/
|   |-- llm/v1/                  Proto 生成包
|   `-- llm_service/
|       |-- main.py              进程入口
|       |-- app.py               依赖初始化与生命周期
|       |-- config.py            配置模型与加载
|       |-- server/              gRPC Server、Handler 和 Health 挂载
|       |-- handler/             Proto 请求、响应适配
|       |-- service/             对话与模型调用编排
|       |-- domain/              领域对象和接口
|       |-- repository/          会话及消息数据访问
|       |-- infrastructure/
|       |   |-- model/           模型供应商适配器
|       |   |-- database/        数据库访问
|       |   |-- redis/           会话缓存与分布式状态
|       |   `-- consul/          注册、发现与动态配置
|       |-- middleware/          gRPC 鉴权、错误和 Request ID
|       |-- observability/       Logs、Metrics 与 Traces
|       |-- rag/                 Embedding 与向量检索
|       `-- mcp/                 MCP Client 与工具注册
|-- tests/
|   |-- unit/
|   `-- integration/
|-- .env.example
|-- requirements.txt            Python 依赖清单
`-- pyproject.toml
```

## 安装依赖

在项目根目录执行：

```powershell
python -m venv .\llm\.venv
.\llm\.venv\Scripts\Activate.ps1
python -m pip install --upgrade pip
python -m pip install -r .\llm\requirements.txt
```

## 启动配置读取

`src/llm_service/config.py` 提供 `load_config()`。默认读取 `llm/config/llm.yaml`，不依赖 PowerShell 当前目录；也可以传入配置文件路径。此加载函数尚需在 `app.py` 的启动流程中调用。

```python
from llm_service.config import duration_seconds, load_config

cfg = load_config()
listen_address = f"{cfg.host}:{cfg.grpc_port}"
consul_address = cfg.consul.address
request_timeout = duration_seconds(cfg.consul.request_timeout)
```

时间保留 `5s`、`2m`、`100ms` 等单单位字符串，需要数值超时时用 `duration_seconds()` 转为秒。拼错字段、非法端口、非正时长以及不一致的注册配置会抛出 `ConfigError`。启用 TLS 时要求证书和私钥文件存在；相对路径以 YAML 文件目录为基准，证书内容由后续 TLS 初始化校验。

Consul ACL Token 只读取进程环境变量 `LLM_CONSUL_TOKEN`，不允许写入 YAML，也不会包含在配置的 `repr()` 或 `model_dump()` 中。调用 Consul 客户端时通过 `cfg.consul.token.get_secret_value()` 获取；不要打印该值。当前没有自动读取 `.env` 文件或热更新。

## 分层约束

- `handler` 只做 gRPC 协议转换，不直接访问数据库或模型 SDK。
- `service` 编排业务流程，通过领域接口使用外部能力。
- `repository` 负责会话和消息持久化。
- `infrastructure` 实现模型、数据库、Redis 和 Consul 接口。
- `domain` 不依赖 gRPC、数据库或具体模型厂商。
- 密码、API Key 和 Token 只从环境变量或 Secret Manager 读取。

## Proto 生成

在项目根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\llm\scripts\generate_proto.ps1
```

生成文件位于 `llm/src/llm/v1`。现有协议源文件为 `proto/llm/v1/llm.proto`。

## 推荐开发顺序

```text
配置加载 -> Proto 生成 -> gRPC Server/健康检查 -> Mock Provider
-> Chat -> StreamChat -> Consul -> OpenTelemetry -> 会话存储
-> Tool Calling -> MCP -> RAG/向量数据库
```

当前只建立工程边界，尚未实现真实模型调用。
