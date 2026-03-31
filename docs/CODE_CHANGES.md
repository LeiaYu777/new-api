# 项目改动说明（NewAPI）

## 1. 修改概述
- 改动标题：依赖安全升级（前后端）
- 改动标题：密钥加密与敏感数据保护
- 改动标题：初始化接口防护（`/api/setup`）
- 改动标题：CORS / Session / Proxy / pprof 安全加固
- 改动标题：默认口令与部署配置加固
- 改动标题：前端 XSS 全面治理（统一安全渲染）
- 改动标题：多租户基础能力（tenant_id 链路 + 鉴权校验）
- 改动标题：租户审计日志能力
- 改动标题：高可用演进脚手架（控制面/转发面/worker）
- 改动标题：测试优化（Go 依赖缓存、构建验证）
- 改动标题：AGPL 合规与发布流程文档化（含 SBOM）
- 相关审查项：阶段 1-8（依赖修复、安全加固、架构演进、测试验收、合规发布）

---

## 2. 详细改动

### 修改点：前端依赖安全升级与锁定
**位置**:
- [web/package.json](/Users/leia/Codex/new-api/new-api/web/package.json)（`dependencies` / `overrides`）

**Before**:
```json
"@douyinfe/semi-icons": "^2.63.1",
"@douyinfe/semi-ui": "^2.69.1",
"@visactor/react-vchart": "~1.8.8",
"@visactor/vchart": "~1.8.8",
"@visactor/vchart-semi-theme": "~1.8.8"
```

```json
// 无 overrides
```

**After**:
```json
"@douyinfe/semi-icons": "2.69.1",
"@douyinfe/semi-ui": "2.69.1",
"@visactor/react-vchart": "~1.13.24",
"@visactor/vchart": "~1.13.24",
"@visactor/vchart-semi-theme": "~1.12.3",
"antd": "^5.28.0",
"dompurify": "^3.3.3"
```

```json
"overrides": {
  "minimist": "^1.2.8",
  "geojson-flatten": "^1.1.1"
}
```

**修改原因解析**:
- 修复前端供应链风险（`minimist` / `geojson-flatten` 通过传递依赖收敛）。
- 升级图表库分支并引入 `dompurify` 支撑 XSS 修复。
- 锁定 `semi-ui` 版本避免构建兼容性回归。

**影响范围说明**:
- 前端依赖树变化，构建体积与 chunk 结构变化。
- 需要重新安装依赖（`npm install --legacy-peer-deps`）。

**测试说明**:
- 已执行：`NODE_OPTIONS='--max-old-space-size=8192' npm run build`，构建通过。

**如果有配置改动**:
- 无额外运行时配置，属于构建依赖层变化。

---

### 修改点：后端依赖安全升级（Go）
**位置**:
- [go.mod](/Users/leia/Codex/new-api/new-api/go.mod)

**Before**:
```go
github.com/gin-gonic/gin v1.9.1
github.com/gorilla/websocket v1.5.0
golang.org/x/net v0.47.0
golang.org/x/crypto v0.45.0
gorm.io/driver/mysql v1.4.3
gorm.io/driver/postgres v1.5.2
gorm.io/gorm v1.25.2
```

**After**:
```go
github.com/gin-gonic/gin v1.12.0
github.com/gorilla/websocket v1.5.3
golang.org/x/net v0.52.0
golang.org/x/crypto v0.49.0
gorm.io/driver/mysql v1.6.0
gorm.io/driver/postgres v1.6.0
gorm.io/gorm v1.31.1
```

**修改原因解析**:
- 修复已知漏洞窗口并提升安全基线。
- 对齐新版本依赖链（`x/sys` / `x/text` / `validator` 等）。

**影响范围说明**:
- 全后端模块编译与运行时依赖变化。
- `go.sum` 自动更新（锁文件变化）。

**测试说明**:
- 已执行并通过：
  - `./scripts/go-test-docker.sh ./middleware/... ./model/...`
  - `./scripts/go-test-docker.sh ./controller/...`

**如果有配置改动**:
- 无业务配置变更，属于模块版本升级。

---

