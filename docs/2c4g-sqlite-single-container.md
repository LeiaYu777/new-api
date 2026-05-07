# 2C4G SQLite 单容器部署验证（第 1 阶段）

本文档用于验证 New API 是否可以在 2 vCPU / 4GB RAM 云服务器上，使用官方预构建镜像、SQLite 和单容器方式启动。

本阶段只验证最小启动路径，不修改后端源码、不修改前端源码、不构建 Docker 镜像、不引入 Redis/PostgreSQL/MySQL。

## 1. 适用场景

适用于以下场景：

- 需要先确认 2C4G 云服务器能否启动 New API。
- 只做第 1 阶段部署验证，不做源码级 lean 改造。
- 允许先使用 SQLite 作为单机本地数据库。
- 希望避免在小规格服务器上构建 Docker 镜像、安装前端依赖或启动额外数据库/缓存容器。
- 暂时只验证登录后台、用户、渠道、Token、日志、额度扣减等基础链路。

不适用于以下场景：

- 高并发生产部署。
- 多节点部署。
- 需要高可用数据库或严格数据库备份恢复 SLA 的生产环境。
- 需要 Redis 分布式限流、缓存或队列能力的部署。

## 2. 为什么第一阶段选择官方镜像

当前仓库文档中推荐的官方预构建镜像为：

```text
calciumion/new-api:latest
```

依据：

- `README.md` 的 Docker 示例使用 `docker pull calciumion/new-api:latest`。
- `README.zh_CN.md` 的 Docker 示例同样使用 `docker pull calciumion/new-api:latest`。
- 当前 `docker-compose.yml` 中 `new-api` 服务使用 `image: calciumion/new-api:latest`。

第一阶段选择官方镜像，是为了避免在 2C4G 服务器上执行前端构建、Go 编译、依赖下载和镜像构建。这可以显著降低内存、CPU、磁盘和网络不稳定带来的干扰。

## 3. 为什么第一阶段选择 SQLite

当前 README 的默认 SQLite 示例是不设置 `SQL_DSN`，只挂载 `./data:/data`：

```bash
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

代码侧在未设置 `SQL_DSN` 时会使用 SQLite。默认 SQLite 文件路径为容器工作目录下的：

```text
/data/one-api.db
```

因此宿主机挂载 `./data:/data` 后，数据库文件会落在：

```text
./data/one-api.db
```

第一阶段选择 SQLite 的原因：

- 只需要一个容器。
- 不需要额外数据库服务。
- 不需要数据库密码或 DSN。
- 更容易定位启动失败是应用问题、镜像问题还是服务器资源问题。
- 对小流量验证足够简单。

## 4. 为什么第一阶段不启用 Redis/Postgres/MySQL

本阶段目标是验证最小可启动路径，因此不启用额外服务：

- 不启用 Redis：避免额外内存占用，并避免 Redis 触发应用内缓存路径。
- 不启用 PostgreSQL：避免数据库容器、连接池、磁盘 WAL 和内存参数带来的复杂度。
- 不启用 MySQL：避免 MySQL 进程内存、字符集、连接池和初始化成本。
- 不设置 `SQL_DSN`：让应用走默认 SQLite。
- 不设置 `REDIS_CONN_STRING`：确保不会连接 Redis。

如果第 1 阶段验证通过，再根据实际并发、数据量和备份要求决定是否进入 Postgres/Redis 阶段。

## 5. 目录准备命令

在服务器上创建部署目录：

```bash
mkdir -p /opt/new-api-2c4g
cd /opt/new-api-2c4g
```

准备持久化目录：

```bash
mkdir -p data logs backups
```

将以下文件放入该目录：

- `docker-compose.sqlite.yml`
- `.env.sqlite`

如果是从仓库复制示例文件：

```bash
cp .env.sqlite.example .env.sqlite
```

然后编辑 `.env.sqlite`，至少替换：

```env
SESSION_SECRET=please-change-this
SOURCE_CODE_URL=https://your-domain.example/source-code
LEGAL_CONTACT_EMAIL=opensource@example.com
MODIFIER_NAME=Your Name or Organization
MODIFIED_VERSION=your-version
MODIFIED_DATE=YYYY-MM-DD
MODIFICATION_SUMMARY=Official image SQLite single-container validation based on New API.
```

注意：不要把真实 API Key、客户信息、账单数据、数据库文件或服务器密钥写入 `.env.sqlite.example` 或源码仓库。

## 6. `.env` 文件准备方式

本阶段使用 `.env.sqlite`：

```bash
cp .env.sqlite.example .env.sqlite
nano .env.sqlite
```

建议第 1 阶段保留以下小内存参数：

```env
TZ=Asia/Shanghai
SESSION_SECRET=please-change-this
ERROR_LOG_ENABLED=false
BATCH_UPDATE_ENABLED=false
MEMORY_CACHE_ENABLED=false
MAX_REQUEST_BODY_MB=8
STREAM_SCANNER_MAX_BUFFER_MB=8
```

AGPLv3 源码获取展示参数：

```env
SOURCE_CODE_URL=https://your-domain.example/source-code
LEGAL_CONTACT_EMAIL=opensource@example.com
MODIFIER_NAME=Your Name or Organization
MODIFIED_VERSION=your-version
MODIFIED_DATE=YYYY-MM-DD
MODIFICATION_SUMMARY=Official image SQLite single-container validation based on New API.
```

生产部署时应将 `SESSION_SECRET` 替换为私有随机值，但不要把真实值提交到仓库。

## 7. Docker Compose 启动命令

拉取官方镜像：

```bash
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite pull
```

启动单容器：

```bash
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite up -d
```

确认容器：

```bash
docker ps
```

本阶段不执行：

```bash
docker build
```

也不启动 Redis/Postgres/MySQL。

## 8. 查看日志命令

查看最近日志：

```bash
docker logs --tail=120 new-api
```

持续跟随日志：

```bash
docker logs -f new-api
```

或使用 compose：

```bash
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite logs -f --tail=120 new-api
```

重点观察：

- 是否显示使用 SQLite。
- 是否完成数据库迁移。
- 是否监听 `http://localhost:3000/`。
- 是否出现 OOM、panic、数据库锁或权限错误。

