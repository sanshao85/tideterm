// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useT } from "@/app/i18n/i18n";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { cn } from "@/util/util";
import { memo, useCallback, useEffect, useMemo, useState } from "react";

type McpServerData = {
    id: string;
    name: string;
    server: McpServerSpecData;
    apps: McpAppsData;
    description?: string;
    homepage?: string;
    docs?: string;
    tags?: string[];
};

// LoadingSpinner component
const LoadingSpinner = memo(({ message }: { message: string }) => {
    return (
        <div className="flex flex-col items-center justify-center gap-3 py-12">
            <i className="fa-sharp fa-solid fa-spinner fa-spin text-2xl text-zinc-400" />
            <span className="text-zinc-400">{message}</span>
        </div>
    );
});
LoadingSpinner.displayName = "LoadingSpinner";

// ErrorDisplay component
const ErrorDisplay = memo(({ message, onClose }: { message: string; onClose?: () => void }) => {
    return (
        <div className="flex items-center justify-between gap-2 p-4 bg-red-500/10 border border-red-500/20 text-red-400 rounded-lg m-4">
            <div className="flex items-center gap-2">
                <i className="fa-sharp fa-solid fa-circle-exclamation" />
                <span>{message}</span>
            </div>
            {onClose && (
                <button onClick={onClose} className="hover:bg-red-500/20 rounded p-1 cursor-pointer transition-colors">
                    <i className="fa-sharp fa-solid fa-times" />
                </button>
            )}
        </div>
    );
});
ErrorDisplay.displayName = "ErrorDisplay";

// WarningBanner component for usage instructions
const WarningBanner = memo(() => {
    const t = useT();
    return (
        <div className="flex items-start gap-3 p-4 bg-amber-500/10 border border-amber-500/20 text-amber-400 rounded-lg mx-4 mt-4">
            <i className="fa-sharp fa-solid fa-triangle-exclamation text-lg flex-shrink-0 mt-0.5" />
            <span className="text-sm">{t("mcp.warning")}</span>
        </div>
    );
});
WarningBanner.displayName = "WarningBanner";

// EmptyState component
const EmptyState = memo(({ onAddServer, onImport }: { onAddServer: () => void; onImport: () => void }) => {
    const t = useT();
    return (
        <div className="flex flex-col items-center justify-center gap-4 py-12 h-full bg-zinc-800/50 rounded-lg m-4">
            <i className="fa-sharp fa-solid fa-server text-4xl text-zinc-600" />
            <h3 className="text-lg font-semibold text-zinc-400">{t("mcp.noServers")}</h3>
            <p className="text-zinc-500 text-center max-w-md">{t("mcp.addToGetStarted")}</p>
            <div className="flex gap-2">
                <button
                    className="flex items-center gap-2 px-4 py-2 bg-accent-600 hover:bg-accent-500 rounded cursor-pointer transition-colors"
                    onClick={onAddServer}
                >
                    <i className="fa-sharp fa-solid fa-plus" />
                    <span className="font-medium">{t("mcp.addServer")}</span>
                </button>
                <button
                    className="flex items-center gap-2 px-4 py-2 bg-zinc-700 hover:bg-zinc-600 rounded cursor-pointer transition-colors"
                    onClick={onImport}
                >
                    <i className="fa-sharp fa-solid fa-file-import" />
                    <span className="font-medium">{t("mcp.importFromApps")}</span>
                </button>
            </div>
        </div>
    );
});
EmptyState.displayName = "EmptyState";

