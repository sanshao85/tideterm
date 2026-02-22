// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Proxy RPC API - provides frontend access to proxy backend
// Uses WaveTerm's wshrpc system for communication with Go backend

import { TabRpcClient } from "@/app/store/wshrpcutil";
import type { ChannelConfig, ProxyStatus, ChannelMetrics, GlobalStats, ChannelType, RequestRecord } from "./proxy-model.tsx";

// Type definitions for RPC requests/responses that match wshrpctypes.go
interface ProxyStatusData {
    running: boolean;
    port: number;
    startedAt?: string;
    uptime?: string;
    version: string;
    channelCount: number;
}

interface CommandProxySetPortData {
    port: number;
}

interface ProxyChannel {
    id: string;
    name: string;
    serviceType: string;
    baseUrl: string;
    baseUrls?: string[];
    // Backward compatible: older servers may return string API keys.
    apiKeys: Array<{ key: string; enabled: boolean } | string>;
    authType?: string;
    priority: number;
    status: string;
    promotionUntil?: string;
    modelMapping?: Record<string, string>;
    lowQuality?: boolean;
    description?: string;
}

interface CommandProxyChannelListData {
    channelType: string;
}

interface CommandProxyChannelListRtnData {
    channels: ProxyChannel[];
}

interface CommandProxyChannelCreateData {
    channelType: string;
    channel: ProxyChannel;
}

interface CommandProxyChannelUpdateData {
    channelType: string;
    index: number;
    channel: ProxyChannel;
}

interface CommandProxyChannelDeleteData {
    channelType: string;
    index: number;
}

interface CommandProxyChannelPingData {
    channelType: string;
    index: number;
}

interface CommandProxyChannelPingRtnData {
    success: boolean;
    latencyMs: number;
    error?: string;
}

interface CommandProxyMetricsData {
    channelId?: string;
}

interface ProxyChannelMetrics {
    channelId: string;
    requestCount: number;
    successCount: number;
    failureCount: number;
    successRate: number;
    consecutiveFailures: number;
    circuitBroken: boolean;
    inputTokens: number;
    outputTokens: number;
    cacheHitRate: number;
    avgLatencyMs: number;
}

interface ProxyGlobalStats {
    totalRequests: number;
    successCount: number;
    failureCount: number;
    successRate: number;
    channelCount: number;
}

interface CommandProxyRequestHistoryData {
    limit?: number;
    offset?: number;
    channelId?: string;
    status?: string;
}

interface ProxyRequestRecord {
    id: string;
    timestamp: string;
    channelId: string;
    channelType: string;
    model: string;
    success: boolean;
    latencyMs: number;
    inputTokens: number;
    outputTokens: number;
    errorMsg?: string;
    errorDetails?: string;
}

interface CommandProxyRequestHistoryRtnData {
    records: ProxyRequestRecord[];
    totalCount: number;
}

function errorMessage(err: unknown): string {
    if (err instanceof Error) {
        return err.message;
    }
    return String(err ?? "");
}

function shouldRetryLegacyApiKeys(err: unknown): boolean {
    const msg = errorMessage(err);
    return (
        msg.includes("error re-marshalling command data") &&
        msg.includes("apiKeys") &&
        (msg.includes("of type string") || msg.includes("into Go struct field"))
    );
}

function normalizeApiKeys(apiKeys: ProxyChannel["apiKeys"]): { key: string; enabled: boolean }[] {
    if (!Array.isArray(apiKeys)) {
        return [];
    }
    const normalized: { key: string; enabled: boolean }[] = [];
    for (const k of apiKeys) {
        if (typeof k === "string") {
            const trimmed = k.trim();
            if (!trimmed) continue;
            normalized.push({ key: trimmed, enabled: true });
            continue;
        }
        if (k && typeof k === "object") {
            const key = String((k as any).key ?? "").trim();
            if (!key) continue;
            const enabled = (k as any).enabled !== false;
            normalized.push({ key, enabled });
        }
    }
    return normalized;
}

