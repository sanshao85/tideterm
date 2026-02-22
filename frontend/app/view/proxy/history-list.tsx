// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import clsx from "clsx";
import * as React from "react";
import { useT } from "@/app/i18n/i18n";
import type { RequestRecord, ChannelType } from "./proxy-model";

type HistoryListProps = {
    records: RequestRecord[];
    totalCount: number;
    loading: boolean;
    filterChannel: string;
    filterStatus: string;
    onFilterChange: (channelId: string) => void;
    onStatusFilterChange: (status: string) => void;
    onRefresh: () => void;
    onClear: () => void;
};

const formatTimestamp = (timestamp: string): string => {
    const date = new Date(timestamp);
    return date.toLocaleString();
};

const formatLatency = (ms: number): string => {
    if (ms >= 1000) {
        return `${(ms / 1000).toFixed(2)}s`;
    }
    return `${ms}ms`;
};

const formatTokens = (count: number): string => {
    if (count >= 1000) {
        return `${(count / 1000).toFixed(1)}k`;
    }
    return String(count);
};

const getChannelTypeIcon = (channelType: ChannelType): string => {
    switch (channelType) {
        case "messages":
            return "fa-comments";
        case "responses":
            return "fa-reply";
        case "gemini":
            return "fa-gem";
        default:
            return "fa-server";
    }
};