### 修改点：渠道密钥加密存储（明文 -> AES-GCM）
**位置**:
- [common/crypto.go](/Users/leia/Codex/new-api/new-api/common/crypto.go)
- [model/channel.go](/Users/leia/Codex/new-api/new-api/model/channel.go)
- [service/codex_credential_refresh.go](/Users/leia/Codex/new-api/new-api/service/codex_credential_refresh.go)
- [controller/codex_usage.go](/Users/leia/Codex/new-api/new-api/controller/codex_usage.go)
- [controller/codex_oauth.go](/Users/leia/Codex/new-api/new-api/controller/codex_oauth.go)

**Before**:
```go
// common/crypto.go 仅有 HMAC / bcrypt，无对称加解密
```

```go
// model/channel.go
type Channel struct {
    Key string `json:"key" gorm:"not null"`
}
```

```go
// 直接明文更新 key
Update("key", string(encoded))
```

**After**:
```go
const encryptedSecretPrefix = "enc:v1:"

func EncryptSecret(plainText string) (string, error) { ... }  // AES-GCM
func DecryptSecret(cipherText string) (string, error) { ... }
func IsEncryptedSecret(value string) bool { ... }
```

```go
type Channel struct {
    TenantId string `json:"tenant_id" gorm:"type:varchar(64);index;default:'default'"`
    Key      string `json:"key" gorm:"not null"`
    KeyHash  string `json:"-" gorm:"type:char(64);index"`
}

func (channel *Channel) BeforeSave(tx *gorm.DB) error { ... }  // 自动加密 + key_hash
func (channel *Channel) AfterFind(tx *gorm.DB) error { ... }   // 自动解密
```

```go
encryptedKey, err := common.EncryptSecret(string(encoded))
Update("key", encryptedKey)
```

**修改原因解析**:
- 原实现将上游 API key / OAuth key 明文落库，存在数据库泄露高风险。
- 新实现做到“透明加密 + 兼容旧数据解密”。

**影响范围说明**:
- 渠道密钥读写路径、搜索逻辑（从 `key` 精确比对改为 `key_hash` 比对）。
- Codex OAuth 回写路径已同步改造，避免绕过模型钩子写明文。

**测试说明**:
- 回归重点：新增/编辑渠道、查询渠道、Codex 刷新与 OAuth 完成流程。

**如果有配置改动**:
- 需要设置：
  - `CRYPTO_SECRET`（建议独立于 `SESSION_SECRET`）

---

### 修改点：`/api/setup` 初始化接口暴露风险修复
**位置**:
- [middleware/setup_guard.go](/Users/leia/Codex/new-api/new-api/middleware/setup_guard.go)
- [router/api-router.go](/Users/leia/Codex/new-api/new-api/router/api-router.go)

**Before**:
```go
apiRouter.GET("/setup", controller.GetSetup)
apiRouter.POST("/setup", controller.PostSetup)
```

**After**:
```go
setupRoute := apiRouter.Group("/setup")
setupRoute.Use(middleware.SetupGuard())
{
    setupRoute.GET("", controller.GetSetup)
    setupRoute.POST("", controller.PostSetup)
}
```

```go
func SetupGuard() gin.HandlerFunc {
    // 未初始化阶段：要求 SETUP_TOKEN 或仅允许内网/回环地址
}
```

**修改原因解析**:
- 未初始化时公开 `/api/setup` 可被未授权用户抢先初始化。

**影响范围说明**:
- 初始化流程调用方需要附带 `X-Setup-Token`（如配置了 `SETUP_TOKEN`）。

**测试说明**:
- 场景：无 token/错误 token/正确 token/内网 IP 初始化。

**如果有配置改动**:
- 新增可配置项：
  - `SETUP_TOKEN`
  - `SETUP_ALLOW_PRIVATE_IP=true|false`

---

### 修改点：CORS 配置从“全放开”改为白名单
**位置**:
- [middleware/cors.go](/Users/leia/Codex/new-api/new-api/middleware/cors.go)

**Before**:
```go
config := cors.DefaultConfig()
config.AllowAllOrigins = true
config.AllowCredentials = true
config.AllowHeaders = []string{"*"}
```

**After**:
```go
config := cors.Config{
  AllowCredentials: common.GetEnvOrDefaultBool("CORS_ALLOW_CREDENTIALS", true),
  AllowMethods: []string{"GET","POST","PUT","PATCH","DELETE","OPTIONS"},
  AllowHeaders: []string{"Authorization","Content-Type","X-Requested-With","X-Setup-Token","X-Newapi-Request-Id"},
  ExposeHeaders: []string{"Content-Length","Content-Type","X-Newapi-Request-Id"},
  MaxAge: 12 * time.Hour,
}
allowedOrigins := getAllowedOrigins()
if len(allowedOrigins) > 0 {
  config.AllowOrigins = allowedOrigins
} else {
  config.AllowOriginFunc = func(string) bool { return false }
}
```