// AppToggle component for enabling/disabling apps
const AppToggle = memo(
    ({
        app,
        enabled,
        installed,
        onToggle,
        disabled,
    }: {
        app: string;
        enabled: boolean;
        installed: boolean;
        onToggle: (enabled: boolean) => void;
        disabled?: boolean;
    }) => {
        const appLabels: Record<string, string> = {
            claude: "Claude",
            codex: "Codex",
            gemini: "Gemini",
        };
        const appIcons: Record<string, string> = {
            claude: "fa-robot",
            codex: "fa-terminal",
            gemini: "fa-star",
        };

        return (
            <label
                className={cn(
                    "flex items-center gap-2 px-3 py-1.5 rounded transition-colors cursor-pointer",
                    !installed && "opacity-50 cursor-not-allowed",
                    enabled ? "bg-accent-600/30 text-accent-400" : "bg-zinc-700/50 text-zinc-400"
                )}
            >
                <input
                    type="checkbox"
                    checked={enabled}
                    onChange={(e) => onToggle(e.target.checked)}
                    disabled={disabled || !installed}
                    className="sr-only"
                />
                <i className={`fa-sharp fa-solid ${appIcons[app] || "fa-circle"} text-sm`} />
                <span className="text-sm">{appLabels[app] || app}</span>
                {enabled && <i className="fa-sharp fa-solid fa-check text-xs" />}
            </label>
        );
    }
);
AppToggle.displayName = "AppToggle";

// ServerListItem component
const ServerListItem = memo(
    ({
        server,
        appStatus,
        onEdit,
        onDelete,
        onToggleApp,
    }: {
        server: McpServerData;
        appStatus: McpAppStatusData;
        onEdit: () => void;
        onDelete: () => void;
        onToggleApp: (app: string, enabled: boolean) => void;
    }) => {
        const t = useT();
        const [isTogglingApp, setIsTogglingApp] = useState<string | null>(null);

        const handleToggleApp = async (app: string, enabled: boolean) => {
            setIsTogglingApp(app);
            try {
                await onToggleApp(app, enabled);
            } finally {
                setIsTogglingApp(null);
            }
        };

        const transportType = server.server.type || "stdio";
        const transportIcon = transportType === "stdio" ? "fa-terminal" : "fa-globe";

        return (
            <div className="flex flex-col gap-3 p-4 border-b border-zinc-700 hover:bg-zinc-800/30 transition-colors">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <i className={`fa-sharp fa-solid ${transportIcon} text-accent-500`} />
                        <div>
                            <div className="font-medium">{server.name || server.id}</div>
                            {server.description && (
                                <div className="text-xs text-zinc-500 mt-0.5">{server.description}</div>
                            )}
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        <button
                            onClick={onEdit}
                            className="p-2 hover:bg-zinc-700 rounded cursor-pointer transition-colors"
                            title={t("common.edit")}
                        >
                            <i className="fa-sharp fa-solid fa-pen text-zinc-400" />
                        </button>
                        <button
                            onClick={onDelete}
                            className="p-2 hover:bg-red-500/20 rounded cursor-pointer transition-colors"
                            title={t("common.delete")}
                        >
                            <i className="fa-sharp fa-solid fa-trash text-red-400" />
                        </button>
                    </div>
                </div>
                <div className="flex flex-wrap gap-2">
                    <AppToggle
                        app="claude"
                        enabled={server.apps.claude}
                        installed={appStatus.claude}
                        onToggle={(enabled) => handleToggleApp("claude", enabled)}
                        disabled={isTogglingApp === "claude"}
                    />
                    <AppToggle
                        app="codex"
                        enabled={server.apps.codex}
                        installed={appStatus.codex}
                        onToggle={(enabled) => handleToggleApp("codex", enabled)}
                        disabled={isTogglingApp === "codex"}
                    />
                    <AppToggle
                        app="gemini"
                        enabled={server.apps.gemini}
                        installed={appStatus.gemini}
                        onToggle={(enabled) => handleToggleApp("gemini", enabled)}
                        disabled={isTogglingApp === "gemini"}
                    />
                </div>
                <div className="text-xs text-zinc-500 font-mono">
                    {transportType === "stdio" ? (
                        <span>
                            {server.server.command} {server.server.args?.join(" ")}
                        </span>
                    ) : (
                        <span>{server.server.url}</span>
                    )}
                </div>
            </div>
        );
    }
);
ServerListItem.displayName = "ServerListItem";

