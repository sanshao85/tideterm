// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import type { BlockNodeModel } from "@/app/block/blocktypes";
import type { TabModel } from "@/app/store/tab-model";
import { getAppLanguageFromSettings, t as tCore } from "@/app/i18n/i18n-core";
import { atoms, getConnStatusAtom, globalStore, pushFlashError, pushNotification, WOS } from "@/store/global";
import { isBlank, isLocalConnName, makeConnRoute } from "@/util/util";
import * as jotai from "jotai";
import { minimizeProxyBlock } from "./proxy-dock-model";
import { ProxyRpcApi } from "./proxy-rpc";

// Channel types
export type ChannelType = "messages" | "responses" | "gemini";

const allChannelTypes: ChannelType[] = ["messages", "responses", "gemini"];

export type EditingChannel = {
    channelType: ChannelType;
    index: number;
    channel: ChannelConfig;
};

export type ProxyAPIKey = {
    key: string;
    enabled: boolean;
};

// Channel configuration
export interface ChannelConfig {
    id: string;
    name: string;
    serviceType: string; // claude, openai, gemini
    baseUrl: string;
    baseUrls?: string[];
    apiKeys: ProxyAPIKey[];
    authType?: string; // x-api-key, x-goog-api-key, bearer, both
    priority: number;
    status: string; // active, suspended, disabled
    promotionUntil?: string;
    modelMapping?: { [key: string]: string };
    lowQuality?: boolean;
    description?: string;
}

// Channel metrics
export interface ChannelMetrics {
    channelId: string;
    requestCount: number;
    successCount: number;
    failureCount: number;
    successRate: number;
    consecutiveFailures: number;
    circuitBroken: boolean;
    lastSuccessAt?: string;
    lastFailureAt?: string;
    inputTokens: number;
    outputTokens: number;
    cacheHitRate: number;
    avgLatencyMs: number;
}

// Proxy server status
export interface ProxyStatus {
    running: boolean;
    port: number;
    startedAt?: string;
    uptime?: string;
    version: string;
    channelCount: number;
}

// Global stats
export interface GlobalStats {
    totalRequests: number;
    successCount: number;
    failureCount: number;
    successRate: number;
    channelCount: number;
}

// Request history record
export interface RequestRecord {
    id: string;
    timestamp: string;
    channelId: string;
    channelType: ChannelType;
    model: string;
    success: boolean;
    latencyMs: number;
    inputTokens: number;
    outputTokens: number;
    errorMsg?: string;
    errorDetails?: string;
}

export class ProxyViewModel implements ViewModel {
    viewType: string;
    nodeModel: BlockNodeModel;
    tabModel: TabModel;
    blockAtom: jotai.Atom<Block>;
    blockId: string;
    viewIcon: jotai.Atom<string>;
    viewText: jotai.Atom<string>;
    viewName: jotai.Atom<string>;
    endIconButtons: jotai.Atom<IconButtonDecl[]>;
    manageConnection: jotai.Atom<boolean>;
    filterOutNowsh?: jotai.Atom<boolean>;
    connectionImmediate: jotai.Atom<string>;
    connStatus: jotai.Atom<ConnStatus>;

    // Proxy state atoms
    proxyStatusAtom: jotai.PrimitiveAtom<ProxyStatus | null>;
    channelsAtom: jotai.PrimitiveAtom<ChannelConfig[]>;
    responseChannelsAtom: jotai.PrimitiveAtom<ChannelConfig[]>;
    geminiChannelsAtom: jotai.PrimitiveAtom<ChannelConfig[]>;
    metricsAtom: jotai.PrimitiveAtom<{ [key: string]: ChannelMetrics }>;
    globalStatsAtom: jotai.PrimitiveAtom<GlobalStats | null>;
    loadingAtom: jotai.PrimitiveAtom<boolean>;
    selectedTabAtom: jotai.PrimitiveAtom<ChannelType>;
    editingChannelAtom: jotai.PrimitiveAtom<EditingChannel | null>;
    isFormOpenAtom: jotai.PrimitiveAtom<boolean>;

    // History state atoms
    historyRecordsAtom: jotai.PrimitiveAtom<RequestRecord[]>;
    historyTotalCountAtom: jotai.PrimitiveAtom<number>;
    historyLoadingAtom: jotai.PrimitiveAtom<boolean>;
    historyFilterChannelAtom: jotai.PrimitiveAtom<string>;
    historyFilterStatusAtom: jotai.PrimitiveAtom<string>;