const HistoryList = React.memo(
    ({ records, totalCount, loading, filterChannel, filterStatus, onFilterChange, onStatusFilterChange, onRefresh, onClear }: HistoryListProps) => {
        const t = useT();
        const statusSelectRef = React.useRef<HTMLSelectElement | null>(null);
        const [detailsRecord, setDetailsRecord] = React.useState<RequestRecord | null>(null);
        const handleFilterClear = React.useCallback(() => {
            onFilterChange("");
        }, [onFilterChange]);

        const handleStatusSelectChange = React.useCallback(
            (e: React.ChangeEvent<HTMLSelectElement>) => {
                onStatusFilterChange(e.target.value);
            },
            [onStatusFilterChange]
        );

        const handleOpenDetails = React.useCallback((record: RequestRecord) => {
            if (record.success) {
                return;
            }
            setDetailsRecord(record);
        }, []);

        const handleCloseDetails = React.useCallback(() => {
            setDetailsRecord(null);
        }, []);

        const formatChannelType = React.useCallback(
            (channelType: ChannelType): string => {
                switch (channelType) {
                    case "messages":
                        return t("proxy.tab.messages");
                    case "responses":
                        return t("proxy.tab.responses");
                    case "gemini":
                        return t("proxy.tab.gemini");
                    default:
                        return t("common.unknown");
                }
            },
            [t]
        );

        if (loading) {
            return (
                <div className="history-loading">
                    <i className="fa fa-spinner fa-spin" />
                    <span>{t("proxy.history.loading")}</span>
                </div>
            );
        }

        return (
            <div className="history-panel">
                <div className="history-toolbar">
                    <div className="history-info">
                        <span className="history-count">{t("proxy.history.recordCount", { count: totalCount })}</span>
                        <span className="history-retention">{t("proxy.history.retentionHint", { hours: 48 })}</span>
                        {filterChannel && (
                            <span className="history-filter-tag">
                                <i className="fa fa-filter" />
                                {t("proxy.history.filterChannel", { channel: filterChannel })}
                                <button className="filter-clear-btn" onClick={handleFilterClear}>
                                    <i className="fa fa-times" />
                                </button>
                            </span>
                        )}
                    </div>
                    <div className="history-actions">
                        <div
                            className="history-status-select"
                            title={t("proxy.history.filterStatus.label")}
                            onClick={(e) => {
                                if ((e.target as HTMLElement | null)?.tagName?.toLowerCase() === "select") {
                                    return;
                                }
                                statusSelectRef.current?.focus();
                                statusSelectRef.current?.click();
                            }}
                        >
                            <i className="fa fa-filter" />
                            <select
                                ref={statusSelectRef}
                                value={filterStatus}
                                onChange={handleStatusSelectChange}
                                aria-label={t("proxy.history.filterStatus.label")}
                            >
                                <option value="all">{t("proxy.history.filterStatus.all")}</option>
                                <option value="error">{t("proxy.history.filterStatus.error")}</option>
                                <option value="success">{t("proxy.history.filterStatus.success")}</option>
                            </select>
                            <i className="fa fa-chevron-down" />
                        </div>

                        <button className="proxy-btn btn-secondary" onClick={onRefresh}>
                            <i className="fa fa-sync-alt" />
                            {t("proxy.metrics.refresh")}
                        </button>
                        <button className="proxy-btn btn-danger" onClick={onClear}>
                            <i className="fa fa-trash" />
                            {t("proxy.history.clear")}
                        </button>
                    </div>
                </div>

                {records.length === 0 ? (
                    <div className="history-empty">
                        <i className="fa fa-history" />
                        <p>{t("proxy.history.emptyTitle")}</p>
                        <span className="text-muted">{t("proxy.history.emptyHint")}</span>
                    </div>
                ) : (
                    <div className="history-table-container">
                        <table className="history-table">
                            <thead>
                                <tr>
                                    <th className="col-status">{t("proxy.history.col.status")}</th>
                                    <th className="col-details">{t("proxy.history.col.details")}</th>
                                    <th className="col-time">{t("proxy.history.col.time")}</th>
                                    <th className="col-type">{t("proxy.history.col.type")}</th>
                                    <th className="col-channel">{t("proxy.history.col.channel")}</th>
                                    <th className="col-model">{t("proxy.history.col.model")}</th>
                                    <th className="col-tokens">{t("proxy.history.col.tokens")}</th>
                                    <th className="col-latency">{t("proxy.history.col.latency")}</th>
                                </tr>
                            </thead>
                            <tbody>
                                {records.map((record) => (
                                    <tr key={record.id} className={clsx("history-row", !record.success && "error-row")}>
                                        <td className="col-status">
                                            {record.success ? (
                                                <i
                                                    className="fa fa-check-circle status-success"
                                                    title={t("proxy.metrics.successful")}
                                                />
                                            ) : (
                                                <i
                                                    className="fa fa-times-circle status-error"
                                                    title={record.errorMsg || t("proxy.metrics.failed")}
                                                />
                                            )}
                                        </td>
                                        <td className="col-details">
                                            {!record.success ? (
                                                <button
                                                    className="details-btn"
                                                    onClick={() => handleOpenDetails(record)}
                                                    title={t("proxy.history.viewErrorDetails")}
                                                >
                                                    <i className="fa fa-info-circle" />
                                                </button>
                                            ) : (
                                                <span className="text-muted">-</span>
                                            )}
                                        </td>
                                        <td className="col-time">
                                            <span className="time-text">{formatTimestamp(record.timestamp)}</span>
                                        </td>
                                        <td className="col-type">
                                            <i className={clsx("fa", getChannelTypeIcon(record.channelType))} />
                                            <span className="type-text">{formatChannelType(record.channelType)}</span>
                                        </td>
                                        <td className="col-channel">
                                            <button
                                                className="channel-link"
                                                onClick={() => onFilterChange(record.channelId)}
                                                title={t("proxy.history.filterByChannel", { channelId: record.channelId })}
                                            >
                                                {record.channelId.slice(0, 8)}...
                                            </button>
                                        </td>
                                        <td className="col-model">
                                            <span className="model-text">{record.model || "-"}</span>
                                        </td>
                                        <td className="col-tokens">
                                            <span className="tokens-in" title={t("proxy.metrics.inputTokens")}>
                                                {formatTokens(record.inputTokens)}
                                            </span>
                                            <span className="tokens-sep">/</span>
                                            <span className="tokens-out" title={t("proxy.metrics.outputTokens")}>
                                                {formatTokens(record.outputTokens)}
                                            </span>
                                        </td>
                                        <td className="col-latency">
                                            <span className={clsx("latency-text", record.latencyMs > 5000 && "slow")}>
                                                {formatLatency(record.latencyMs)}
                                            </span>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}

                {detailsRecord && (
                    <div className="history-details-overlay" onClick={handleCloseDetails}>
                        <div className="history-details-modal" onClick={(e) => e.stopPropagation()}>
                            <div className="details-header">
                                <h3>{t("proxy.history.detailsTitle")}</h3>
                                <button className="close-btn" onClick={handleCloseDetails}>
                                    <i className="fa fa-times" />
                                </button>
                            </div>
                            <div className="details-body">
                                <div className="details-meta">
                                    <div className="meta-row">
                                        <span className="label">{t("proxy.history.detailsSummary")}:</span>
                                        <span className="value">{detailsRecord.errorMsg || "-"}</span>
                                    </div>
                                    <div className="meta-row">
                                        <span className="label">{t("proxy.history.detailsTime")}:</span>
                                        <span className="value">{formatTimestamp(detailsRecord.timestamp)}</span>
                                    </div>
                                    <div className="meta-row">
                                        <span className="label">{t("proxy.history.detailsChannel")}:</span>
                                        <span className="value">{detailsRecord.channelId}</span>
                                    </div>
                                </div>
                                <pre className="details-pre">{detailsRecord.errorDetails || detailsRecord.errorMsg || "-"}</pre>
                            </div>
                            <div className="details-footer">
                                <button className="proxy-btn btn-secondary" onClick={handleCloseDetails}>
                                    {t("common.close")}
                                </button>
                            </div>
                        </div>
                    </div>
                )}
            </div>
        );
    }
);

export { HistoryList };
