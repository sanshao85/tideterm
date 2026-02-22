# WaveProxy 双重认证问题排查文档

## 1. 项目概述

### 1.1 项目信息
- **项目名称**: TideTerm (基于 WaveTerm 的分支)
- **项目路径**: `/Users/admin/Desktop/code/wave/waveterm`
- **模块**: WaveProxy - AI API 代理服务
- **功能**: 为 Claude Code 等 AI 客户端提供 API 代理，支持多渠道负载均衡、故障转移、熔断器等功能

### 1.2 核心目录结构
```
/Users/admin/Desktop/code/wave/waveterm/
├── pkg/waveproxy/                    # Go 后端代理服务
│   ├── config/config.go              # 配置结构定义
│   ├── handler/                      # HTTP 处理器
│   │   ├── messages.go               # /v1/messages 端点
│   │   ├── responses.go              # /v1/responses 端点
│   │   └── auth.go                   # 认证中间件
│   ├── scheduler/scheduler.go        # 渠道调度器（含熔断器）
│   ├── channel/channel.go            # 渠道类型定义
│   ├── rpc_interface.go              # RPC 接口
│   └── rpc_helpers.go                # RPC 辅助函数
├── frontend/app/view/proxy/          # React 前端
│   ├── proxy-view.tsx                # 代理主视图
│   ├── proxy-model.tsx               # 数据模型
│   ├── proxy-rpc.ts                  # 前端 RPC 调用
│   └── channel-form.tsx              # 渠道编辑表单
└── frontend/app/i18n/i18n-core.ts    # 国际化
```

## 2. 需求说明

### 2.1 背景
Claude Code 客户端连接第三方 API 服务（如 code123.shop）时，需要同时发送两种认证头：
- `x-api-key: sk-xxx`
- `Authorization: Bearer sk-xxx`

### 2.2 用户配置方式
用户在 Claude Code 中配置环境变量：
```bash
export ANTHROPIC_BASE_URL=http://127.0.0.1:3000
export ANTHROPIC_API_KEY=sk-code123-xxx
export ANTHROPIC_AUTH_TOKEN=sk-code123-xxx
```

### 2.3 功能需求

#### 需求 1: AuthType 认证类型配置
渠道应支持配置认证类型：
- `x-api-key`: 只发送 `x-api-key` 头（Claude 官方 API 默认）
- `bearer`: 只发送 `Authorization: Bearer` 头（OpenAI 风格）
- `both`: 同时发送两种头（code123.shop 等第三方服务需要）

#### 需求 2: Passthrough 透传模式
当渠道未配置 API Key 时，代理应透传用户请求中的认证信息：
- 从请求头 `x-api-key` 获取 API Key
- 从请求头 `Authorization: Bearer xxx` 获取 Token
- 根据渠道的 `authType` 配置决定转发哪些头

## 3. 当前问题

### 3.1 错误现象
```
503 {"error":{"message":"no available channels","type":"error"}}
```

### 3.2 完整错误日志
```
2025/01/07 00:21:32 [Scheduler-Debug] SelectChannel called: channelType=messages, userID=, excludeChannels=map[]
2025/01/07 00:21:32 [Scheduler-Debug] Total channels in config: 1
2025/01/07 00:21:32 [Scheduler-Debug] Checking channel[0]: id=58c2e32d-bb82-4b8c-895b-344aa599ebbb, name=kkk, status=active
2025/01/07 00:21:32 [Scheduler-Debug] Channel 58c2e32d-bb82-4b8c-895b-344aa599ebbb: isAvailable=false
2025/01/07 00:21:32 [Scheduler-Debug] No available channels found, returning error
```

### 3.3 问题分析
1. 渠道存在于配置中（`Total channels in config: 1`）
2. 渠道状态为 active
3. 但 `isChannelAvailable` 返回 `false`
4. 很可能是熔断器（Circuit Breaker）处于打开状态

