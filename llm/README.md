# LLM 服务

`llm` 是项目中的 Python 内部服务，通过 gRPC 接收 Gateway 转发的请求。第一阶段先完成普通对话和流式对话闭环，之后再扩展会话存储、Tool Calling、MCP 与 RAG。

## 目录结构

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
|       |-- server/              gRPC Server 与服务注册
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