    constructor(blockId: string, nodeModel: BlockNodeModel, tabModel: TabModel) {
        this.nodeModel = nodeModel;
        this.tabModel = tabModel;
        this.viewType = "proxy";
        this.blockId = blockId;
        this.blockAtom = WOS.getWaveObjectAtom<Block>(`block:${blockId}`);
        this.manageConnection = jotai.atom(true);
        this.filterOutNowsh = jotai.atom(true);
        this.connectionImmediate = jotai.atom<string>((get) => {
            return get(this.blockAtom)?.meta?.connection;
        });
        this.connStatus = jotai.atom((get) => {
            const connName = get(this.connectionImmediate);
            const connAtom = getConnStatusAtom(connName);
            return get(connAtom);
        });

        // Initialize atoms
        this.proxyStatusAtom = jotai.atom<ProxyStatus | null>(null);
        this.channelsAtom = jotai.atom<ChannelConfig[]>([]);
        this.responseChannelsAtom = jotai.atom<ChannelConfig[]>([]);
        this.geminiChannelsAtom = jotai.atom<ChannelConfig[]>([]);
        this.metricsAtom = jotai.atom<{ [key: string]: ChannelMetrics }>({});
        this.globalStatsAtom = jotai.atom<GlobalStats | null>(null);
        this.loadingAtom = jotai.atom<boolean>(true);
        this.selectedTabAtom = jotai.atom<ChannelType>("messages");
        this.editingChannelAtom = jotai.atom<EditingChannel | null>(null);
        this.isFormOpenAtom = jotai.atom<boolean>(false);

        // Initialize history atoms
        this.historyRecordsAtom = jotai.atom<RequestRecord[]>([]);
        this.historyTotalCountAtom = jotai.atom<number>(0);
        this.historyLoadingAtom = jotai.atom<boolean>(false);
        this.historyFilterChannelAtom = jotai.atom<string>("");
        this.historyFilterStatusAtom = jotai.atom<string>("all");

        this.viewIcon = jotai.atom(() => "server");
        this.viewName = jotai.atom((get) => {
            const lang = getAppLanguageFromSettings(get(atoms.settingsAtom));
            return tCore(lang, "proxy.viewName");
        });
        this.viewText = jotai.atom(() => "");
        this.endIconButtons = jotai.atom((get) => {
            const lang = getAppLanguageFromSettings(get(atoms.settingsAtom));
            return [
                {
                    elemtype: "iconbutton",
                    icon: "minus",
                    title: tCore(lang, "proxy.dock.minimize"),
                    click: () => {
                        minimizeProxyBlock(this.blockId);
                    },
                },
            ];
        });
    }

    get viewComponent(): ViewComponent {
        return ProxyView;
    }

    private getRouteForConn(connName: string): string | null {
        if (isBlank(connName) || isLocalConnName(connName) || connName.startsWith("aws:")) {
            return null;
        }
        return makeConnRoute(connName);
    }

    private getCurrentRoute(): string | null {
        return this.getRouteForConn(globalStore.get(this.connectionImmediate));
    }

    private showProxyError(actionLabel: string, err: unknown) {
        const lang = getAppLanguageFromSettings(globalStore.get(atoms.settingsAtom));
        const msg = err instanceof Error ? err.message : String(err);
        pushFlashError({
            id: "",
            icon: "exclamation-triangle",
            title: tCore(lang, "proxy.title"),
            message: `${actionLabel}: ${msg}`,
            expiration: Date.now() + 5000,
        });
    }

    async syncLocalChannelsToRemote() {
        const route = this.getCurrentRoute();
        if (!route) {
            return;
        }

        const lang = getAppLanguageFromSettings(globalStore.get(atoms.settingsAtom));
        const connName = globalStore.get(this.connectionImmediate) || "remote";

        try {
            for (const channelType of allChannelTypes) {
                const [remoteChannels, localChannels] = await Promise.all([
                    ProxyRpcApi.channelList(channelType, route),
                    ProxyRpcApi.channelList(channelType, null),
                ]);

                // Overwrite remote channels with local ones.
                for (let i = remoteChannels.length - 1; i >= 0; i--) {
                    await ProxyRpcApi.channelDelete(channelType, i, route);
                }
                for (const ch of localChannels) {
                    await ProxyRpcApi.channelCreate(channelType, ch, route);
                }
            }

            pushNotification({
                icon: "server",
                type: "info",
                title: tCore(lang, "proxy.title"),
                message: tCore(lang, "proxy.sync.syncedAll", { connection: connName }),
                timestamp: new Date().toISOString(),
                expiration: Date.now() + 5000,
            });

            await this.loadChannels("messages");
            await this.loadChannels("responses");
            await this.loadChannels("gemini");
            await this.loadProxyStatus();
        } catch (e) {
            this.showProxyError(tCore(lang, "proxy.sync.syncFromLocal"), e);
        }
    }