### 3.4 熔断器逻辑
文件: `pkg/waveproxy/scheduler/scheduler.go`

熔断器在以下情况会打开：
- 连续失败次数达到阈值（默认 5 次）
- 打开后会持续一段时间（默认 30 秒）才会尝试恢复

## 4. 已完成的代码修改

### 4.1 config/config.go - 添加 AuthType 字段
```go
// AuthType constants for channel authentication
const (
    AuthTypeAPIKey = "x-api-key" // Default: only x-api-key header
    AuthTypeBearer = "bearer"    // Only Authorization: Bearer header
    AuthTypeBoth   = "both"      // Both x-api-key and Authorization: Bearer headers
)

type Channel struct {
    // ... 其他字段
    AuthType           string            `json:"authType,omitempty"` // x-api-key, bearer, both
    // ... 其他字段
}
```

### 4.2 handler/messages.go - 透传认证逻辑
```go
// proxyToUpstream 函数中的认证处理
// 第 159-183 行

// Determine authentication keys
// Priority: 1. Channel configured keys  2. Passthrough from request headers
var keyForApiKey, keyForAuth string

if len(ch.APIKeys) > 0 && ch.APIKeys[0] != "" {
    // Use channel configured keys
    keyForApiKey = ch.APIKeys[0]
    keyForAuth = ch.APIKeys[0]
    log.Printf("[Messages-Auth] Using channel configured API key")
} else {
    // Passthrough mode: use keys from request headers
    keyForApiKey = r.Header.Get("x-api-key")
    authHeader := r.Header.Get("authorization")
    if strings.HasPrefix(authHeader, "Bearer ") {
        keyForAuth = strings.TrimPrefix(authHeader, "Bearer ")
    }
    log.Printf("[Messages-Auth] Passthrough mode: x-api-key=%v, auth=%v",
        keyForApiKey != "", keyForAuth != "")
}

// Check if we have at least one authentication method
if keyForApiKey == "" && keyForAuth == "" {
    ErrorResponse(w, http.StatusUnauthorized, "no authentication provided")
    return false, 0, 0, 0, 0
}

// ... 后续根据 authType 设置请求头

switch authType {
case config.AuthTypeBearer:
    authKey := keyForAuth
    if authKey == "" {
        authKey = keyForApiKey
    }
    upstreamReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authKey))
case config.AuthTypeBoth:
    apiKey := keyForApiKey
    if apiKey == "" {
        apiKey = keyForAuth
    }
    authKey := keyForAuth
    if authKey == "" {
        authKey = keyForApiKey
    }
    upstreamReq.Header.Set("x-api-key", apiKey)
    upstreamReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authKey))
default: // AuthTypeAPIKey
    apiKey := keyForApiKey
    if apiKey == "" {
        apiKey = keyForAuth
    }
    upstreamReq.Header.Set("x-api-key", apiKey)
}
```

### 4.3 handler/responses.go - 同样的透传逻辑
与 messages.go 相同的修改，位于 `proxyResponsesRequest` 函数中。

### 4.4 rpc_helpers.go - RPC 解析 AuthType
```go
type rpcProxyChannel struct {
    // ... 其他字段
    AuthType           string            `json:"authType,omitempty"`
    // ... 其他字段
}

func decodeProxyChannel(input interface{}) (*config.Channel, error) {
    // ...
    ch := &config.Channel{
        // ... 其他字段
        AuthType:           strings.TrimSpace(rpcCh.AuthType),
        // ... 其他字段
    }
    // ...
}
```

### 4.5 frontend/app/view/proxy/proxy-rpc.ts - 前端 RPC
```typescript
interface ProxyChannel {
    id: string;
    name: string;
    serviceType: string;
    baseUrl: string;
    baseUrls?: string[];
    apiKeys: string[];
    authType?: string;  // 新增
    priority: number;
    status: string;
    // ...
}

function toChannelConfig(ch: ProxyChannel): ChannelConfig {
    return {
        // ...
        authType: ch.authType,  // 新增
        // ...
    };
}

function toProxyChannel(cfg: ChannelConfig): ProxyChannel {
    return {
        // ...
        authType: cfg.authType,  // 新增
        // ...
    };
}
```