## 9. 查看内存命令

查看一次性资源占用：

```bash
docker stats --no-stream new-api
```

查看容器进程：

```bash
docker top new-api
```

查看宿主机内存：

```bash
free -h
```

如果容器内存持续增长或频繁重启，查看：

```bash
docker inspect new-api --format '{{.State.Status}} {{.State.OOMKilled}} {{.State.RestartCount}}'
```

## 10. 健康检查命令

本 compose 的 healthcheck 使用：

```text
GET http://localhost:3000/api/status
```

手动检查：

```bash
curl http://localhost:3000/api/status
```

期望响应中包含：

```json
{"success": true}
```

查看 Docker health 状态：

```bash
docker inspect --format '{{json .State.Health}}' new-api
```

也可以执行 smoke test：

```bash
./scripts/smoke-2c4g-sqlite.sh
```

脚本只检查启动、健康接口和一次性 `docker stats`，不会读取 API Key，不会调用真实模型，也不会访问客户数据。

## 11. 登录后台检查步骤

1. 浏览器访问：

```text
http://服务器IP:3000
```

2. 如果系统未初始化，按页面提示完成初始化。
3. 创建管理员账号。
4. 登录后台。
5. 检查以下页面是否可打开：

- 用户管理。
- 渠道管理。
- 令牌管理。
- 使用日志。
- 账单管理或个人账单。
- 开源许可证与源码获取页面。

6. 访问：

```text
http://服务器IP:3000/about/open-source
```

确认源码获取说明可见，且不展示 API Key、数据库连接、客户数据或服务器密钥。

## 12. 如何配置火山方舟 / 字节模型渠道

本阶段不把上游 API Key 写入源码或 `.env.sqlite.example`。

在后台配置渠道：

1. 登录管理员后台。
2. 进入“渠道管理”。
3. 新增渠道。
4. 选择与火山方舟 / 字节模型对应的渠道类型，例如 Volcengine / 火山方舟，或按项目支持的 OpenAI-compatible 自定义渠道。
5. 填写渠道名称，例如：

```text
volcengine-seedance-validation
```

6. 按火山方舟控制台和官方文档填写 Base URL、API Key、模型名称或模型映射。
7. 模型名称按实际开通模型填写，例如字节/火山方舟控制台中分配的模型名。
8. 保存渠道。
9. 使用后台“测试”或后续 curl 命令验证。

安全要求：

- API Key 只允许填写在私有后台或私有部署环境中。
- 不要把 API Key 写入 Git 仓库。
- 不要把 API Key 写入 `docker-compose.sqlite.yml`。
- 不要把客户名称、客户账号、账单明细写入示例文件。

## 13. 如何创建 token

1. 登录普通用户或管理员账号。
2. 进入“令牌管理”。
3. 创建新的 API Token。
4. 设置名称、额度、分组和模型权限。
5. 保存后复制 token。
6. token 只在本地安全位置临时使用，不要写入仓库、文档或日志。

## 14. 如何测试 `/v1/chat/completions`

完成渠道配置和 token 创建后，可以从服务器本地测试 OpenAI-compatible relay。