function toLegacyApiKeyStrings(apiKeys: ChannelConfig["apiKeys"]): string[] {
    if (!Array.isArray(apiKeys)) {
        return [];
    }
    const out: string[] = [];
    for (const k of apiKeys as any[]) {
        if (typeof k === "string") {
            const trimmed = k.trim();
            if (trimmed) out.push(trimmed);
            continue;
        }
        if (k && typeof k === "object") {
            const key = String((k as any).key ?? "").trim();
            if (!key) continue;
            const enabled = (k as any).enabled !== false;
            if (enabled) out.push(key);
        }
    }
    return out;
}

// Convert backend ProxyChannel to frontend ChannelConfig
function toChannelConfig(ch: ProxyChannel): ChannelConfig {
    return {
        id: ch.id,
        name: ch.name,
        serviceType: ch.serviceType as "claude" | "openai" | "gemini" | "custom",
        baseUrl: ch.baseUrl,
        baseUrls: ch.baseUrls,
        apiKeys: normalizeApiKeys(ch.apiKeys),
        authType: ch.authType,
        priority: ch.priority,
        status: ch.status as "active" | "suspended" | "disabled",
        promotionUntil: ch.promotionUntil,
        modelMapping: ch.modelMapping,
        lowQuality: ch.lowQuality,
        description: ch.description,
    };
}

// Convert frontend ChannelConfig to backend ProxyChannel
function toProxyChannel(cfg: ChannelConfig): ProxyChannel {
    return {
        id: cfg.id,
        name: cfg.name,
        serviceType: cfg.serviceType,
        baseUrl: cfg.baseUrl,
        baseUrls: cfg.baseUrls,
        apiKeys: cfg.apiKeys,
        authType: cfg.authType,
        priority: cfg.priority,
        status: cfg.status,
        promotionUntil: cfg.promotionUntil,
        modelMapping: cfg.modelMapping,
        lowQuality: cfg.lowQuality,
        description: cfg.description,
    };
}

function toProxyChannelLegacyApiKeys(cfg: ChannelConfig): ProxyChannel {
    // Some older remote servers expect apiKeys as string[]; send enabled keys only.
    return {
        ...toProxyChannel(cfg),
        apiKeys: toLegacyApiKeyStrings(cfg.apiKeys),
    };
}

// Convert backend ProxyChannelMetrics to frontend ChannelMetrics
function toChannelMetrics(m: ProxyChannelMetrics): ChannelMetrics {
    return {
        channelId: m.channelId,
        requestCount: m.requestCount,
        successCount: m.successCount,
        failureCount: m.failureCount,
        successRate: m.successRate,
        consecutiveFailures: m.consecutiveFailures,
        circuitBroken: m.circuitBroken,
        inputTokens: m.inputTokens,
        outputTokens: m.outputTokens,
        cacheHitRate: m.cacheHitRate,
        avgLatencyMs: m.avgLatencyMs,
    };
}

// Convert backend ProxyRequestRecord to frontend RequestRecord
function toRequestRecord(r: ProxyRequestRecord): RequestRecord {
    return {
        id: r.id,
        timestamp: r.timestamp,
        channelId: r.channelId,
        channelType: r.channelType as ChannelType,
        model: r.model,
        success: r.success,
        latencyMs: r.latencyMs,
        inputTokens: r.inputTokens,
        outputTokens: r.outputTokens,
        errorMsg: r.errorMsg,
        errorDetails: r.errorDetails,
    };
}

// ProxyRpcApi provides RPC methods for the proxy service
// These methods call the Go backend via WaveTerm's wshrpc system
export class ProxyRpcApi {
    // Start the proxy server
    static async proxyStart(route?: string | null): Promise<void> {
        return TabRpcClient.wshRpcCall("proxystart", null, { route });
    }

    // Stop the proxy server
    static async proxyStop(route?: string | null): Promise<void> {
        return TabRpcClient.wshRpcCall("proxystop", null, { route });
    }

    // Get proxy status
    static async proxyStatus(route?: string | null): Promise<ProxyStatus> {
        const data: ProxyStatusData = await TabRpcClient.wshRpcCall("proxystatus", null, { route });
        return {
            running: data.running,
            port: data.port,
            startedAt: data.startedAt,
            uptime: data.uptime,
            version: data.version,
            channelCount: data.channelCount,
        };
    }

    static async proxySetPort(port: number, route?: string | null): Promise<void> {
        const req: CommandProxySetPortData = { port };
        return TabRpcClient.wshRpcCall("proxysetport", req, { route });
    }

