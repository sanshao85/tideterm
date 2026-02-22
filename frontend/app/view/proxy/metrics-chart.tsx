// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { useT } from "@/app/i18n/i18n";
import {
    LineChart,
    Line,
    AreaChart,
    Area,
    BarChart,
    Bar,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    Legend,
    ResponsiveContainer,
    PieChart,
    Pie,
    Cell,
} from "recharts";
import type { ChannelMetrics, GlobalStats } from "./proxy-model";

// Color palette for charts
const COLORS = ["#00C49F", "#FF8042", "#0088FE", "#FFBB28", "#8884D8", "#82CA9D"];

interface MetricsChartProps {
    metrics: { [key: string]: ChannelMetrics };
    globalStats: GlobalStats | null;
}

// Request distribution pie chart
interface RequestDistributionProps {
    metrics: { [key: string]: ChannelMetrics };
}

export function RequestDistributionChart({ metrics }: RequestDistributionProps) {
    const t = useT();
    const data = React.useMemo(() => {
        return Object.values(metrics)
            .filter((m) => m.requestCount > 0)
            .map((m) => ({
                name: m.channelId,
                value: m.requestCount,
            }));
    }, [metrics]);

    if (data.length === 0) {
        return (
            <div className="chart-empty">
                <span>{t("proxy.noData")}</span>
            </div>
        );
    }

    return (
        <ResponsiveContainer width="100%" height={200}>
            <PieChart>
                <Pie
                    data={data}
                    cx="50%"
                    cy="50%"
                    innerRadius={40}
                    outerRadius={80}
                    paddingAngle={2}
                    dataKey="value"
                    label={({ name, percent }) => `${name}: ${(percent * 100).toFixed(0)}%`}
                    labelLine={false}
                >
                    {data.map((_, index) => (
                        <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                    ))}
                </Pie>
                <Tooltip
                    contentStyle={{
                        backgroundColor: "var(--block-bg-color)",
                        border: "1px solid var(--border-color)",
                        borderRadius: "4px",
                    }}
                />
            </PieChart>
        </ResponsiveContainer>
    );
}

// Success rate bar chart
interface SuccessRateChartProps {
    metrics: { [key: string]: ChannelMetrics };
}

export function SuccessRateChart({ metrics }: SuccessRateChartProps) {
    const t = useT();
    const data = React.useMemo(() => {
        return Object.values(metrics).map((m) => ({
            name: m.channelId.slice(0, 10) + (m.channelId.length > 10 ? "..." : ""),
            fullName: m.channelId,
            successRate: Number((m.successRate * 100).toFixed(1)),
            failureRate: Number(((1 - m.successRate) * 100).toFixed(1)),
        }));
    }, [metrics]);

    if (data.length === 0) {
        return (
            <div className="chart-empty">
                <span>{t("proxy.noData")}</span>
            </div>
        );
    }

    return (
        <ResponsiveContainer width="100%" height={200}>
            <BarChart data={data} layout="vertical">
                <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" />
                <XAxis type="number" domain={[0, 100]} stroke="var(--secondary-text-color)" />
                <YAxis type="category" dataKey="name" width={80} stroke="var(--secondary-text-color)" />
                <Tooltip
                    contentStyle={{
                        backgroundColor: "var(--block-bg-color)",
                        border: "1px solid var(--border-color)",
                        borderRadius: "4px",
                    }}
                    formatter={(value: number, name: string) => [
                        `${value}%`,
                        name === "successRate" ? t("proxy.metrics.successful") : t("proxy.metrics.failed"),
                    ]}
                />
                <Bar dataKey="successRate" stackId="a" fill="#00C49F" name={t("proxy.metrics.successful")} />
                <Bar dataKey="failureRate" stackId="a" fill="#FF8042" name={t("proxy.metrics.failed")} />
            </BarChart>
        </ResponsiveContainer>
    );
}

// Latency comparison chart
interface LatencyChartProps {
    metrics: { [key: string]: ChannelMetrics };
}

export function LatencyChart({ metrics }: LatencyChartProps) {
    const t = useT();
    const data = React.useMemo(() => {
        return Object.values(metrics)
            .filter((m) => m.avgLatencyMs > 0)
            .map((m) => ({
                name: m.channelId.slice(0, 10) + (m.channelId.length > 10 ? "..." : ""),
                fullName: m.channelId,
                latency: Number(m.avgLatencyMs.toFixed(0)),
            }));
    }, [metrics]);

    if (data.length === 0) {
        return (
            <div className="chart-empty">
                <span>{t("proxy.noData")}</span>
            </div>
        );
    }

    return (
        <ResponsiveContainer width="100%" height={200}>
            <BarChart data={data}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" />
                <XAxis dataKey="name" stroke="var(--secondary-text-color)" />
                <YAxis stroke="var(--secondary-text-color)" />
                <Tooltip
                    contentStyle={{
                        backgroundColor: "var(--block-bg-color)",
                        border: "1px solid var(--border-color)",
                        borderRadius: "4px",
                    }}
                    formatter={(value: number) => [`${value}ms`, t("proxy.metrics.avgLatency")]}
                />
                <Bar dataKey="latency" fill="#0088FE" name={t("proxy.metrics.avgLatency")} />
            </BarChart>
        </ResponsiveContainer>
    );
}

// Token usage chart
interface TokenUsageChartProps {
    metrics: { [key: string]: ChannelMetrics };
}