### 4.6 frontend/app/view/proxy/proxy-model.tsx
```typescript
export interface ChannelConfig {
    // ...
    authType?: string; // x-api-key, bearer, both
    // ...
}
```

### 4.7 frontend/app/view/proxy/channel-form.tsx - UI 表单
添加了 AuthType 选择器：
```tsx
<div className="form-row">
    <label>
        <span>{t("proxy.channel.authType")}</span>
        <select name="authType" value={formData.authType || ""} onChange={handleInputChange}>
            <option value="">{t("proxy.channel.authTypeDefault")}</option>
            <option value="x-api-key">x-api-key</option>
            <option value="bearer">Bearer Token</option>
            <option value="both">{t("proxy.channel.authTypeBoth")}</option>
        </select>
    </label>
</div>
```

### 4.8 i18n-core.ts - 国际化
```typescript
// English
"proxy.channel.authType": "Auth Type",
"proxy.channel.authTypeDefault": "Default (based on service type)",
"proxy.channel.authTypeBoth": "Both (x-api-key + Bearer)",

// Chinese
"proxy.channel.authType": "认证类型",
"proxy.channel.authTypeDefault": "默认（根据服务类型）",
"proxy.channel.authTypeBoth": "双重（x-api-key + Bearer）",
```

## 5. 配置文件

### 5.1 配置文件路径
```
~/.config/tideterm/waveproxy.json
```

### 5.2 当前配置内容示例
```json
{
  "channels": [
    {
      "id": "58c2e32d-bb82-4b8c-895b-344aa599ebbb",
      "name": "kkk",
      "serviceType": "claude",
      "baseUrl": "https://www.code123.shop/api",
      "apiKeys": ["sk-code123-xxx"],
      "priority": 9,
      "status": "active",
      "lowQuality": true
    }
  ]
}
```

**问题**: `authType` 字段没有保存到配置文件中！

## 6. 需要排查的问题

### 6.1 问题 1: authType 未保存到配置
虽然前端已添加了 authType 字段的支持，但配置文件中仍然没有这个字段。

**排查方向**:
1. 检查 `rpc_interface.go` 中的 `UpdateChannel` 方法是否正确处理 AuthType
2. 检查配置保存逻辑 `config.SaveConfig()` 是否正确序列化 AuthType
3. 检查是否有其他地方覆盖或丢弃了 AuthType 字段

### 6.2 问题 2: 熔断器状态导致渠道不可用
即使配置正确，熔断器可能因之前的失败而处于打开状态。

**排查方向**:
1. 检查 `scheduler/scheduler.go` 中的 `isChannelAvailable` 函数
2. 检查熔断器状态和重置逻辑
3. 考虑添加强制重置熔断器的功能

### 6.3 问题 3: 上游认证失败
如果请求到达上游但认证失败，会触发熔断器。

**排查方向**:
1. 添加更详细的日志，记录发送到上游的请求头
2. 确认 code123.shop 需要的确切认证格式
3. 验证透传模式是否正确获取用户的认证信息

## 7. 关键代码文件清单

| 文件路径 | 功能 | 关键函数/结构 |
|---------|------|--------------|
| `pkg/waveproxy/config/config.go` | 配置定义 | `Channel` 结构体, `AuthType` 常量 |
| `pkg/waveproxy/handler/messages.go` | Messages API 处理 | `proxyToUpstream()` |
| `pkg/waveproxy/handler/responses.go` | Responses API 处理 | `proxyResponsesRequest()` |
| `pkg/waveproxy/scheduler/scheduler.go` | 渠道调度 | `SelectChannel()`, `isChannelAvailable()` |
| `pkg/waveproxy/rpc_interface.go` | RPC 接口 | `UpdateChannel()`, `CreateChannel()` |
| `pkg/waveproxy/rpc_helpers.go` | RPC 辅助 | `decodeProxyChannel()` |
| `frontend/app/view/proxy/proxy-rpc.ts` | 前端 RPC | `ProxyChannel`, `toProxyChannel()` |
| `frontend/app/view/proxy/proxy-model.tsx` | 前端模型 | `ChannelConfig` |
| `frontend/app/view/proxy/channel-form.tsx` | 渠道表单 | AuthType 选择器 |