示例命令中 `<your-private-token>` 是占位符，请替换为后台生成的私有 token，不要提交到仓库：

```bash
curl http://localhost:3000/v1/chat/completions \
  -H 'Authorization: Bearer <your-private-token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "your-volcengine-or-openai-compatible-model",
    "messages": [
      {"role": "user", "content": "ping"}
    ]
  }'
```

注意：这个命令会在渠道配置正确时调用真实上游模型，可能产生费用。第 1 阶段 smoke test 不执行这个命令。

## 15. 如何查看日志和额度扣减

后台检查：

1. 进入“使用日志”。
2. 按用户、模型、时间筛选。
3. 查看请求状态、消耗额度、token 统计和错误信息。
4. 进入“账单管理”或“我的账单”。
5. 检查额度扣减、退款或账单汇总是否符合预期。

容器日志检查：

```bash
docker logs --tail=200 new-api
```

SQLite 数据库文件位置：

```text
./data/one-api.db
```

如果需要直接检查数据库，请先备份，再使用 SQLite 工具只读打开。

## 16. 如何备份 SQLite 数据

建议停服务后备份，避免复制过程中出现不一致：

```bash
mkdir -p backups
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite stop new-api
cp -a data/one-api.db "backups/one-api-$(date +%Y%m%d-%H%M%S).db"
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite up -d
```

如果启用了 WAL 或其他 SQLite 附属文件，也应一并备份：

```bash
cp -a data/one-api.db* backups/
```

恢复时：

```bash
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite down
cp -a backups/one-api-YYYYMMDD-HHMMSS.db data/one-api.db
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite up -d
```

## 17. 如何回滚

停止并删除容器，但保留数据目录：

```bash
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite down
```

如果要回到启动前状态：

```bash
mv data "data.rollback.$(date +%Y%m%d-%H%M%S)"
mv logs "logs.rollback.$(date +%Y%m%d-%H%M%S)"
mkdir -p data logs
```

如果要回滚到某个备份：

```bash
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite down
cp -a backups/one-api-YYYYMMDD-HHMMSS.db data/one-api.db
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite up -d
```

删除镜像不是必须。如确需清理：

```bash
docker image rm calciumion/new-api:latest
```

## 18. 常见问题

### Q1: 为什么没有启动 Redis？

第 1 阶段只验证单容器启动和基础功能。Redis 会增加内存占用和部署复杂度，因此暂不启用。

### Q2: 为什么没有启动 PostgreSQL 或 MySQL？

本阶段目标是确认官方镜像在 2C4G 上能否跑起来。SQLite 更简单，不需要数据库服务、密码、连接池和网络依赖。

### Q3: 访问 `http://服务器IP:3000` 失败怎么办？

检查：

```bash
docker ps
docker logs --tail=120 new-api
curl http://localhost:3000/api/status
```

如果本机 curl 正常但外部访问失败，检查云服务器安全组、防火墙和端口开放。

### Q4: `/api/status` 返回失败怎么办？

查看容器日志：

```bash
docker logs --tail=200 new-api
```

重点看数据库迁移、文件权限、端口占用和 panic。

### Q5: SQLite 文件没有生成在哪里？

默认在：

```text
./data/one-api.db
```

确认目录权限：

```bash
ls -la data
```

### Q6: 容器反复重启怎么办？

查看状态和日志：

```bash
docker inspect new-api --format '{{.State.Status}} {{.State.OOMKilled}} {{.State.RestartCount}}'
docker logs --tail=200 new-api
```

如果 `OOMKilled=true`，先确认没有在同机运行其他高内存服务，并保持本阶段不启用 Redis/Postgres/MySQL。

### Q7: 如何避免公开源码页面泄露敏感信息？

只在 `.env.sqlite` 中设置公开源码地址和联系邮箱。不要在源码页面、README、compose 或示例 env 中写入：

- API Key。
- 客户数据。
- 账单明细。
- 数据库密码。
- 服务器密钥。
- 生产 `.env`。

### Q8: 这个方案可以直接用于长期生产吗？

不建议。SQLite 单容器适合第 1 阶段小流量验证。长期生产应根据并发、备份、恢复、审计和可用性要求，评估是否迁移到外部 PostgreSQL、引入 Redis、反向代理 HTTPS 和监控告警。

## 验收命令汇总

```bash
cp .env.sqlite.example .env.sqlite
nano .env.sqlite
mkdir -p data logs backups
docker compose -f docker-compose.sqlite.yml --env-file .env.sqlite up -d
docker ps
docker logs new-api
curl http://localhost:3000/api/status
docker stats --no-stream new-api
```