**修改原因解析**:
- `AllowAllOrigins + Credentials` 组合会引入跨站凭证风险。
- 改为显式白名单，默认不允许跨域。

**影响范围说明**:
- 前端跨域调用需配置 `CORS_ALLOW_ORIGINS` 或 `FRONTEND_BASE_URL`。

**测试说明**:
- 需验证浏览器端预检请求与跨域携带 cookie 场景。

**如果有配置改动**:
- `CORS_ALLOW_ORIGINS=https://a.com,https://b.com`
- `CORS_ALLOW_CREDENTIALS=true|false`

---

### 修改点：Session / Proxy / pprof 安全加固
**位置**:
- [main.go](/Users/leia/Codex/new-api/new-api/main.go)

**Before**:
```go
log.Println(http.ListenAndServe("0.0.0.0:8005", nil))
Secure:   false,
SameSite: http.SameSiteStrictMode,
```

**After**:
```go
pprofAddr := common.GetEnvOrDefaultString("PPROF_BIND_ADDR", "127.0.0.1:8005")
log.Println(http.ListenAndServe(pprofAddr, nil))
```

```go
trustedProxies := getTrustedProxies()
server.SetTrustedProxies(trustedProxies)
```

```go
secureCookie := common.GetEnvOrDefaultBool("SESSION_COOKIE_SECURE", os.Getenv("GIN_MODE") != "debug")
sameSite := http.SameSiteLaxMode // strict/none 可配
store.Options(sessions.Options{
  HttpOnly: true,
  Secure: secureCookie,
  SameSite: sameSite,
  Domain: strings.TrimSpace(common.GetEnvOrDefaultString("SESSION_COOKIE_DOMAIN", "")),
})
```

**修改原因解析**:
- 避免默认暴露 profiling 端口到公网。
- 防止反代链路 `X-Forwarded-*` 信任过宽。
- Session cookie 默认强度提升。

**影响范围说明**:
- 反向代理部署需设置 `TRUSTED_PROXIES`。
- HTTPS 场景建议开启 `SESSION_COOKIE_SECURE=true`。

**测试说明**:
- 登录会话保持、跨子域 cookie、代理后真实 IP 识别。

**如果有配置改动**:
- `PPROF_BIND_ADDR`
- `TRUSTED_PROXIES`
- `SESSION_COOKIE_SECURE`
- `SESSION_COOKIE_SAMESITE`
- `SESSION_COOKIE_DOMAIN`

---

### 修改点：默认口令与部署暴露面加固（Compose）
**位置**:
- [docker-compose.yml](/Users/leia/Codex/new-api/new-api/docker-compose.yml)

**Before**:
```yaml
ports:
  - "3000:3000"
SQL_DSN=postgresql://root:123456@postgres:5432/new-api
POSTGRES_PASSWORD: 123456
```

**After**:
```yaml
ports:
  - "${BIND_HOST:-127.0.0.1}:${APP_PORT:-3000}:3000"
SQL_DSN=postgresql://root:${POSTGRES_PASSWORD:-ChangeThisStrongPassword!}@postgres:5432/new-api
POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-ChangeThisStrongPassword!}
SESSION_SECRET=${SESSION_SECRET:-}
CRYPTO_SECRET=${CRYPTO_SECRET:-}
SETUP_TOKEN=${SETUP_TOKEN:-}
TRUSTED_PROXIES=${TRUSTED_PROXIES:-}
CORS_ALLOW_ORIGINS=${CORS_ALLOW_ORIGINS:-}
```

**修改原因解析**:
- 默认弱口令与公网端口直暴露风险高。

**影响范围说明**:
- 本地/生产部署参数化程度提升；不再建议直接公网绑定 3000。

**测试说明**:
- 校验 Compose 启动、环境变量注入与健康检查。

**如果有配置改动**:
- 增加 `BIND_HOST / APP_PORT / POSTGRES_PASSWORD / SESSION_SECRET / CRYPTO_SECRET / SETUP_TOKEN ...`

---

### 修改点：自动创建 root 用户安全策略调整
**位置**:
- [model/main.go](/Users/leia/Codex/new-api/new-api/model/main.go)