    async loadProxyStatus() {
        globalStore.set(this.loadingAtom, true);
        try {
            const status = await ProxyRpcApi.proxyStatus(this.getCurrentRoute());
            globalStore.set(this.proxyStatusAtom, status);
        } catch (e) {
            console.error("[Proxy] Error loading status:", e);
            // Fallback to default status on error
            globalStore.set(this.proxyStatusAtom, {
                running: false,
                port: 3000,
                version: "1.0.0",
                channelCount: 0,
            });
        } finally {
            globalStore.set(this.loadingAtom, false);
        }
    }

    async startProxy() {
        try {
            const route = this.getCurrentRoute();
            await ProxyRpcApi.proxyStart(route);
            // Refresh status after starting
            await this.loadProxyStatus();
        } catch (e) {
            console.error("[Proxy] Error starting proxy:", e);
            const lang = getAppLanguageFromSettings(globalStore.get(atoms.settingsAtom));
            this.showProxyError(tCore(lang, "proxy.start"), e);
        }
    }

    async stopProxy() {
        try {
            await ProxyRpcApi.proxyStop(this.getCurrentRoute());
            // Refresh status after stopping
            await this.loadProxyStatus();
        } catch (e) {
            console.error("[Proxy] Error stopping proxy:", e);
            const lang = getAppLanguageFromSettings(globalStore.get(atoms.settingsAtom));
            this.showProxyError(tCore(lang, "proxy.stop"), e);
        }
    }

    async setPort(port: number) {
        try {
            await ProxyRpcApi.proxySetPort(port, this.getCurrentRoute());
            await this.loadProxyStatus();
        } catch (e) {
            console.error("[Proxy] Error setting port:", e);
            const lang = getAppLanguageFromSettings(globalStore.get(atoms.settingsAtom));
            this.showProxyError(tCore(lang, "proxy.port"), e);
        }
    }

    async loadChannels(channelType: ChannelType) {
        try {
            const channels = await ProxyRpcApi.channelList(channelType, this.getCurrentRoute());
            switch (channelType) {
                case "messages":
                    globalStore.set(this.channelsAtom, channels);
                    break;
                case "responses":
                    globalStore.set(this.responseChannelsAtom, channels);
                    break;
                case "gemini":
                    globalStore.set(this.geminiChannelsAtom, channels);
                    break;
            }
        } catch (e) {
            console.error("[Proxy] Error loading channels:", e);
            // Avoid showing stale channel lists when a routed call fails.
            switch (channelType) {
                case "messages":
                    globalStore.set(this.channelsAtom, []);
                    break;
                case "responses":
                    globalStore.set(this.responseChannelsAtom, []);
                    break;
                case "gemini":
                    globalStore.set(this.geminiChannelsAtom, []);
                    break;
            }
        }
    }

    async addChannel(channelType: ChannelType, channel: ChannelConfig) {
        try {
            await ProxyRpcApi.channelCreate(channelType, channel, this.getCurrentRoute());
            await this.loadChannels(channelType);
            // Refresh status to update channel count
            await this.loadProxyStatus();
        } catch (e) {
            console.error("[Proxy] Error adding channel:", e);
            const lang = getAppLanguageFromSettings(globalStore.get(atoms.settingsAtom));
            this.showProxyError(tCore(lang, "proxy.addChannel"), e);
        }
    }

    async updateChannel(channelType: ChannelType, index: number, channel: ChannelConfig) {
        try {
            await ProxyRpcApi.channelUpdate(channelType, index, channel, this.getCurrentRoute());
            await this.loadChannels(channelType);
        } catch (e) {
            console.error("[Proxy] Error updating channel:", e);
            const lang = getAppLanguageFromSettings(globalStore.get(atoms.settingsAtom));
            this.showProxyError(tCore(lang, "proxy.editChannel"), e);
        }
    }

    async bulkUpdateChannels(updates: Array<{ channelType: ChannelType; index: number; channel: ChannelConfig }>) {
        if (!updates || updates.length === 0) {
            return;
        }
        const route = this.getCurrentRoute();
        try {
            for (const update of updates) {
                await ProxyRpcApi.channelUpdate(update.channelType, update.index, update.channel, route);
            }
            const channelTypes = Array.from(new Set(updates.map((u) => u.channelType)));
            for (const channelType of channelTypes) {
                await this.loadChannels(channelType);
            }
        } catch (e) {
            console.error("[Proxy] Error bulk updating channels:", e);
            const lang = getAppLanguageFromSettings(globalStore.get(atoms.settingsAtom));
            this.showProxyError(tCore(lang, "proxy.channel.priority"), e);
        }
    }