// ServerForm component for adding/editing servers
const ServerForm = memo(
    ({
        server,
        appStatus,
        isLoading,
        onSave,
        onCancel,
    }: {
        server: McpServerData | null;
        appStatus: McpAppStatusData;
        isLoading: boolean;
        onSave: (server: McpServerData) => void;
        onCancel: () => void;
    }) => {
        const t = useT();
        const isEditing = !!server;

        const [formData, setFormData] = useState<McpServerData>(() => {
            if (server) {
                return { ...server };
            }
            return {
                id: "",
                name: "",
                server: {
                    type: "stdio",
                    command: "",
                    args: [],
                },
                apps: {
                    claude: true,
                    codex: false,
                    gemini: false,
                },
            };
        });

        const [argsText, setArgsText] = useState(() => {
            return formData.server.args?.join(" ") || "";
        });

        const [envText, setEnvText] = useState(() => {
            if (!formData.server.env) return "";
            return Object.entries(formData.server.env)
                .map(([k, v]) => `${k}=${v}`)
                .join("\n");
        });

        const updateFormData = (updates: Partial<McpServerData>) => {
            setFormData((prev) => ({ ...prev, ...updates }));
        };

        const updateServerSpec = (updates: Partial<McpServerSpecData>) => {
            setFormData((prev) => ({
                ...prev,
                server: { ...prev.server, ...updates },
            }));
        };

        const updateApps = (app: string, enabled: boolean) => {
            setFormData((prev) => ({
                ...prev,
                apps: { ...prev.apps, [app]: enabled },
            }));
        };

        const handleSubmit = () => {
            const args = argsText
                .split(/\s+/)
                .map((a) => a.trim())
                .filter((a) => a);
            const env: Record<string, string> = {};
            envText.split("\n").forEach((line) => {
                const idx = line.indexOf("=");
                if (idx > 0) {
                    const key = line.slice(0, idx).trim();
                    const value = line.slice(idx + 1).trim();
                    if (key) env[key] = value;
                }
            });

            const finalData: McpServerData = {
                ...formData,
                server: {
                    ...formData.server,
                    args: args.length > 0 ? args : undefined,
                    env: Object.keys(env).length > 0 ? env : undefined,
                },
            };

            onSave(finalData);
        };

        const isValid = formData.id.trim() !== "" &&
            ((formData.server.type === "stdio" && formData.server.command?.trim()) ||
             ((formData.server.type === "http" || formData.server.type === "sse") && formData.server.url?.trim()));

        return (
            <div className="flex flex-col gap-4 p-6 bg-zinc-800/50 rounded-lg m-4">
                <h3 className="text-lg font-semibold">{isEditing ? t("mcp.editServer") : t("mcp.addServer")}</h3>

                {/* ID */}
                <div className="flex flex-col gap-2">
                    <label className="text-sm font-medium">{t("mcp.serverId")}</label>
                    <input
                        type="text"
                        className={cn(
                            "px-3 py-2 bg-zinc-800 border rounded focus:outline-none",
                            "border-zinc-600 focus:border-accent-500",
                            isEditing && "opacity-50 cursor-not-allowed"
                        )}
                        value={formData.id}
                        onChange={(e) => updateFormData({ id: e.target.value })}
                        placeholder="my-mcp-server"
                        disabled={isLoading || isEditing}
                    />
                    <div className="text-xs text-zinc-400">{t("mcp.serverId.help")}</div>
                </div>

                {/* Name */}
                <div className="flex flex-col gap-2">
                    <label className="text-sm font-medium">{t("mcp.serverName")}</label>
                    <input
                        type="text"
                        className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500"
                        value={formData.name}
                        onChange={(e) => updateFormData({ name: e.target.value })}
                        placeholder="My MCP Server"
                        disabled={isLoading}
                    />
                </div>

                {/* Transport Type */}
                <div className="flex flex-col gap-2">
                    <label className="text-sm font-medium">{t("mcp.transportType")}</label>
                    <select
                        className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500"
                        value={formData.server.type || "stdio"}
                        onChange={(e) => updateServerSpec({ type: e.target.value as any })}
                        disabled={isLoading}
                    >
                        <option value="stdio">stdio</option>
                        <option value="http">http</option>
                        <option value="sse">sse</option>
                    </select>
                </div>

                {/* Stdio fields */}
                {(formData.server.type === "stdio" || !formData.server.type) && (
                    <>
                        <div className="flex flex-col gap-2">
                            <label className="text-sm font-medium">{t("mcp.command")}</label>
                            <input
                                type="text"
                                className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500 font-mono"
                                value={formData.server.command || ""}
                                onChange={(e) => updateServerSpec({ command: e.target.value })}
                                placeholder="npx"
                                disabled={isLoading}
                            />
                        </div>
                        <div className="flex flex-col gap-2">
                            <label className="text-sm font-medium">{t("mcp.args")}</label>
                            <input
                                type="text"
                                className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500 font-mono"
                                value={argsText}
                                onChange={(e) => setArgsText(e.target.value)}
                                placeholder="-y @modelcontextprotocol/server-filesystem /path"
                                disabled={isLoading}
                            />
                            <div className="text-xs text-zinc-400">{t("mcp.args.help")}</div>
                        </div>
                        <div className="flex flex-col gap-2">
                            <label className="text-sm font-medium">{t("mcp.env")}</label>
                            <textarea
                                className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500 font-mono text-sm"
                                value={envText}
                                onChange={(e) => setEnvText(e.target.value)}
                                placeholder="KEY=value&#10;ANOTHER_KEY=another_value"
                                disabled={isLoading}
                                rows={3}
                            />
                            <div className="text-xs text-zinc-400">{t("mcp.env.help")}</div>
                        </div>
                    </>
                )}

                {/* HTTP/SSE fields */}
                {(formData.server.type === "http" || formData.server.type === "sse") && (
                    <div className="flex flex-col gap-2">
                        <label className="text-sm font-medium">{t("mcp.url")}</label>
                        <input
                            type="text"
                            className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500 font-mono"
                            value={formData.server.url || ""}
                            onChange={(e) => updateServerSpec({ url: e.target.value })}
                            placeholder="https://example.com/mcp"
                            disabled={isLoading}
                        />
                    </div>
                )}

                {/* Description */}
                <div className="flex flex-col gap-2">
                    <label className="text-sm font-medium">{t("mcp.description")}</label>
                    <input
                        type="text"
                        className="px-3 py-2 bg-zinc-800 border border-zinc-600 rounded focus:outline-none focus:border-accent-500"
                        value={formData.description || ""}
                        onChange={(e) => updateFormData({ description: e.target.value })}
                        placeholder={t("mcp.description.placeholder")}
                        disabled={isLoading}
                    />
                </div>

                {/* Apps */}
                <div className="flex flex-col gap-2">
                    <label className="text-sm font-medium">{t("mcp.enabledApps")}</label>
                    <div className="flex flex-wrap gap-2">
                        <AppToggle
                            app="claude"
                            enabled={formData.apps.claude}
                            installed={appStatus.claude}
                            onToggle={(enabled) => updateApps("claude", enabled)}
                            disabled={isLoading}
                        />
                        <AppToggle
                            app="codex"
                            enabled={formData.apps.codex}
                            installed={appStatus.codex}
                            onToggle={(enabled) => updateApps("codex", enabled)}
                            disabled={isLoading}
                        />
                        <AppToggle
                            app="gemini"
                            enabled={formData.apps.gemini}
                            installed={appStatus.gemini}
                            onToggle={(enabled) => updateApps("gemini", enabled)}
                            disabled={isLoading}
                        />
                    </div>
                    <div className="text-xs text-zinc-400">{t("mcp.enabledApps.help")}</div>
                </div>

                {/* Actions */}
                <div className="flex gap-2 justify-end mt-2">
                    <button
                        className="px-4 py-2 bg-zinc-700 hover:bg-zinc-600 rounded cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                        onClick={onCancel}
                        disabled={isLoading}
                    >
                        {t("common.cancel")}
                    </button>
                    <button
                        className="px-4 py-2 bg-accent-600 hover:bg-accent-500 rounded cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                        onClick={handleSubmit}
                        disabled={isLoading || !isValid}
                    >
                        {isLoading ? (
                            <>
                                <i className="fa-sharp fa-solid fa-spinner fa-spin" />
                                {t("common.saving")}
                            </>
                        ) : (
                            t("common.save")
                        )}
                    </button>
                </div>
            </div>
        );
    }
);
ServerForm.displayName = "ServerForm";