**Before**:
```go
common.SysLog("no user exists, create a root user for you: username is root, password is 123456")
hashedPassword, err := common.Password2Hash("123456")
```

**After**:
```go
if !common.GetEnvOrDefaultBool("AUTO_CREATE_ROOT", false) {
  common.SysLog("no user exists, skip auto root creation, please use /api/setup to initialize")
  return nil
}
initialPassword := strings.TrimSpace(os.Getenv("INITIAL_ROOT_PASSWORD"))
if initialPassword == "" {
  return errors.New("INITIAL_ROOT_PASSWORD must be set when AUTO_CREATE_ROOT=true")
}
hashedPassword, err := common.Password2Hash(initialPassword)
```

**修改原因解析**:
- 禁止隐式弱口令 root 初始化。

**影响范围说明**:
- 首次启动流程改为推荐 `/api/setup`，若强制自动创建需显式配置。

**测试说明**:
- 空库首次启动：`AUTO_CREATE_ROOT=false` / `true + INITIAL_ROOT_PASSWORD` 两种路径验证。

**如果有配置改动**:
- `AUTO_CREATE_ROOT`
- `INITIAL_ROOT_PASSWORD`

---

### 修改点：前端 XSS 统一修复（SafeHtml 组件化）
**位置**:
- 新增 [web/src/components/common/SafeHtml.jsx](/Users/leia/Codex/new-api/new-api/web/src/components/common/SafeHtml.jsx)
- 变更调用点：
  - [web/src/components/common/DocumentRenderer/index.jsx](/Users/leia/Codex/new-api/new-api/web/src/components/common/DocumentRenderer/index.jsx)
  - [web/src/helpers/utils.jsx](/Users/leia/Codex/new-api/new-api/web/src/helpers/utils.jsx)
  - [web/src/components/layout/NoticeModal.jsx](/Users/leia/Codex/new-api/new-api/web/src/components/layout/NoticeModal.jsx)
  - [web/src/components/playground/CodeViewer.jsx](/Users/leia/Codex/new-api/new-api/web/src/components/playground/CodeViewer.jsx)
  - [web/src/pages/About/index.jsx](/Users/leia/Codex/new-api/new-api/web/src/pages/About/index.jsx)
  - [web/src/pages/Home/index.jsx](/Users/leia/Codex/new-api/new-api/web/src/pages/Home/index.jsx)
  - [web/src/components/dashboard/AnnouncementsPanel.jsx](/Users/leia/Codex/new-api/new-api/web/src/components/dashboard/AnnouncementsPanel.jsx)
  - [web/src/components/dashboard/FaqPanel.jsx](/Users/leia/Codex/new-api/new-api/web/src/components/dashboard/FaqPanel.jsx)
  - [web/src/components/layout/Footer.jsx](/Users/leia/Codex/new-api/new-api/web/src/components/layout/Footer.jsx)
  - [web/src/components/settings/OtherSetting.jsx](/Users/leia/Codex/new-api/new-api/web/src/components/settings/OtherSetting.jsx)

**Before**:
```jsx
<div dangerouslySetInnerHTML={{ __html: htmlContent }} />
```

**After**:
```jsx
<SafeHtml html={htmlContent} />
```

```jsx
const SANITIZE_CONFIG = {
  USE_PROFILES: { html: true },
  FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'link', 'meta'],
};
```

**修改原因解析**:
- 原来多个入口直接渲染 HTML，存在持久化 XSS/反射型 XSS 传播面。

**影响范围说明**:
- 所有公告、页脚、关于页、FAQ、文档渲染等 HTML 输出统一走净化。
- 部分危险标签将被清理，前端显示可能与历史“富文本”有差异（安全优先）。

**测试说明**:
- 人工注入 `<script>alert(1)</script>`、`<iframe>`、`onerror=` 等 payload 验证不执行。

**如果有配置改动**:
- 无新增 env，依赖 `dompurify`。

---

### 修改点：多租户配置入口与默认值
**位置**:
- [common/constants.go](/Users/leia/Codex/new-api/new-api/common/constants.go)
- [common/init.go](/Users/leia/Codex/new-api/new-api/common/init.go)
- [common/tenant.go](/Users/leia/Codex/new-api/new-api/common/tenant.go)
- [.env.example](/Users/leia/Codex/new-api/new-api/.env.example)

