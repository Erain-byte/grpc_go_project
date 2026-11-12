# PostgreSQL User 与 LLM 分表设计和操作指南

本文记录 User 和 LLM 服务的分表边界、表结构示例、应用路由方式及日常运维步骤。Docker Compose 只负责运行 PostgreSQL；所有业务表必须通过版本化迁移创建。

## 1. 总体约定

| 服务 | 数据库 | 本地地址 | 分表方式 |
|---|---|---|---|
| User | `user_service` | `127.0.0.1:5432` | 固定 8 张物理用户表 |
| LLM | `llm_service` | `127.0.0.1:5432` | PostgreSQL 按月原生分区 |

共同约定：

- 业务 ID 使用 Snowflake `BIGINT`。
- 时间统一使用 `TIMESTAMPTZ`，应用内部和数据库连接统一时区策略。
- 业务代码不能随意拼接表名，表路由集中在 Repository 或专用 Router 中。
- 迁移文件进入 Git，并记录明确的版本号、执行顺序和回滚方案。
- 服务之间不建立跨数据库外键，只保存对方的业务 ID。

## 2. User：固定 8 张用户表

### 2.1 物理结构

```text
user_identity_index
users_00
users_01
users_02
users_03
users_04
users_05
users_06
users_07
```

用户主表投入使用后，逻辑表数量固定为 8。不能直接把取模基数改成 16，否则原有用户会被路由到错误的表。

### 2.2 路由规则

第一版采用容易理解且跨语言一致的规则：

```go
const userTableCount uint64 = 8

func UserTableIndex(userID uint64) uint64 {
	return userID % userTableCount
}

func UserTableName(userID uint64) string {
	return fmt.Sprintf("users_%02d", UserTableIndex(userID))
}
```

表名只能从固定白名单中选择，不能把 HTTP、gRPC 或配置中心传入的任意字符串作为表名。

Snowflake ID 的低位通常包含节点编号和序列号。正式投入前需要生成足够多的 ID，统计 8 个分片的分布。如果具体 Snowflake 实现导致低位明显偏斜，应在第一批生产数据写入前调整路由算法；一旦写入正式数据就不能直接改变算法。

### 2.3 用户表模板

以下为结构示例，最终字段以 User protobuf 和领域模型为准：

```sql
CREATE TABLE users_00 (
    id                BIGINT PRIMARY KEY,
    keycloak_subject  UUID NOT NULL,
    username          VARCHAR(64) NOT NULL,
    display_name      VARCHAR(128) NOT NULL DEFAULT '',
    status            SMALLINT NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX uk_users_00_keycloak_subject
    ON users_00 (keycloak_subject);

CREATE INDEX idx_users_00_created_at
    ON users_00 (created_at DESC);
```

迁移程序使用相同模板循环创建 `users_00` 到 `users_07`。不要复制八份由人工分别维护的模型定义。

### 2.4 身份路由索引

Gateway 和内部服务从 JWT 获得的是 Keycloak `sub`，它不能直接通过 `user_id % 8` 定位用户表。因此保留一个不分表的身份路由索引：

```sql
CREATE TABLE user_identity_index (
    keycloak_subject UUID PRIMARY KEY,
    user_id          BIGINT NOT NULL UNIQUE,
    table_index      SMALLINT NOT NULL CHECK (table_index BETWEEN 0 AND 7),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

查询链路：

```text
JWT sub
  → user_identity_index
  → user_id + table_index
  → users_00 ... users_07
```

写入用户与身份索引时，两者处于同一个 PostgreSQL 实例，可以使用一个本地事务：

```text
BEGIN
  INSERT users_XX
  INSERT user_identity_index
COMMIT
```

如果以后把不同用户表迁移到不同数据库，本地事务将失效，需要升级为 Outbox、幂等消费和补偿任务。

### 2.5 User 查询规则

- 已知 `user_id`：直接计算目标表。
- 只有 Keycloak `sub`：先查 `user_identity_index`。
- 后台跨分片列表：分别查询 8 张表，再在服务层合并；这类接口必须分页并限制查询范围。
- 用户总数和状态统计：优先维护统计表或离线聚合，避免每次实时扫描 8 张表。

### 2.6 User 后续分库

扩容时保持 8 个逻辑表编号不变，按整张表迁移：

```text
扩容前：
user-db-01 → users_00 ... users_07

