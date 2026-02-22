// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import clsx from "clsx";
import * as jotai from "jotai";
import * as React from "react";
import { OverlayScrollbarsComponent } from "overlayscrollbars-react";

import { useT } from "@/app/i18n/i18n";
import { ProxyViewModel, ChannelConfig, ChannelType, setProxyViewComponent } from "./proxy-model";
import { ChannelCard } from "./channel-card";
import { ChannelForm } from "./channel-form";
import { StatusBadge } from "./status-badge";
import { MetricsDashboard, CircuitBreakerStatus } from "./metrics-chart";
import { HistoryList } from "./history-list";
import { removeProxyDockItem } from "./proxy-dock-model";
import "./proxy.scss";

type TabType = ChannelType | "metrics" | "history";

type ProxyViewProps = {
    blockId: string;
    model: ProxyViewModel;
};

type DisplayChannel = {
    channelType: ChannelType;
    index: number;
    channel: ChannelConfig;
};

function moveArrayItem<T>(arr: readonly T[], fromIndex: number, toIndex: number): T[] {
    if (fromIndex === toIndex) {
        return [...arr];
    }
    const copy = [...arr];
    const [item] = copy.splice(fromIndex, 1);
    copy.splice(toIndex, 0, item);
    return copy;
}

const ProxyView = React.memo(({ model, blockId }: ProxyViewProps) => {
    const t = useT();
    const loading = jotai.useAtomValue(model.loadingAtom);
    const connection = jotai.useAtomValue(model.connectionImmediate);
    const connStatus = jotai.useAtomValue(model.connStatus);
    const proxyStatus = jotai.useAtomValue(model.proxyStatusAtom);
    const selectedTab = jotai.useAtomValue(model.selectedTabAtom);
    const setSelectedTab = jotai.useSetAtom(model.selectedTabAtom);
    const channels = jotai.useAtomValue(model.channelsAtom);
    const responseChannels = jotai.useAtomValue(model.responseChannelsAtom);
    const geminiChannels = jotai.useAtomValue(model.geminiChannelsAtom);
    const metrics = jotai.useAtomValue(model.metricsAtom);
    const globalStats = jotai.useAtomValue(model.globalStatsAtom);
    const isFormOpen = jotai.useAtomValue(model.isFormOpenAtom);
    const editingChannel = jotai.useAtomValue(model.editingChannelAtom);

    // History state
    const historyRecords = jotai.useAtomValue(model.historyRecordsAtom);
    const historyTotalCount = jotai.useAtomValue(model.historyTotalCountAtom);
    const historyLoading = jotai.useAtomValue(model.historyLoadingAtom);
    const historyFilterChannel = jotai.useAtomValue(model.historyFilterChannelAtom);
    const historyFilterStatus = jotai.useAtomValue(model.historyFilterStatusAtom);

    const [activeTab, setActiveTab] = React.useState<TabType>("messages");
    const channelSelectRef = React.useRef<HTMLSelectElement>(null);
    const portInputRef = React.useRef<HTMLInputElement>(null);
    const [draggingIndex, setDraggingIndex] = React.useState<number | null>(null);
    const [dragOverIndex, setDragOverIndex] = React.useState<number | null>(null);
    const [portDraft, setPortDraft] = React.useState<string>("3000");
    const [isAddressCopied, setIsAddressCopied] = React.useState(false);
    const copyResetTimerRef = React.useRef<number | null>(null);

    React.useEffect(() => {
        removeProxyDockItem(connection);
    }, [connection]);

    const parsedPort = React.useMemo(() => Number(portDraft), [portDraft]);
    const isPortValid = React.useMemo(
        () => Number.isInteger(parsedPort) && parsedPort >= 1 && parsedPort <= 65535,
        [parsedPort]
    );
    const isPortDirty = React.useMemo(() => {
        if (proxyStatus?.port == null) {
            return false;
        }
        return parsedPort !== proxyStatus.port;
    }, [parsedPort, proxyStatus?.port]);

    const proxyAddress = React.useMemo(() => {
        if (proxyStatus?.port != null) {
            return `http://127.0.0.1:${proxyStatus.port}`;
        }
        const fallbackPort = Number.isInteger(parsedPort) && parsedPort >= 1 && parsedPort <= 65535 ? parsedPort : 3000;
        return `http://127.0.0.1:${fallbackPort}`;
    }, [proxyStatus?.port, parsedPort]);

    React.useEffect(() => {
        if (proxyStatus?.port == null) {
            return;
        }
        // Avoid clobbering the input while the user is typing.
        if (portInputRef.current && document.activeElement === portInputRef.current) {
            return;
        }
        setPortDraft(String(proxyStatus.port));
    }, [proxyStatus?.port]);

    React.useEffect(() => {
        return () => {
            if (copyResetTimerRef.current != null) {
                window.clearTimeout(copyResetTimerRef.current);
            }
        };
    }, []);

    React.useEffect(() => {
        if (connStatus?.status !== "connected") {
            return;
        }
        model.loadProxyStatus();
    }, [connection, connStatus?.status]);

    React.useEffect(() => {
        if (connStatus?.status !== "connected") {
            return;
        }
        if (activeTab === "metrics") {
            model.loadMetrics();
            model.loadGlobalStats();
        } else if (activeTab === "history") {
            model.loadHistory();
        } else {
            setSelectedTab(activeTab as ChannelType);
            // Keep all channel lists warm so the UI can classify by provider type
            // even if the user previously added channels under a different tab.
            model.loadChannels("messages");
            model.loadChannels("responses");
            model.loadChannels("gemini");
        }
    }, [activeTab, connection, connStatus?.status]);

    const handleStartStop = React.useCallback(() => {
        if (connStatus?.status !== "connected") {
            return;
        }
        if (proxyStatus?.running) {
            model.stopProxy();
        } else {
            model.startProxy();
        }
    }, [connStatus?.status, proxyStatus?.running, model]);

    const handleSavePort = React.useCallback(async () => {
        if (connStatus?.status !== "connected") {
            return;
        }
        if (!isPortValid || !isPortDirty) {
            return;
        }
        await model.setPort(parsedPort);
    }, [connStatus?.status, isPortValid, isPortDirty, parsedPort, model]);

    const handleCopyProxyAddress = React.useCallback(async () => {
        try {
            await navigator.clipboard.writeText(proxyAddress);
            setIsAddressCopied(true);
            if (copyResetTimerRef.current != null) {
                window.clearTimeout(copyResetTimerRef.current);
            }
            copyResetTimerRef.current = window.setTimeout(() => {
                setIsAddressCopied(false);
            }, 1500);
        } catch (error) {
            console.error("failed to copy proxy address", error);
        }
    }, [proxyAddress]);

    const handleSyncFromLocal = React.useCallback(async () => {
        if (connStatus?.status !== "connected") {
            return;
        }
        if (!connection || connection === "local") {
            return;
        }
        if (!confirm(t("proxy.sync.confirmOverwriteRemote", { connection }))) {
            return;
        }
        await model.syncLocalChannelsToRemote();
    }, [connStatus?.status, connection, model, t]);

    const handleAddChannel = React.useCallback(() => {
        model.openAddChannelForm();
    }, [model]);

    const handleEditChannel = React.useCallback(
        (channelType: ChannelType, index: number, channel: ChannelConfig) => {
            model.openEditChannelForm(channelType, index, channel);
        },
        [model]
    );

    const handleDeleteChannel = React.useCallback(
        (channelType: ChannelType, index: number) => {
            model.deleteChannel(channelType, index);
        },
        [model]
    );

    const handlePingChannel = React.useCallback(
        async (channelType: ChannelType, index: number) => {
            return await model.pingChannel(channelType, index);
        },
        [model]
    );

    const handleToggleChannelEnabled = React.useCallback(
        async (channelType: ChannelType, index: number, channel: ChannelConfig) => {
            const status = (channel.status || "active").toLowerCase();
            const nextStatus = status === "disabled" ? "active" : "disabled";
            await model.updateChannel(channelType, index, { ...channel, status: nextStatus });
        },
        [model]
    );

    const handleFormSubmit = React.useCallback(
        async (channel: ChannelConfig) => {
            if (editingChannel) {
                await model.updateChannel(editingChannel.channelType, editingChannel.index, channel);
            } else {
                await model.addChannel(selectedTab, channel);
            }
            model.closeChannelForm();
        },
        [model, selectedTab, editingChannel]
    );

    const handleFormClose = React.useCallback(() => {
        model.closeChannelForm();
    }, [model]);

    const handleResetCircuit = React.useCallback(
        (channelId: string) => {
            model.resetScheduler(channelId);
        },
        [model]
    );

    const handleRefreshMetrics = React.useCallback(() => {
        model.loadMetrics();
        model.loadGlobalStats();
    }, [model]);

    const handleHistoryFilterChange = React.useCallback(
        (channelId: string) => {
            model.setHistoryFilter(channelId);
        },
        [model]
    );

    const handleRefreshHistory = React.useCallback(() => {
        const filterChannel = historyFilterChannel || undefined;
        model.loadHistory(50, 0, filterChannel, historyFilterStatus);
    }, [model, historyFilterChannel, historyFilterStatus]);

    const handleHistoryStatusFilterChange = React.useCallback(
        (status: string) => {
            model.setHistoryStatusFilter(status);
        },
        [model]
    );

    const handleClearHistory = React.useCallback(async () => {
        await model.clearHistory();
    }, [model]);

    const desiredServiceType = React.useMemo(() => {
        switch (selectedTab) {
            case "responses":
                return "openai";
            case "gemini":
                return "gemini";
            case "messages":
            default:
                return "claude";
        }
    }, [selectedTab]);

    const currentChannels: DisplayChannel[] = React.useMemo(() => {
        const channelTypeOrder: Record<ChannelType, number> = { messages: 0, responses: 1, gemini: 2 };
        const normalizeServiceType = (channelType: ChannelType, channel: ChannelConfig) => {
            const normalized = channel.serviceType?.trim().toLowerCase();
            if (normalized) {
                return normalized;
            }
            switch (channelType) {
                case "responses":
                    return "openai";
                case "gemini":
                    return "gemini";
                case "messages":
                default:
                    return "claude";
            }
        };

        const all: DisplayChannel[] = [
            ...channels.map((channel, index) => ({ channelType: "messages" as const, index, channel })),
            ...responseChannels.map((channel, index) => ({ channelType: "responses" as const, index, channel })),
            ...geminiChannels.map((channel, index) => ({ channelType: "gemini" as const, index, channel })),
        ];

        const filtered = all.filter((entry) => normalizeServiceType(entry.channelType, entry.channel) === desiredServiceType);
        const effectivePriority = (entry: DisplayChannel) => {
            const p = entry.channel.priority || 0;
            if (p > 0) {
                return p;
            }
            // Keep UI ordering consistent with backend defaults:
            // when priority==0, backend uses the channel index for ordering.
            return entry.index;
        };
        return filtered.sort((a, b) => {
            const pa = effectivePriority(a);
            const pb = effectivePriority(b);
            if (pa !== pb) {
                return pa - pb;
            }
            const ta = channelTypeOrder[a.channelType] ?? 0;
            const tb = channelTypeOrder[b.channelType] ?? 0;
            if (ta !== tb) {
                return ta - tb;
            }
            return a.index - b.index;
        });
    }, [channels, responseChannels, geminiChannels, desiredServiceType]);

    const handleReorder = React.useCallback(
        async (from: number, to: number) => {
            if (from === to || from < 0 || to < 0 || from >= currentChannels.length || to >= currentChannels.length) {
                return;
            }
            const nextOrder = moveArrayItem(currentChannels, from, to);
            const updates = nextOrder.map((entry, idx) => ({
                channelType: entry.channelType,
                index: entry.index,
                channel: {
                    ...entry.channel,
                    priority: idx + 1,
                },
            }));
            const changed = updates.filter((u) => u.channel.priority !== (currentChannels.find((c) => c.channelType === u.channelType && c.index === u.index)?.channel.priority || 0));
            await model.bulkUpdateChannels(changed);
        },
        [currentChannels, model]
    );

    if (connStatus?.status !== "connected") {
        return null;
    }

    if (loading) {
        return (
            <div className="proxy-view proxy-loading">
                <div className="loading-spinner">
                    <i className="fa fa-spinner fa-spin" />
                    <span>{t("common.loading")}</span>
                </div>
            </div>
        );
    }

    return (
        <div className="proxy-view">
            {/* Header Section */}
            <div className="proxy-header">
                <div className="proxy-status-section">
                    <div className="proxy-title">
                        <i className="fa fa-server" />
                        <span>{t("proxy.title")}</span>
                        <StatusBadge running={proxyStatus?.running ?? false} />
                    </div>
                    <div className="proxy-info">
                        {proxyStatus?.running ? (
                            <>
                                <span className="info-item">
                                    <i className="fa fa-network-wired" />
                                    {t("proxy.port")}: {proxyStatus.port}
                                </span>
                                <span className="info-item">
                                    <i className="fa fa-clock" />
                                    {t("proxy.uptime")}: {proxyStatus.uptime || "0s"}
                                </span>
                                <span className="info-item">
                                    <i className="fa fa-layer-group" />
                                    {t("proxy.channels")}: {proxyStatus.channelCount}
                                </span>
                            </>
                        ) : (
                            <span className="info-item text-muted">{t("proxy.serviceNotRunning")}</span>
                        )}
                    </div>
                </div>
                <div className="proxy-actions">
                    <div className="proxy-port-config" title={proxyStatus?.running ? t("proxy.portChangeRestarts") : undefined}>
                        <label htmlFor="proxy-port-input">{t("proxy.port")}</label>
                        <input
                            id="proxy-port-input"
                            ref={portInputRef}
                            type="number"
                            inputMode="numeric"
                            min={1}
                            max={65535}
                            step={1}
                            value={portDraft}
                            onChange={(e) => setPortDraft(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === "Enter") {
                                    e.preventDefault();
                                    handleSavePort();
                                }
                            }}
                            className={clsx(!isPortValid && portDraft.trim() !== "" && "invalid")}
                        />
                        <button
                            className={clsx("proxy-btn", "btn-secondary", "btn-small")}
                            onClick={handleSavePort}
                            disabled={!isPortValid || !isPortDirty}
                        >
                            <i className="fa fa-save" />
                            {t("common.save")}
                        </button>
                    </div>
                    <button className={clsx("proxy-btn", "btn-secondary", "btn-small")} onClick={handleCopyProxyAddress} title={proxyAddress}>
                        <i className={clsx("fa", isAddressCopied ? "fa-check" : "fa-link")} />
                        {isAddressCopied ? t("proxy.copyAddress.copied") : t("proxy.copyAddress")}
                    </button>
                    {connection && connection !== "local" && (
                        <button className={clsx("proxy-btn", "btn-secondary")} onClick={handleSyncFromLocal}>
                            <i className="fa fa-download" />
                            {t("proxy.sync.syncFromLocal")}
                        </button>
                    )}
                    <button
                        className={clsx("proxy-btn", proxyStatus?.running ? "btn-danger" : "btn-primary")}
                        onClick={handleStartStop}
                        disabled={connStatus?.status !== "connected"}
                    >
                        <i className={clsx("fa", proxyStatus?.running ? "fa-stop" : "fa-play")} />
                        {proxyStatus?.running ? t("proxy.stop") : t("proxy.start")}
                    </button>
                </div>
            </div>

            {/* Tab Navigation */}
            <div className="proxy-tabs">
                <div
                    className={clsx(
                        "tab-select",
                        (activeTab === "messages" || activeTab === "responses" || activeTab === "gemini") && "active"
                    )}
                    title={t("proxy.channels")}
                    onClick={(e) => {
                        if ((e.target as HTMLElement | null)?.tagName?.toLowerCase() === "select") {
                            return;
                        }
                        // If we're on a non-channel tab (metrics/history), clicking the tab should
                        // switch back to the currently selected channel list without requiring a selection change.
                        if (activeTab === "metrics" || activeTab === "history") {
                            setActiveTab(selectedTab);
                            return;
                        }
                        // When already on the channel list, clicking the dropdown arrow (or the container)
                        // should open the select.
                        channelSelectRef.current?.focus();
                        channelSelectRef.current?.click();
                    }}
                >
                    <i className="fa fa-layer-group" />
                    <select
                        ref={channelSelectRef}
                        value={selectedTab}
                        onChange={(e) => setActiveTab(e.target.value as ChannelType)}
                        aria-label={t("proxy.channels")}
                    >
                        <option value="messages">{t("proxy.tab.messages")}</option>
                        <option value="responses">{t("proxy.tab.responses")}</option>
                        <option value="gemini">{t("proxy.tab.gemini")}</option>
                    </select>
                    <i className="fa fa-chevron-down" />
                </div>
                <button
                    className={clsx("tab-btn", activeTab === "metrics" && "active")}
                    onClick={() => setActiveTab("metrics")}
                >
                    <i className="fa fa-chart-line" />
                    {t("proxy.tab.metrics")}
                </button>
                <button
                    className={clsx("tab-btn", activeTab === "history" && "active")}
                    onClick={() => setActiveTab("history")}
                >
                    <i className="fa fa-history" />
                    {t("proxy.tab.history")}
                </button>
            </div>

            {/* Content Area */}
            {activeTab === "metrics" ? (
                <div className="proxy-content">
                    <div className="metrics-panel">
                        <div className="metrics-section">
                            <div className="channel-toolbar">
                                <h3>
                                    <i className="fa fa-chart-pie" />
                                    {t("proxy.metrics.title")}
                                </h3>
                                <button className="metrics-refresh-btn" onClick={handleRefreshMetrics}>
                                    <i className="fa fa-sync-alt" />
                                    {t("proxy.metrics.refresh")}
                                </button>
                            </div>
                            <MetricsDashboard metrics={metrics} globalStats={globalStats} />
                        </div>

                        <div className="metrics-section">
                            <h3>
                                <i className="fa fa-exclamation-circle" />
                                {t("proxy.circuitBreaker.title")}
                            </h3>
                            <CircuitBreakerStatus metrics={metrics} onReset={handleResetCircuit} />
                        </div>
                    </div>
                </div>
            ) : activeTab === "history" ? (
                <div className="proxy-content">
                    <HistoryList
                        records={historyRecords}
                        totalCount={historyTotalCount}
                        loading={historyLoading}
                        filterChannel={historyFilterChannel}
                        filterStatus={historyFilterStatus}
                        onFilterChange={handleHistoryFilterChange}
                        onStatusFilterChange={handleHistoryStatusFilterChange}
                        onRefresh={handleRefreshHistory}
                        onClear={handleClearHistory}
                    />
                </div>
            ) : (
                <div className="proxy-content">
                    <div className="channel-toolbar">
                        <span className="channel-count">
                            {t("proxy.channelCount", { count: currentChannels.length })}
                        </span>
                        <button className="proxy-btn btn-secondary" onClick={handleAddChannel}>
                            <i className="fa fa-plus" />
                            {t("proxy.addChannel")}
                        </button>
                    </div>

                    <OverlayScrollbarsComponent className="channel-list-container" options={{ scrollbars: { autoHide: "leave" } }}>
                        {currentChannels.length === 0 ? (
                            <div className="empty-state">
                                <i className="fa fa-inbox" />
                                <p>{t("proxy.noChannels")}</p>
                                <button className="proxy-btn btn-primary" onClick={handleAddChannel}>
                                    <i className="fa fa-plus" />
                                    {t("proxy.addFirstChannel")}
                                </button>
                            </div>
                        ) : (
                            <div className="channel-list">
                                {currentChannels.map((entry, displayIndex) => (
                                    <div
                                        key={`${entry.channelType}:${entry.channel.id || entry.index}`}
                                        className={clsx(
                                            "channel-card-wrapper",
                                            draggingIndex === displayIndex && "dragging",
                                            dragOverIndex === displayIndex && "drag-over"
                                        )}
                                        draggable
                                        onDragStart={(e) => {
                                            setDraggingIndex(displayIndex);
                                            setDragOverIndex(displayIndex);
                                            e.dataTransfer.effectAllowed = "move";
                                            e.dataTransfer.setData("text/plain", String(displayIndex));
                                        }}
                                        onDragEnd={() => {
                                            setDraggingIndex(null);
                                            setDragOverIndex(null);
                                        }}
                                        onDragOver={(e) => {
                                            e.preventDefault();
                                            if (dragOverIndex !== displayIndex) {
                                                setDragOverIndex(displayIndex);
                                            }
                                            e.dataTransfer.dropEffect = "move";
                                        }}
                                        onDragLeave={() => {
                                            if (dragOverIndex === displayIndex) {
                                                setDragOverIndex(null);
                                            }
                                        }}
                                        onDrop={async (e) => {
                                            e.preventDefault();
                                            const from = draggingIndex;
                                            setDraggingIndex(null);
                                            setDragOverIndex(null);
                                            if (from == null) {
                                                return;
                                            }
                                            await handleReorder(from, displayIndex);
                                        }}
                                    >
                                        <ChannelCard
                                            channel={entry.channel}
                                            index={displayIndex}
                                            onEdit={() => handleEditChannel(entry.channelType, entry.index, entry.channel)}
                                            onDelete={() => handleDeleteChannel(entry.channelType, entry.index)}
                                            onPing={() => handlePingChannel(entry.channelType, entry.index)}
                                            onToggleEnabled={() => handleToggleChannelEnabled(entry.channelType, entry.index, entry.channel)}
                                        />
                                    </div>
                                ))}
                            </div>
                        )}
                    </OverlayScrollbarsComponent>
                </div>
            )}

            {/* Channel Form Modal */}
            {isFormOpen && (
                <ChannelForm
                    channel={editingChannel ? editingChannel.channel : null}
                    channelType={editingChannel ? editingChannel.channelType : selectedTab}
                    onSubmit={handleFormSubmit}
                    onClose={handleFormClose}
                />
            )}

        </div>
    );
});

// Register the component with the model
setProxyViewComponent(ProxyView);

export { ProxyView, ProxyViewModel };