**Before**:
```go
// 无 MultiTenantEnabled / DefaultTenantId / TenantHeaderKey
```

**After**:
```go
var MultiTenantEnabled bool
var DefaultTenantId = "default"
var TenantHeaderKey = "X-Tenant-Id"
var TenantAuditEnabled = true
```

```go
MultiTenantEnabled = GetEnvOrDefaultBool("MULTI_TENANT_ENABLED", false)
DefaultTenantId = strings.TrimSpace(GetEnvOrDefaultString("DEFAULT_TENANT_ID", "default"))
TenantHeaderKey = strings.TrimSpace(GetEnvOrDefaultString("TENANT_HEADER_KEY", "X-Tenant-Id"))
TenantAuditEnabled = GetEnvOrDefaultBool("TENANT_AUDIT_ENABLED", true)
```

**修改原因解析**:
- 为租户隔离提供统一开关和 header 规范。

**影响范围说明**:
- 认证中间件、缓存上下文、审计逻辑都依赖这些配置。

**测试说明**:
- `MULTI_TENANT_ENABLED=true` 与 `false` 两种模式切换验证。

**如果有配置改动**:
- 新增：
  - `MULTI_TENANT_ENABLED`
  - `DEFAULT_TENANT_ID`
  - `TENANT_HEADER_KEY`
  - `TENANT_AUDIT_ENABLED`

---

### 修改点：多租户数据模型与迁移
**位置**:
- [model/user.go](/Users/leia/Codex/new-api/new-api/model/user.go)
- [model/token.go](/Users/leia/Codex/new-api/new-api/model/token.go)
- [model/channel.go](/Users/leia/Codex/new-api/new-api/model/channel.go)
- [model/user_cache.go](/Users/leia/Codex/new-api/new-api/model/user_cache.go)
- 新增 [model/tenant.go](/Users/leia/Codex/new-api/new-api/model/tenant.go)
- [model/main.go](/Users/leia/Codex/new-api/new-api/model/main.go)

**Before**:
```go
// User / Token / Channel 无 tenant_id 字段
```

**After**:
```go
TenantId string `json:"tenant_id" gorm:"type:varchar(64);index;default:'default'"`
```

```go
func (user *User) BeforeCreate(...) { user.TenantId = common.GetDefaultTenantID() }
func (token *Token) BeforeCreate(...) { token.TenantId = common.GetDefaultTenantID() }
func (channel *Channel) BeforeSave(...) { channel.TenantId = common.GetDefaultTenantID() }
```

```go
type Tenant struct { ... }
type TenantAuditLog struct { ... }
func EnsureDefaultTenant() error { ... }
```

**修改原因解析**:
- 多租户链路需要数据层可追踪并可约束。

**影响范围说明**:
- 数据库表结构迁移；用户缓存结构变化（新增 tenant_id）。

**测试说明**:
- 迁移后老数据默认租户回填逻辑与新数据创建逻辑验证。

**如果有配置改动**:
- 需启用 `MULTI_TENANT_ENABLED` 才强制租户校验。

---

### 修改点：鉴权链路租户校验（RBAC 联动）
**位置**:
- [middleware/auth.go](/Users/leia/Codex/new-api/new-api/middleware/auth.go)
- 新增 [middleware/tenant_context.go](/Users/leia/Codex/new-api/new-api/middleware/tenant_context.go)
- [router/api-router.go](/Users/leia/Codex/new-api/new-api/router/api-router.go)

**Before**:
```go
// authHelper / TokenAuth 无 tenant_id 校验
```

**After**:
```go
if common.MultiTenantEnabled && role.(int) < common.RoleRootUser && requestTenant != tenantID {
  c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "租户不匹配，无权访问该租户资源"})
  c.Abort()
  return
}
```

```go
func TenantContext() gin.HandlerFunc {
  // 统一读取 tenant_id（context/header/query），并做格式校验
}
```

**修改原因解析**:
- 防止低权限用户跨租户访问资源。

**影响范围说明**:
- 所有 `/api` 请求在开启多租户时都受租户校验约束。

**测试说明**:
- 普通用户跨租户请求应 403；Root 用户可跨租户运维。

**如果有配置改动**:
- 客户端需按约定传递 `X-Tenant-Id`（或 query `tenant_id`）。

---