扩容后：
user-db-01 → users_00 ... users_03
user-db-02 → users_04 ... users_07
```

路由分为两步：

```text
user_id % 8 → table_index
table_index → database
```

第二步由配置管理，因此迁移整表时不改变用户 ID 的取模规则。

## 3. LLM：按年月进行 PostgreSQL 原生分区

### 3.1 数据分类

保持普通单表：

- `conversations`
- `prompt_templates`
- `model_configs`
- 初期的 `feedback`

按 `created_at` 进行月份分区：

- `conversation_messages`
- `llm_requests`
- `llm_usage`

物理分区带完整年份和月份：

```text
conversation_messages_202610
conversation_messages_202611
conversation_messages_default
```

不采用循环复用的 `_01` 到 `_12`，因为循环表会混合多个年份的数据，归档时无法直接删除整个历史月份。

### 3.2 会话主表

```sql
CREATE TABLE conversations (
    id               BIGINT PRIMARY KEY,
    user_id          BIGINT NOT NULL,
    title            VARCHAR(255) NOT NULL DEFAULT '',
    status           SMALLINT NOT NULL DEFAULT 1,
    started_at       TIMESTAMPTZ NOT NULL,
    last_message_at  TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_conversations_user_last_message
    ON conversations (user_id, last_message_at DESC, id DESC);
```

`started_at` 和 `last_message_at` 用于确定消息查询的时间范围，让 PostgreSQL 能够裁剪无关月份分区。

### 3.3 消息父表

```sql
CREATE TABLE conversation_messages (
    id               BIGINT NOT NULL,
    conversation_id  BIGINT NOT NULL,
    user_id           BIGINT NOT NULL,
    role              VARCHAR(16) NOT NULL,
    content           TEXT NOT NULL,
    input_tokens      INTEGER NOT NULL DEFAULT 0,
    output_tokens     INTEGER NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE INDEX idx_conversation_messages_lookup
    ON conversation_messages (conversation_id, created_at DESC, id DESC);
```

PostgreSQL 分区表的主键或唯一约束需要包含分区键，因此这里使用 `(id, created_at)`。Snowflake 仍在业务层保证 `id` 唯一。

### 3.4 创建月份分区

```sql
CREATE TABLE conversation_messages_202610
PARTITION OF conversation_messages
FOR VALUES FROM ('2026-10-01 00:00:00+08')
             TO ('2026-11-01 00:00:00+08');

CREATE TABLE conversation_messages_202611
PARTITION OF conversation_messages
FOR VALUES FROM ('2026-11-01 00:00:00+08')
             TO ('2026-12-01 00:00:00+08');

CREATE TABLE conversation_messages_default
PARTITION OF conversation_messages DEFAULT;
```

`llm_requests` 和 `llm_usage` 使用相同方式创建父表和月份分区。

迁移程序每次保证存在：

1. 当前月份分区。
2. 下一个月份分区。
3. 默认分区。

默认分区是故障兜底。监控发现默认分区出现数据时，应迁移数据并补建缺失分区，不能让默认分区长期增长。

### 3.5 LLM 写入和查询

应用始终写父表，不拼接月份表名：

```sql
INSERT INTO conversation_messages (
    id, conversation_id, user_id, role, content, created_at
) VALUES ($1, $2, $3, $4, $5, $6);
```

PostgreSQL 根据 `created_at` 自动选择物理分区。

查询会话消息时携带时间范围：

```sql
SELECT id, conversation_id, user_id, role, content, created_at
FROM conversation_messages
WHERE conversation_id = $1
  AND created_at >= $2
  AND created_at <= $3
ORDER BY created_at ASC, id ASC;
```

分页使用 `(created_at, id)` 游标，避免较大的 `OFFSET`：

```sql
SELECT id, conversation_id, user_id, role, content, created_at
FROM conversation_messages
WHERE conversation_id = $1
  AND created_at >= $2
  AND created_at <= $3
  AND (created_at, id) < ($4, $5)
ORDER BY created_at DESC, id DESC
LIMIT 50;
```

### 3.6 创建新月份分区的操作流程

建议每月 25 日提前创建下个月分区：

1. 计算下个月第一天和再下个月第一天。
2. 检查目标分区是否已经存在。
3. 检查默认分区中是否含有目标月份数据。
4. 如果存在，先迁出默认分区中的相关数据。
5. 创建目标月份分区。
6. 验证父表写入能够进入新分区。
7. 记录迁移版本和执行结果。

创建分区应当幂等。迁移程序可以查询 `pg_class`、`pg_inherits` 或使用迁移版本表判断是否已经创建。

### 3.7 历史数据归档

月份分区达到保留期限后：

1. 将分区设置为只读或暂停相关写入。
2. 备份或导出需要长期保存的数据。
3. 从父表分离分区：`DETACH PARTITION`。
4. 验证线上查询不再依赖该分区。
5. 最后删除归档分区。

相比执行大批量 `DELETE`，删除或分离整个月份分区产生的表膨胀和维护压力更小。

## 4. 本地启动

User、LLM、Keycloak、Admin 和 Web3 共用一个 PostgreSQL 实例，但数据库和账号彼此独立：

```powershell
docker network create grpc-go-identity
docker network create grpc-go-backend
docker volume create identity_keycloak-postgres-data
Copy-Item deploy\postgresql\.env.example deploy\postgresql\.env
docker compose --env-file deploy\postgresql\.env -f deploy\postgresql\compose.yaml up -d
docker compose --env-file deploy\postgresql\.env -f deploy\postgresql\compose.yaml exec postgres /opt/grpc-go/create-service-databases.sh
```

检查健康状态：

```powershell
docker compose --env-file deploy\postgresql\.env -f deploy\postgresql\compose.yaml ps
```

## 5. 实现顺序

User：

1. 把数据库驱动和配置从 MySQL 切换到 PostgreSQL。
2. 接入 Snowflake ID。
3. 实现固定的 8 表 Router。
4. 编写迁移创建 8 张用户表和身份索引表。
5. Repository 统一调用 Router。
6. 测试 8 个分片分布、身份索引查询和事务回滚。

LLM：

1. 增加 PostgreSQL 配置和异步连接池。
2. 增加版本化迁移工具。
3. 创建会话主表和消息父表。
4. 创建当前月、下个月和默认分区。
5. Repository 只操作父表。
6. 测试跨月查询、分区裁剪、默认分区告警和游标分页。