export function TokenUsageChart({ metrics }: TokenUsageChartProps) {
    const t = useT();
    const data = React.useMemo(() => {
        return Object.values(metrics)
            .filter((m) => m.inputTokens > 0 || m.outputTokens > 0)
            .map((m) => ({
                name: m.channelId.slice(0, 10) + (m.channelId.length > 10 ? "..." : ""),
                fullName: m.channelId,
                input: m.inputTokens,
                output: m.outputTokens,
            }));
    }, [metrics]);

    if (data.length === 0) {
        return (
            <div className="chart-empty">
                <span>{t("proxy.noData")}</span>
            </div>
        );
    }

    return (
        <ResponsiveContainer width="100%" height={200}>
            <BarChart data={data}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" />
                <XAxis dataKey="name" stroke="var(--secondary-text-color)" />
                <YAxis stroke="var(--secondary-text-color)" />
                <Tooltip
                    contentStyle={{
                        backgroundColor: "var(--block-bg-color)",
                        border: "1px solid var(--border-color)",
                        borderRadius: "4px",
                    }}
                    formatter={(value: number) => [value.toLocaleString(), ""]}
                />
                <Legend />
                <Bar dataKey="input" fill="#8884D8" name={t("proxy.metrics.inputTokens")} />
                <Bar dataKey="output" fill="#82CA9D" name={t("proxy.metrics.outputTokens")} />
            </BarChart>
        </ResponsiveContainer>
    );
}

// Global stats summary cards
interface GlobalStatsCardsProps {
    stats: GlobalStats | null;
}

export function GlobalStatsCards({ stats }: GlobalStatsCardsProps) {
    const t = useT();
    if (!stats) {
        return (
            <div className="stats-cards">
                <div className="stats-card">
                    <div className="stats-value">-</div>
                    <div className="stats-label">{t("proxy.metrics.totalRequests")}</div>
                </div>
                <div className="stats-card">
                    <div className="stats-value">-</div>
                    <div className="stats-label">{t("proxy.metrics.successRate")}</div>
                </div>
                <div className="stats-card">
                    <div className="stats-value">-</div>
                    <div className="stats-label">{t("proxy.channels")}</div>
                </div>
            </div>
        );
    }

    return (
        <div className="stats-cards">
            <div className="stats-card">
                <div className="stats-value">{stats.totalRequests.toLocaleString()}</div>
                <div className="stats-label">{t("proxy.metrics.totalRequests")}</div>
            </div>
            <div className="stats-card success">
                <div className="stats-value">{(stats.successRate * 100).toFixed(1)}%</div>
                <div className="stats-label">{t("proxy.metrics.successRate")}</div>
            </div>
            <div className="stats-card">
                <div className="stats-value">{stats.successCount.toLocaleString()}</div>
                <div className="stats-label">{t("proxy.metrics.successful")}</div>
            </div>
            <div className="stats-card error">
                <div className="stats-value">{stats.failureCount.toLocaleString()}</div>
                <div className="stats-label">{t("proxy.metrics.failed")}</div>
            </div>
            <div className="stats-card">
                <div className="stats-value">{stats.channelCount}</div>
                <div className="stats-label">{t("proxy.channels")}</div>
            </div>
        </div>
    );
}

// Circuit breaker status display
interface CircuitBreakerStatusProps {
    metrics: { [key: string]: ChannelMetrics };
    onReset?: (channelId: string) => void;
}

export function CircuitBreakerStatus({ metrics, onReset }: CircuitBreakerStatusProps) {
    const t = useT();
    const brokenCircuits = React.useMemo(() => {
        return Object.values(metrics).filter((m) => m.circuitBroken || m.consecutiveFailures >= 3);
    }, [metrics]);

    if (brokenCircuits.length === 0) {
        return (
            <div className="circuit-status healthy">
                <i className="fa-solid fa-check-circle"></i>
                <span>{t("proxy.circuitBreaker.healthy")}</span>
            </div>
        );
    }

    return (
        <div className="circuit-status-list">
            {brokenCircuits.map((m) => (
                <div key={m.channelId} className={`circuit-item ${m.circuitBroken ? "broken" : "warning"}`}>
                        <div className="circuit-info">
                            <i className={`fa-solid ${m.circuitBroken ? "fa-times-circle" : "fa-exclamation-triangle"}`}></i>
                            <span className="circuit-name">{m.channelId}</span>
                            <span className="circuit-failures">
                                {t("proxy.circuitBreaker.failures", { count: m.consecutiveFailures })}
                            </span>
                        </div>
                    {onReset && (
                        <button
                            className="circuit-reset-btn"
                            onClick={() => onReset(m.channelId)}
                            title={t("proxy.circuitBreaker.reset")}
                        >
                            <i className="fa-solid fa-redo"></i>
                        </button>
                    )}
                </div>
            ))}
        </div>
    );
}

// Main metrics dashboard component
export function MetricsDashboard({ metrics, globalStats }: MetricsChartProps) {
    const t = useT();
    return (
        <div className="metrics-dashboard">
            <GlobalStatsCards stats={globalStats} />

            <div className="charts-grid">
                <div className="chart-container">
                    <h4>{t("proxy.metrics.requestDistribution")}</h4>
                    <RequestDistributionChart metrics={metrics} />
                </div>

                <div className="chart-container">
                    <h4>{t("proxy.metrics.successRateByChannel")}</h4>
                    <SuccessRateChart metrics={metrics} />
                </div>

                <div className="chart-container">
                    <h4>{t("proxy.metrics.avgLatency")}</h4>
                    <LatencyChart metrics={metrics} />
                </div>

                <div className="chart-container">
                    <h4>{t("proxy.metrics.tokenUsage")}</h4>
                    <TokenUsageChart metrics={metrics} />
                </div>
            </div>
        </div>
    );
}