### 修改点：租户审计日志（写操作审计）
**位置**:
- 新增 [middleware/tenant_audit.go](/Users/leia/Codex/new-api/new-api/middleware/tenant_audit.go)
- [router/api-router.go](/Users/leia/Codex/new-api/new-api/router/api-router.go)
- [model/tenant.go](/Users/leia/Codex/new-api/new-api/model/tenant.go)

**Before**:
```go
// 无租户审计中间件
```

**After**:
```go
apiRouter.Use(middleware.TenantAudit())
```

```go
if c.Request.Method == GET/HEAD/OPTIONS { return }
// 写操作记录 tenant_id/user_id/action/resource/status/ip
```

**修改原因解析**:
- 企业 SaaS 需具备租户级审计可追溯能力。

**影响范围说明**:
- 写接口新增审计落库路径（轻量，不阻塞主流程）。

**测试说明**:
- 创建/更新/删除类接口触发后，校验 `tenant_audit_logs` 有记录。

**如果有配置改动**:
- `TENANT_AUDIT_ENABLED=true|false`

---

### 修改点：高可用架构演进样例
**位置**:
- 新增 [docker-compose.ha.yml](/Users/leia/Codex/new-api/new-api/docker-compose.ha.yml)
- 新增 [docs/HA_EVOLUTION.md](/Users/leia/Codex/new-api/new-api/docs/HA_EVOLUTION.md)

**Before**:
```yaml
# 仅单服务 compose
```

**After**:
```yaml
services:
  new-api-control: ...
  new-api-relay: ...
  new-api-worker: ...
```

**修改原因解析**:
- 为控制面/转发面/后台任务拆分提供落地起点。

**影响范围说明**:
- 部署拓扑与日志目录结构变化；需配合网关路由策略。

**测试说明**:
- 建议 smoke：控制面管理 API、转发面模型调用、worker 异步任务。

**如果有配置改动**:
- 三类节点均需注入 `SQL_DSN / REDIS_CONN_STRING / SESSION_SECRET / CRYPTO_SECRET`。

---

### 修改点：Go 测试耗时优化（缓存化）
**位置**:
- 新增 [scripts/go-test-docker.sh](/Users/leia/Codex/new-api/new-api/scripts/go-test-docker.sh)
- 新增 [scripts/go-test-local.sh](/Users/leia/Codex/new-api/new-api/scripts/go-test-local.sh)
- [makefile](/Users/leia/Codex/new-api/new-api/makefile)
- 新增 [docs/TESTING.md](/Users/leia/Codex/new-api/new-api/docs/TESTING.md)

**Before**:
```makefile
.PHONY: all build-frontend start-backend
```

**After**:
```makefile
.PHONY: all build-frontend start-backend test-go-local test-go-docker sbom
test-go-docker:
	@$(SCRIPTS_DIR)/go-test-docker.sh
```

```bash
# go-test-docker.sh 使用持久化 cache 目录
-v "${ROOT_DIR}/.cache/go-mod:/go/pkg/mod"
-v "${ROOT_DIR}/.cache/go-build:/root/.cache/go-build"
```

**修改原因解析**:
- 解决“每次容器测试都全量重新拉依赖”的效率问题。

**影响范围说明**:
- CI / 本地测试流程可复用缓存，首次慢、后续快。

**测试说明**:
- 实测：
  - 首次跑 `./scripts/go-test-docker.sh ./middleware/... ./model/...` 预热较慢
  - 第二次命中缓存，快速返回 `ok ... (cached)`

**如果有配置改动**:
- 无需 env；默认使用项目内 `.cache` 目录。

---

### 修改点：AGPL 合规、SBOM、发布流程文档化
**位置**:
- 新增 [scripts/generate-sbom.sh](/Users/leia/Codex/new-api/new-api/scripts/generate-sbom.sh)
- 新增 [compliance/AGPL_COMPLIANCE.md](/Users/leia/Codex/new-api/new-api/compliance/AGPL_COMPLIANCE.md)
- 新增 [compliance/RELEASE_CHECKLIST.md](/Users/leia/Codex/new-api/new-api/compliance/RELEASE_CHECKLIST.md)

**Before**:
```text
# 项目中无系统化合规执行文档与 SBOM 自动脚本
```

**After**:
```bash
docker run --rm -v "${ROOT_DIR}:/src" anchore/syft:latest /src -o spdx-json > "${OUTPUT_PATH}"
```