    // List channels
    static async channelList(channelType: ChannelType, route?: string | null): Promise<ChannelConfig[]> {
        const req: CommandProxyChannelListData = { channelType };
        const resp: CommandProxyChannelListRtnData = await TabRpcClient.wshRpcCall("proxychannellist", req, { route });
        return (resp.channels || []).map(toChannelConfig);
    }

    // Create channel
    static async channelCreate(channelType: ChannelType, channel: ChannelConfig, route?: string | null): Promise<void> {
        const req: CommandProxyChannelCreateData = { channelType, channel: toProxyChannel(channel) };
        try {
            return await TabRpcClient.wshRpcCall("proxychannelcreate", req, { route });
        } catch (e) {
            if (route && shouldRetryLegacyApiKeys(e)) {
                const legacyReq: CommandProxyChannelCreateData = {
                    channelType,
                    channel: toProxyChannelLegacyApiKeys(channel) as any,
                };
                return await TabRpcClient.wshRpcCall("proxychannelcreate", legacyReq, { route });
            }
            throw e;
        }
    }

    // Update channel
    static async channelUpdate(
        channelType: ChannelType,
        index: number,
        channel: ChannelConfig,
        route?: string | null
    ): Promise<void> {
        const req: CommandProxyChannelUpdateData = { channelType, index, channel: toProxyChannel(channel) };
        try {
            return await TabRpcClient.wshRpcCall("proxychannelupdate", req, { route });
        } catch (e) {
            if (route && shouldRetryLegacyApiKeys(e)) {
                const legacyReq: CommandProxyChannelUpdateData = {
                    channelType,
                    index,
                    channel: toProxyChannelLegacyApiKeys(channel) as any,
                };
                return await TabRpcClient.wshRpcCall("proxychannelupdate", legacyReq, { route });
            }
            throw e;
        }
    }

    // Delete channel
    static async channelDelete(channelType: ChannelType, index: number, route?: string | null): Promise<void> {
        const req: CommandProxyChannelDeleteData = { channelType, index };
        return TabRpcClient.wshRpcCall("proxychanneldelete", req, { route });
    }

    // Ping channel
    static async channelPing(
        channelType: ChannelType,
        index: number,
        route?: string | null
    ): Promise<{ success: boolean; latencyMs: number; error?: string }> {
        const req: CommandProxyChannelPingData = { channelType, index };
        const resp: CommandProxyChannelPingRtnData = await TabRpcClient.wshRpcCall("proxychannelping", req, { route });
        return {
            success: resp.success,
            latencyMs: resp.latencyMs,
            error: resp.error,
        };
    }

    // Get channel metrics
    static async channelMetrics(channelId?: string, route?: string | null): Promise<ChannelMetrics[]> {
        const req: CommandProxyMetricsData = { channelId };
        const resp: ProxyChannelMetrics[] = await TabRpcClient.wshRpcCall("proxymetrics", req, { route });
        return (resp || []).map(toChannelMetrics);
    }

    // Get global stats
    static async globalStats(route?: string | null): Promise<GlobalStats> {
        const resp: ProxyGlobalStats = await TabRpcClient.wshRpcCall("proxyglobalstats", null, { route });
        return {
            totalRequests: resp.totalRequests,
            successCount: resp.successCount,
            failureCount: resp.failureCount,
            successRate: resp.successRate,
            channelCount: resp.channelCount,
        };
    }

    // Reset scheduler circuit breaker
    static async schedulerReset(channelId: string, route?: string | null): Promise<void> {
        return TabRpcClient.wshRpcCall("proxyschedulerreset", channelId, { route });
    }

    // Get request history
    static async requestHistory(
        limit?: number,
        offset?: number,
        channelId?: string,
        status?: string,
        route?: string | null
    ): Promise<{ records: RequestRecord[]; totalCount: number }> {
        const req: CommandProxyRequestHistoryData = { limit, offset, channelId, status };
        const resp: CommandProxyRequestHistoryRtnData = await TabRpcClient.wshRpcCall("proxyrequesthistory", req, { route });
        return {
            records: (resp.records || []).map(toRequestRecord),
            totalCount: resp.totalCount,
        };
    }

    static async historyClear(route?: string | null): Promise<void> {
        return TabRpcClient.wshRpcCall("proxyhistoryclear", null, { route });
    }

}