    async deleteChannel(channelType: ChannelType, index: number) {
        try {
            await ProxyRpcApi.channelDelete(channelType, index, this.getCurrentRoute());
            await this.loadChannels(channelType);
            // Refresh status to update channel count
            await this.loadProxyStatus();
        } catch (e) {
            console.error("[Proxy] Error deleting channel:", e);
            const lang = getAppLanguageFromSettings(globalStore.get(atoms.settingsAtom));
            this.showProxyError(tCore(lang, "proxy.deleteChannel"), e);
        }
    }

    async pingChannel(channelType: ChannelType, index: number): Promise<{ success: boolean; latencyMs: number }> {
        try {
            const result = await ProxyRpcApi.channelPing(channelType, index, this.getCurrentRoute());
            return result;
        } catch (e) {
            console.error("[Proxy] Error pinging channel:", e);
            return { success: false, latencyMs: 0 };
        }
    }

    async loadMetrics(channelId?: string) {
        try {
            const metricsList = await ProxyRpcApi.channelMetrics(channelId, this.getCurrentRoute());
            const metricsMap: { [key: string]: ChannelMetrics } = {};
            for (const m of metricsList) {
                metricsMap[m.channelId] = m;
            }
            globalStore.set(this.metricsAtom, metricsMap);
        } catch (e) {
            console.error("[Proxy] Error loading metrics:", e);
        }
    }

    async loadGlobalStats() {
        try {
            const stats = await ProxyRpcApi.globalStats(this.getCurrentRoute());
            globalStore.set(this.globalStatsAtom, stats);
        } catch (e) {
            console.error("[Proxy] Error loading global stats:", e);
        }
    }

    async resetScheduler(channelId: string) {
        try {
            await ProxyRpcApi.schedulerReset(channelId, this.getCurrentRoute());
            // Refresh metrics after reset
            await this.loadMetrics();
        } catch (e) {
            console.error("[Proxy] Error resetting scheduler:", e);
        }
    }

    openAddChannelForm() {
        globalStore.set(this.editingChannelAtom, null);
        globalStore.set(this.isFormOpenAtom, true);
    }

    openEditChannelForm(channelType: ChannelType, index: number, channel: ChannelConfig) {
        globalStore.set(this.editingChannelAtom, { channelType, index, channel });
        globalStore.set(this.isFormOpenAtom, true);
    }

    closeChannelForm() {
        globalStore.set(this.editingChannelAtom, null);
        globalStore.set(this.isFormOpenAtom, false);
    }

    async loadHistory(limit = 50, offset = 0, channelId?: string, status?: string) {
        globalStore.set(this.historyLoadingAtom, true);
        try {
            const currentStatus = status ?? globalStore.get(this.historyFilterStatusAtom) ?? "all";
            const result = await ProxyRpcApi.requestHistory(limit, offset, channelId, currentStatus, this.getCurrentRoute());
            globalStore.set(this.historyRecordsAtom, result.records);
            globalStore.set(this.historyTotalCountAtom, result.totalCount);
        } catch (e) {
            console.error("[Proxy] Error loading history:", e);
        } finally {
            globalStore.set(this.historyLoadingAtom, false);
        }
    }

    setHistoryFilter(channelId: string) {
        globalStore.set(this.historyFilterChannelAtom, channelId);
        const status = globalStore.get(this.historyFilterStatusAtom) ?? "all";
        this.loadHistory(50, 0, channelId || undefined, status);
    }

    setHistoryStatusFilter(status: string) {
        globalStore.set(this.historyFilterStatusAtom, status);
        const channelId = globalStore.get(this.historyFilterChannelAtom) || undefined;
        this.loadHistory(50, 0, channelId, status);
    }

    async clearHistory() {
        await ProxyRpcApi.historyClear(this.getCurrentRoute());
        const channelId = globalStore.get(this.historyFilterChannelAtom) || undefined;
        const status = globalStore.get(this.historyFilterStatusAtom) ?? "all";
        await this.loadHistory(50, 0, channelId, status);
    }

    getSettingsMenuItems(): ContextMenuItem[] {
        return [];
    }
}

// Forward declaration - actual component will be imported
let ProxyView: ViewComponent;

export function setProxyViewComponent(component: ViewComponent) {
    ProxyView = component;
}