```md
# AGPL-3.0 SaaS Compliance Checklist
- Source Offer
- Copyright / License Notice
- Change Log
- SBOM
- Closed-source Commercial Path
```

**修改原因解析**:
- 满足 AGPL 网络服务场景合规要求，并为闭源商业授权路径保留操作清单。

**影响范围说明**:
- 发布流程新增合规工件（SBOM、变更记录、许可证声明）。

**测试说明**:
- 运行 `make sbom`，检查 `compliance/sbom.spdx.json` 产物。

**如果有配置改动**:
- 需要可访问容器镜像 `anchore/syft` 的环境。

---

### 修改点：前端构建兼容性修复（图标导出变化）
**位置**:
- [web/src/helpers/render.jsx](/Users/leia/Codex/new-api/new-api/web/src/helpers/render.jsx)

**Before**:
```jsx
import { SiLinkedin } from 'react-icons/si';
linkedin: SiLinkedin,
```

**After**:
```jsx
import { FaLinkedin } from 'react-icons/fa';
linkedin: FaLinkedin,
```

**修改原因解析**:
- 新版 `react-icons/si` 中 `SiLinkedin` 导出不可用，导致构建失败。

**影响范围说明**:
- OAuth Provider 图标映射中的 LinkedIn 图标来源变更。

**测试说明**:
- 前端构建已通过，且图标渲染路径可用。

**如果有配置改动**:
- 无。

---

## 3. 统一测试与验证记录
- 已执行：
  - `./scripts/go-test-docker.sh ./middleware/... ./model/...`（通过，二次命中缓存）
  - `./scripts/go-test-docker.sh ./controller/...`（通过）
  - `NODE_OPTIONS='--max-old-space-size=8192' npm run build`（通过）
- 说明：
  - 根包 `go test ./...` 在未提供 `web/dist` 时会受 `embed` 影响；已在测试脚本中支持 `SKIP_ROOT_EMBED=1` 规避不必要失败。

---

## 4. 配置迁移清单（汇总）
建议同步更新以下配置与部署参数：
- `.env` 新增/确认：
  - `CRYPTO_SECRET`
  - `SETUP_TOKEN`
  - `SETUP_ALLOW_PRIVATE_IP`
  - `CORS_ALLOW_ORIGINS`
  - `TRUSTED_PROXIES`
  - `SESSION_COOKIE_SECURE`
  - `SESSION_COOKIE_SAMESITE`
  - `SESSION_COOKIE_DOMAIN`
  - `MULTI_TENANT_ENABLED`
  - `DEFAULT_TENANT_ID`
  - `TENANT_HEADER_KEY`
  - `TENANT_AUDIT_ENABLED`
  - `AUTO_CREATE_ROOT`
  - `INITIAL_ROOT_PASSWORD`
- `docker-compose.yml`：
  - 端口绑定建议默认仅本机：`127.0.0.1`
  - 密码与密钥走环境变量，不再使用硬编码默认值

---

## 5. 按文件分节版本

### 5.1 [common/crypto.go](/Users/leia/Codex/new-api/new-api/common/crypto.go)
- 变更类型：新增加解密能力
- 关键函数：`EncryptSecret` / `DecryptSecret` / `IsEncryptedSecret`
- 影响：为 `Channel.Key`、Codex OAuth key 提供统一加密能力

### 5.2 [model/channel.go](/Users/leia/Codex/new-api/new-api/model/channel.go)
- 变更类型：模型字段扩展 + 生命周期钩子
- 关键变更：`TenantId`、`KeyHash`、`BeforeSave`、`AfterFind`
- 影响：渠道 key 自动加密，检索走 `key_hash`

### 5.3 [middleware/setup_guard.go](/Users/leia/Codex/new-api/new-api/middleware/setup_guard.go)
- 变更类型：新增中间件
- 关键逻辑：初始化阶段 token/IP 白名单校验
- 影响：`/api/setup` 不再裸露

### 5.4 [middleware/cors.go](/Users/leia/Codex/new-api/new-api/middleware/cors.go)
- 变更类型：配置收敛
- 关键逻辑：默认拒绝跨域，仅白名单放行
- 影响：前端跨域需配置 `CORS_ALLOW_ORIGINS`

### 5.5 [main.go](/Users/leia/Codex/new-api/new-api/main.go)
- 变更类型：运行时安全配置
- 关键变更：`PPROF_BIND_ADDR`、trusted proxies、session cookie 强化
- 影响：部署侧需要显式代理/会话配置