// Main McpContent component
export const McpContent = memo(() => {
    const t = useT();
    const [servers, setServers] = useState<Record<string, McpServerData>>({});
    const [appStatus, setAppStatus] = useState<McpAppStatusData>({ claude: false, codex: false, gemini: false });
    const [isLoading, setIsLoading] = useState(true);
    const [isSaving, setIsSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [editingServer, setEditingServer] = useState<McpServerData | null>(null);
    const [isAddingNew, setIsAddingNew] = useState(false);

    const loadData = useCallback(async () => {
        setIsLoading(true);
        setError(null);
        try {
            const [serversData, statusData] = await Promise.all([
                RpcApi.McpGetServersCommand(TabRpcClient),
                RpcApi.McpGetAppStatusCommand(TabRpcClient),
            ]);
            setServers(serversData || {});
            setAppStatus(statusData);
        } catch (err: any) {
            setError(`Failed to load MCP configuration: ${err.message || String(err)}`);
        } finally {
            setIsLoading(false);
        }
    }, []);

    useEffect(() => {
        loadData();
    }, [loadData]);

    const sortedServers = useMemo(() => {
        return Object.values(servers).sort((a, b) => a.id.localeCompare(b.id));
    }, [servers]);

    const handleSaveServer = async (serverData: McpServerData) => {
        setIsSaving(true);
        setError(null);
        try {
            await RpcApi.McpUpsertServerCommand(TabRpcClient, serverData);
            await loadData();
            setEditingServer(null);
            setIsAddingNew(false);
        } catch (err: any) {
            setError(`Failed to save server: ${err.message || String(err)}`);
        } finally {
            setIsSaving(false);
        }
    };

    const handleDeleteServer = async (serverId: string) => {
        if (!confirm(t("mcp.deleteConfirm"))) {
            return;
        }
        setIsSaving(true);
        setError(null);
        try {
            await RpcApi.McpDeleteServerCommand(TabRpcClient, serverId);
            await loadData();
        } catch (err: any) {
            setError(`Failed to delete server: ${err.message || String(err)}`);
        } finally {
            setIsSaving(false);
        }
    };

    const handleToggleApp = async (serverId: string, app: string, enabled: boolean) => {
        setError(null);
        try {
            await RpcApi.McpToggleAppCommand(TabRpcClient, { serverid: serverId, app, enabled });
            await loadData();
        } catch (err: any) {
            setError(`Failed to toggle app: ${err.message || String(err)}`);
        }
    };

    const handleImport = async () => {
        setIsSaving(true);
        setError(null);
        try {
            const result = await RpcApi.McpImportFromAppCommand(TabRpcClient, { app: "" });
            await loadData();
            if (result.imported > 0) {
                alert(t("mcp.importSuccess", { count: result.imported }));
            } else {
                alert(t("mcp.importNoServers"));
            }
        } catch (err: any) {
            setError(`Failed to import: ${err.message || String(err)}`);
        } finally {
            setIsSaving(false);
        }
    };

    const handleSyncAll = async () => {
        setIsSaving(true);
        setError(null);
        try {
            await RpcApi.McpSyncAllCommand(TabRpcClient);
            alert(t("mcp.syncSuccess"));
        } catch (err: any) {
            setError(`Failed to sync: ${err.message || String(err)}`);
        } finally {
            setIsSaving(false);
        }
    };

    if (isLoading) {
        return (
            <div className="w-full h-full">
                <LoadingSpinner message={t("mcp.loading")} />
            </div>
        );
    }

    if (isAddingNew || editingServer) {
        return (
            <div className="w-full h-full overflow-auto">
                {error && <ErrorDisplay message={error} onClose={() => setError(null)} />}
                <ServerForm
                    server={editingServer}
                    appStatus={appStatus}
                    isLoading={isSaving}
                    onSave={handleSaveServer}
                    onCancel={() => {
                        setEditingServer(null);
                        setIsAddingNew(false);
                    }}
                />
            </div>
        );
    }

    if (sortedServers.length === 0) {
        return (
            <div className="w-full h-full">
                <WarningBanner />
                {error && <ErrorDisplay message={error} onClose={() => setError(null)} />}
                <EmptyState onAddServer={() => setIsAddingNew(true)} onImport={handleImport} />
            </div>
        );
    }

    return (
        <div className="w-full h-full overflow-auto">
            <WarningBanner />
            {error && <ErrorDisplay message={error} onClose={() => setError(null)} />}

            {/* Header with actions */}
            <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-700">
                <div className="text-sm text-zinc-400">
                    {t("mcp.serverCount", { count: sortedServers.length })}
                </div>
                <div className="flex gap-2">
                    <button
                        className="flex items-center gap-2 px-3 py-1.5 text-sm bg-zinc-700 hover:bg-zinc-600 rounded cursor-pointer transition-colors disabled:opacity-50"
                        onClick={handleSyncAll}
                        disabled={isSaving}
                        title={t("mcp.syncAll.tooltip")}
                    >
                        <i className="fa-sharp fa-solid fa-sync" />
                        {t("mcp.syncAll")}
                    </button>
                    <button
                        className="flex items-center gap-2 px-3 py-1.5 text-sm bg-zinc-700 hover:bg-zinc-600 rounded cursor-pointer transition-colors disabled:opacity-50"
                        onClick={handleImport}
                        disabled={isSaving}
                    >
                        <i className="fa-sharp fa-solid fa-file-import" />
                        {t("mcp.import")}
                    </button>
                    <button
                        className="flex items-center gap-2 px-3 py-1.5 text-sm bg-accent-600 hover:bg-accent-500 rounded cursor-pointer transition-colors"
                        onClick={() => setIsAddingNew(true)}
                    >
                        <i className="fa-sharp fa-solid fa-plus" />
                        {t("mcp.addServer")}
                    </button>
                </div>
            </div>

            {/* Server list */}
            <div className="divide-y divide-zinc-700">
                {sortedServers.map((server) => (
                    <ServerListItem
                        key={server.id}
                        server={server}
                        appStatus={appStatus}
                        onEdit={() => setEditingServer(server)}
                        onDelete={() => handleDeleteServer(server.id)}
                        onToggleApp={(app, enabled) => handleToggleApp(server.id, app, enabled)}
                    />
                ))}
            </div>

            {/* App Status Footer */}
            <div className="px-4 py-3 border-t border-zinc-700 bg-zinc-800/30">
                <div className="text-xs text-zinc-500 mb-2">{t("mcp.installedApps")}</div>
                <div className="flex gap-4">
                    <div className={cn("flex items-center gap-2", appStatus.claude ? "text-green-400" : "text-zinc-500")}>
                        <i className={`fa-sharp fa-solid ${appStatus.claude ? "fa-check-circle" : "fa-times-circle"}`} />
                        <span>Claude Code</span>
                    </div>
                    <div className={cn("flex items-center gap-2", appStatus.codex ? "text-green-400" : "text-zinc-500")}>
                        <i className={`fa-sharp fa-solid ${appStatus.codex ? "fa-check-circle" : "fa-times-circle"}`} />
                        <span>Codex CLI</span>
                    </div>
                    <div className={cn("flex items-center gap-2", appStatus.gemini ? "text-green-400" : "text-zinc-500")}>
                        <i className={`fa-sharp fa-solid ${appStatus.gemini ? "fa-check-circle" : "fa-times-circle"}`} />
                        <span>Gemini CLI</span>
                    </div>
                </div>
            </div>
        </div>
    );
});

McpContent.displayName = "McpContent";