## 8. 测试步骤

### 8.1 启动代理
1. 打开 TideTerm 应用
2. 进入 Proxy 视图
3. 确保代理服务已启动

### 8.2 配置渠道
1. 添加或编辑渠道
2. 设置 Base URL: `https://www.code123.shop/api`
3. 设置认证类型为 "Both (x-api-key + Bearer)"
4. 保存

### 8.3 验证配置保存
检查配置文件是否包含 authType:
```bash
cat ~/.config/tideterm/waveproxy.json | jq .
```

### 8.4 测试连接
使用 Claude Code 测试:
```bash
export ANTHROPIC_BASE_URL=http://127.0.0.1:3000
export ANTHROPIC_API_KEY=sk-code123-xxx
export ANTHROPIC_AUTH_TOKEN=sk-code123-xxx
claude
```

## 9. 期望行为

### 9.1 配置保存后
`~/.config/tideterm/waveproxy.json` 应包含:
```json
{
  "channels": [
    {
      "id": "xxx",
      "name": "kkk",
      "serviceType": "claude",
      "baseUrl": "https://www.code123.shop/api",
      "authType": "both",
      "apiKeys": [],
      "priority": 9,
      "status": "active"
    }
  ]
}
```

### 9.2 代理转发请求时
当 authType 为 "both" 时，发送到上游的请求应包含:
```
x-api-key: sk-code123-xxx
Authorization: Bearer sk-code123-xxx
```

### 9.3 日志输出
```
[Messages-Auth] Passthrough mode: x-api-key=true, auth=true
[Messages-Auth] Sending both x-api-key and Bearer token
```

## 10. 参考项目

### 10.1 code123.shop API 服务
- 需要同时发送 `x-api-key` 和 `Authorization: Bearer` 头
- 兼容 Claude Messages API 格式

### 10.2 Claude 官方 API
- 默认只需要 `x-api-key` 头
- API 文档: https://docs.anthropic.com/

### 10.3 OpenAI API
- 只需要 `Authorization: Bearer` 头
- API 文档: https://platform.openai.com/docs/

## 11. 调试建议

### 11.1 添加详细日志
在以下位置添加日志:
1. `rpc_interface.go` 的 `UpdateChannel` - 打印接收到的 AuthType 值
2. `config.go` 的 `SaveConfig` - 打印保存的配置内容
3. `scheduler.go` 的 `isChannelAvailable` - 打印熔断器状态详情
4. `messages.go` 的 `proxyToUpstream` - 打印发送到上游的所有请求头

### 11.2 重置熔断器
在修复问题前，可能需要手动重置熔断器状态:
1. 重启整个 TideTerm 应用
2. 或者调用 `schedulerReset` RPC 方法

### 11.3 直接测试上游
绕过代理直接测试上游服务:
```bash
curl -X POST https://www.code123.shop/api/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-code123-xxx" \
  -H "Authorization: Bearer sk-code123-xxx" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-3-opus-20240229","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}'
```

## 12. 总结

当前问题的根本原因很可能是:

1. **authType 字段未正确保存到配置文件** - 这导致代理不知道要发送双重认证头
2. **熔断器处于打开状态** - 之前的请求失败（因认证问题）触发了熔断器

修复优先级:
1. 首先确保 authType 能正确保存到配置文件
2. 然后重置熔断器状态
3. 最后验证双重认证是否正确发送到上游

---

文档创建时间: 2026-01-07
最后更新: 2026-01-07