### 5.6 [docker-compose.yml](/Users/leia/Codex/new-api/new-api/docker-compose.yml)
- 变更类型：部署模板加固
- 关键变更：本地绑定端口、密码参数化、密钥参数化
- 影响：默认配置更安全，发布前需注入真实 secrets

### 5.7 [common/constants.go](/Users/leia/Codex/new-api/new-api/common/constants.go)、[common/init.go](/Users/leia/Codex/new-api/new-api/common/init.go)、[common/tenant.go](/Users/leia/Codex/new-api/new-api/common/tenant.go)
- 变更类型：多租户全局配置
- 关键变更：`MULTI_TENANT_ENABLED` 等
- 影响：租户模式开关与默认租户值统一来源

### 5.8 [model/user.go](/Users/leia/Codex/new-api/new-api/model/user.go)、[model/token.go](/Users/leia/Codex/new-api/new-api/model/token.go)、[model/user_cache.go](/Users/leia/Codex/new-api/new-api/model/user_cache.go)、[model/tenant.go](/Users/leia/Codex/new-api/new-api/model/tenant.go)、[model/main.go](/Users/leia/Codex/new-api/new-api/model/main.go)
- 变更类型：多租户数据层
- 关键变更：`tenant_id` 字段与迁移、默认租户保证、审计日志模型
- 影响：数据库 schema 与缓存结构更新

### 5.9 [middleware/auth.go](/Users/leia/Codex/new-api/new-api/middleware/auth.go)、[middleware/tenant_context.go](/Users/leia/Codex/new-api/new-api/middleware/tenant_context.go)、[middleware/tenant_audit.go](/Users/leia/Codex/new-api/new-api/middleware/tenant_audit.go)、[router/api-router.go](/Users/leia/Codex/new-api/new-api/router/api-router.go)
- 变更类型：多租户鉴权与审计链路
- 关键变更：请求 tenant 校验、写操作审计
- 影响：API 请求需按租户维度约束

### 5.10 [web/src/components/common/SafeHtml.jsx](/Users/leia/Codex/new-api/new-api/web/src/components/common/SafeHtml.jsx) + 相关调用文件
- 变更类型：前端安全渲染统一
- 关键变更：替换多处 `dangerouslySetInnerHTML`
- 影响：富文本危险标签会被净化

### 5.11 [go.mod](/Users/leia/Codex/new-api/new-api/go.mod)、[go.sum](/Users/leia/Codex/new-api/new-api/go.sum)、[web/package.json](/Users/leia/Codex/new-api/new-api/web/package.json)
- 变更类型：依赖升级
- 关键变更：Go / npm 依赖安全版本收敛
- 影响：需要重新安装依赖与回归构建

### 5.12 [makefile](/Users/leia/Codex/new-api/new-api/makefile)、[scripts/go-test-docker.sh](/Users/leia/Codex/new-api/new-api/scripts/go-test-docker.sh)、[scripts/go-test-local.sh](/Users/leia/Codex/new-api/new-api/scripts/go-test-local.sh)、[docs/TESTING.md](/Users/leia/Codex/new-api/new-api/docs/TESTING.md)
- 变更类型：测试效率优化
- 关键变更：持久化 Go 构建缓存
- 影响：首次慢，后续测试显著提速

### 5.13 [scripts/generate-sbom.sh](/Users/leia/Codex/new-api/new-api/scripts/generate-sbom.sh)、[compliance/AGPL_COMPLIANCE.md](/Users/leia/Codex/new-api/new-api/compliance/AGPL_COMPLIANCE.md)、[compliance/RELEASE_CHECKLIST.md](/Users/leia/Codex/new-api/new-api/compliance/RELEASE_CHECKLIST.md)
- 变更类型：合规发布能力
- 关键变更：SBOM 生成、AGPL 合规流程、发布清单
- 影响：交付流程标准化，支持审计

### 5.14 [docker-compose.ha.yml](/Users/leia/Codex/new-api/new-api/docker-compose.ha.yml)、[docs/HA_EVOLUTION.md](/Users/leia/Codex/new-api/new-api/docs/HA_EVOLUTION.md)
- 变更类型：高可用演进样例
- 关键变更：控制面/转发面/worker 拆分模板
- 影响：可作为后续生产架构演进起点
