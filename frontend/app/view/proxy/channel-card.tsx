// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import clsx from "clsx";
import * as React from "react";
import { useT } from "@/app/i18n/i18n";
import { ChannelConfig } from "./proxy-model";

type ChannelCardProps = {
    channel: ChannelConfig;
    index: number;
    onEdit: () => void;
    onDelete: () => void;
    onPing: () => Promise<{ success: boolean; latencyMs: number }>;
    onToggleEnabled: () => Promise<void>;
    reorderEnabled?: boolean;
    onReorderDragStart?: (e: React.DragEvent<HTMLDivElement>) => void;
    onReorderDragEnd?: (e: React.DragEvent<HTMLDivElement>) => void;
};

export const ChannelCard = React.memo(
    ({
        channel,
        index,
        onEdit,
        onDelete,
        onPing,
        onToggleEnabled,
        reorderEnabled,
        onReorderDragStart,
        onReorderDragEnd,
    }: ChannelCardProps) => {
    const t = useT();
    const [pingStatus, setPingStatus] = React.useState<"idle" | "loading" | "success" | "error">("idle");
    const [pingLatency, setPingLatency] = React.useState<number | null>(null);
    const [toggleBusy, setToggleBusy] = React.useState(false);

    const handlePing = React.useCallback(async () => {
        setPingStatus("loading");
        setPingLatency(null);
        try {
            const result = await onPing();
            setPingStatus(result.success ? "success" : "error");
            setPingLatency(result.latencyMs);
            setTimeout(() => {
                setPingStatus("idle");
                setPingLatency(null);
            }, 3000);
        } catch {
            setPingStatus("error");
            setTimeout(() => setPingStatus("idle"), 3000);
        }
    }, [onPing]);

    const handleToggleEnabled = React.useCallback(async () => {
        if (toggleBusy) return;
        setToggleBusy(true);
        try {
            await onToggleEnabled();
        } finally {
            setToggleBusy(false);
        }
    }, [onToggleEnabled, toggleBusy]);

    const getServiceIcon = (serviceType: string) => {
        switch (serviceType?.toLowerCase()) {
            case "claude":
                return "fa-robot";
            case "openai":
                return "fa-brain";
            case "gemini":
                return "fa-gem";
            default:
                return "fa-server";
        }
    };

    const getStatusClass = (status: string) => {
        switch (status?.toLowerCase()) {
            case "active":
                return "status-active";
            case "suspended":
                return "status-suspended";
            case "disabled":
                return "status-disabled";
            default:
                return "status-unknown";
        }
    };

    const getStatusLabel = (status: string) => {
        switch (status?.toLowerCase()) {
            case "active":
                return t("proxy.channel.active");
            case "suspended":
                return t("proxy.channel.suspended");
            case "disabled":
                return t("proxy.channel.disabled");
            default:
                return t("common.unknown");
        }
    };

    const maskApiKey = (key: string) => {
        const trimmed = (key || "").trim();
        if (!trimmed || trimmed.length < 8) return "****";
        return trimmed.substring(0, 4) + "..." + trimmed.substring(trimmed.length - 4);
    };

    const totalKeys = channel.apiKeys?.length || 0;
    const enabledKeys = channel.apiKeys?.filter((k) => k.enabled && k.key?.trim()).length || 0;
    const sampleKey =
        channel.apiKeys?.find((k) => k.enabled && k.key?.trim())?.key ||
        channel.apiKeys?.find((k) => k.key?.trim())?.key ||
        "";
    const isDisabled = (channel.status || "").toLowerCase() === "disabled";

    return (
        <div className={clsx("channel-card", getStatusClass(channel.status))}>
            <div className="channel-card-header">
                <div className="channel-info">
                    {reorderEnabled && (
                        <div
                            className="channel-drag-handle"
                            draggable
                            onDragStart={onReorderDragStart}
                            onDragEnd={onReorderDragEnd}
                            title={t("proxy.channel.dragToReorder")}
                        >
                            <i className="fa fa-grip-vertical" />
                        </div>
                    )}
                    <div className="channel-icon">
                        <i className={clsx("fa", getServiceIcon(channel.serviceType))} />
                    </div>
                    <div className="channel-details">
                        <div className="channel-name">
                            <span className="name">
                                {channel.name || t("proxy.channel.channelWithIndex", { index: index + 1 })}
                            </span>
                            <span className={clsx("status-badge", getStatusClass(channel.status))}>
                                {getStatusLabel(channel.status)}
                            </span>
                        </div>
                        <div className="channel-meta">
                            <span className="service-type">{channel.serviceType}</span>
                            <span className="priority">
                                {t("proxy.channel.priority")}: {channel.priority}
                            </span>
                        </div>
                    </div>
                </div>
                <div className="channel-actions">
                    <button
                        className="action-btn"
                        onClick={handleToggleEnabled}
                        disabled={toggleBusy}
                        title={isDisabled ? t("proxy.channel.enableHint") : t("proxy.channel.disableHint")}
                    >
                        {toggleBusy ? <i className="fa fa-spinner fa-spin" /> : <i className={clsx("fa", isDisabled ? "fa-play" : "fa-pause")} />}
                    </button>
                    <button
                        className={clsx("action-btn", pingStatus)}
                        onClick={handlePing}
                        disabled={pingStatus === "loading"}
                        title={pingLatency !== null ? t("proxy.latency", { ms: pingLatency }) : t("proxy.testConnection")}
                    >
                        {pingStatus === "loading" ? (
                            <i className="fa fa-spinner fa-spin" />
                        ) : pingStatus === "success" ? (
                            <span className="ping-result">
                                <i className="fa fa-check text-success" />
                                {pingLatency !== null && <span className="latency">{pingLatency}ms</span>}
                            </span>
                        ) : pingStatus === "error" ? (
                            <i className="fa fa-times text-error" />
                        ) : (
                            <i className="fa fa-wifi" />
                        )}
                    </button>
                    <button className="action-btn" onClick={onEdit} title={t("proxy.editChannel")}>
                        <i className="fa fa-edit" />
                    </button>
                    <button className="action-btn action-danger" onClick={onDelete} title={t("proxy.deleteChannel")}>
                        <i className="fa fa-trash" />
                    </button>
                </div>
            </div>

            <div className="channel-card-body">
                <div className="channel-field">
                    <span className="field-label">{t("proxy.channel.baseUrl")}:</span>
                    <span className="field-value">{channel.baseUrl || "-"}</span>
                </div>
                {channel.baseUrls && channel.baseUrls.length > 0 && (
                    <div className="channel-field">
                        <span className="field-label">{t("proxy.channel.backupUrls")}:</span>
                        <span className="field-value">
                            {t("proxy.channel.configuredCount", { count: channel.baseUrls.length })}
                        </span>
                    </div>
                )}
                <div className="channel-field">
                    <span className="field-label">{t("proxy.channel.apiKeys")}:</span>
                    <span className="field-value">
                        {totalKeys > 0
                            ? enabledKeys > 0
                                ? t("proxy.channel.apiKeysSummaryEnabled", {
                                      enabled: enabledKeys,
                                      total: totalKeys,
                                      sample: maskApiKey(sampleKey),
                                  })
                                : t("proxy.channel.apiKeysAllPaused", { total: totalKeys })
                            : t("proxy.channel.noneConfigured")}
                    </span>
                </div>
                {channel.modelMapping && Object.keys(channel.modelMapping).length > 0 && (
                    <div className="channel-field">
                        <span className="field-label">{t("proxy.channel.modelMapping")}:</span>
                        <span className="field-value">
                            {t("proxy.channel.modelMappingSummary", { count: Object.keys(channel.modelMapping).length })}
                        </span>
                    </div>
                )}
                {channel.description && (
                    <div className="channel-field">
                        <span className="field-label">{t("proxy.channel.description")}:</span>
                        <span className="field-value">{channel.description}</span>
                    </div>
                )}
            </div>

            {channel.lowQuality && (
                <div className="channel-warning">
                    <i className="fa fa-exclamation-triangle" />
                    <span>{t("proxy.channel.lowQuality")}</span>
                </div>
            )}
        </div>
    );
});
