// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { Block } from "@/app/block/block";
import { Button } from "@/app/element/button";
import { useT } from "@/app/i18n/i18n";
import { Modal } from "@/app/modals/modal";
import { modalsModel } from "@/app/store/modalmodel";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { WOS } from "@/store/global";
import { stringToBase64 } from "@/util/util";
import clsx from "clsx";
import * as React from "react";

import "./tmuxsessions.scss";

type TmuxSessionsModalProps = {
    blockId: string;
};

const TMUX_MANAGED_PREFIX = "tideterm-";
const MAX_ALIAS_LENGTH = 64;

function tmuxSessionNameForBlockId(blockId: string): string {
    const trimmed = blockId.trim();
    if (!trimmed) {
        return "";
    }
    let cleaned = "";
    for (let i = 0; i < trimmed.length; i++) {
        const ch = trimmed[i];
        if (ch >= "a" && ch <= "z") {
            cleaned += ch;
            continue;
        }
        if (ch >= "A" && ch <= "Z") {
            cleaned += ch;
            continue;
        }
        if (ch >= "0" && ch <= "9") {
            cleaned += ch;
            continue;
        }
        if (ch === "_") {
            cleaned += ch;
        }
    }
    if (!cleaned) {
        return "";
    }
    return `${TMUX_MANAGED_PREFIX}${cleaned}`;
}

function isLocalConnectionName(connection: string | null | undefined): boolean {
    if (connection == null || connection === "") {
        return true;
    }
    if (connection === "local") {
        return true;
    }
    return connection.startsWith("local:");
}

function shellQuoteSingle(value: string): string {
    if (value === "") {
        return "''";
    }
    return "'" + value.replace(/'/g, "'\\''") + "'";
}

function buildAttachCommand(sessionName: string, force: boolean): string {
    const quoted = shellQuoteSingle(sessionName);
    if (force) {
        return [
            `if [ -n "$TMUX" ]; then`,
            `  tmux switch-client -t ${quoted} \\; refresh-client -S \\; detach-client -a -s ${quoted};`,
            `else`,
            `  tmux attach -d -t ${quoted};`,
            `fi`,
        ].join(" ");
    }
    return [
        `if [ -n "$TMUX" ]; then`,
        `  tmux switch-client -t ${quoted} \\; refresh-client -S;`,
        `else`,
        `  tmux attach -t ${quoted};`,
        `fi`,
    ].join(" ");
}

function formatRecentActivity(t: (key: string, vars?: Record<string, string | number>) => string, activity: number): string {
    if (!activity || activity <= 0) {
        return t("term.tmuxSessions.activity.unknown");
    }
    const nowSec = Math.floor(Date.now() / 1000);
    const diff = Math.max(0, nowSec - activity);
    if (diff < 60) {
        return t("term.tmuxSessions.activity.justNow");
    }
    if (diff < 3600) {
        return t("term.tmuxSessions.activity.minutesAgo", { count: Math.floor(diff / 60) });
    }
    if (diff < 86400) {
        return t("term.tmuxSessions.activity.hoursAgo", { count: Math.floor(diff / 3600) });
    }
    return t("term.tmuxSessions.activity.daysAgo", { count: Math.floor(diff / 86400) });
}

function trimAliasInput(value: string): string {
    let alias = value.trim();
    alias = alias.replace(/[\r\n\t]/g, " ");
    if (alias.length > MAX_ALIAS_LENGTH) {
        alias = alias.slice(0, MAX_ALIAS_LENGTH).trim();
    }
    return alias;
}

const TmuxSessionsModal = ({ blockId }: TmuxSessionsModalProps) => {
    const t = useT();
    const [blockData] = WOS.useWaveObjectValue<Block>(WOS.makeORef("block", blockId));
    const connName = (blockData?.meta?.connection as string) ?? "local";

    const [sessions, setSessions] = React.useState<TmuxSessionInfo[]>([]);
    const [loading, setLoading] = React.useState(false);
    const [error, setError] = React.useState<string | null>(null);
    const [actionSession, setActionSession] = React.useState<string | null>(null);
    const [bulkAction, setBulkAction] = React.useState(false);
    const [renameSessionName, setRenameSessionName] = React.useState<string | null>(null);
    const [renameValue, setRenameValue] = React.useState("");
    const currentSessionName = React.useMemo(() => tmuxSessionNameForBlockId(blockId), [blockId]);

    const refreshSessions = React.useCallback(async () => {
        if (isLocalConnectionName(connName)) {
            setError(t("term.tmuxSessions.errors.localUnsupported"));
            setSessions([]);
            return;
        }
        setLoading(true);
        setError(null);
        try {
            const resp = await RpcApi.TmuxListSessionsCommand(TabRpcClient, {
                connname: connName,
                logblockid: blockId,
            });
            setSessions(resp?.sessions ?? []);
        } catch (err) {
            const message = err instanceof Error ? err.message : String(err);
            setError(message);
            setSessions([]);
        } finally {
            setLoading(false);
        }
    }, [blockId, connName, t]);

    React.useEffect(() => {
        void refreshSessions();
    }, [refreshSessions]);

    const sortedSessions = React.useMemo(() => {
        return [...sessions].sort((a, b) => {
            const aAct = a?.activity ?? 0;
            const bAct = b?.activity ?? 0;
            return bAct - aAct;
        });
    }, [sessions]);

    const sendControllerCommand = React.useCallback(
        (cmd: string) => {
            const payload = stringToBase64(cmd + "\n");
            RpcApi.ControllerInputCommand(TabRpcClient, { blockid: blockId, inputdata64: payload }).catch(() => {});
        },
        [blockId]
    );

    const handleAttach = React.useCallback(
        (session: TmuxSessionInfo, force: boolean) => {
            const isManaged = session.name?.startsWith(TMUX_MANAGED_PREFIX);
            const isAttached = (session.attached ?? 0) > 0;
            if (force && (!isManaged || isAttached)) {
                const ok = window.confirm(t("term.tmuxSessions.confirm.forceAttach"));
                if (!ok) return;
            }
            if (!force && !isManaged && isAttached) {
                const ok = window.confirm(t("term.tmuxSessions.confirm.attach"));
                if (!ok) return;
            }
            const cmd = buildAttachCommand(session.name, force);
            sendControllerCommand(cmd);
            modalsModel.popModal();
        },
        [sendControllerCommand, t]
    );

    const handleKill = React.useCallback(
        async (session: TmuxSessionInfo) => {
            const isManaged = session.name?.startsWith(TMUX_MANAGED_PREFIX);
            const isAttached = (session.attached ?? 0) > 0;
            if (!window.confirm(isManaged && !isAttached ? t("term.tmuxSessions.confirm.kill") : t("term.tmuxSessions.confirm.killRisk"))) {
                return;
            }
            setActionSession(session.name);
            try {
                await RpcApi.TmuxKillSessionCommand(TabRpcClient, {
                    connname: connName,
                    sessionname: session.name,
                    logblockid: blockId,
                });
                await refreshSessions();
            } catch (err) {
                const message = err instanceof Error ? err.message : String(err);
                setError(message);
            } finally {
                setActionSession(null);
            }
        },
        [blockId, connName, refreshSessions, t]
    );

    const handleKillAll = React.useCallback(async () => {
        if (sortedSessions.length === 0) {
            return;
        }
        if (!window.confirm(t("term.tmuxSessions.confirm.killAll"))) {
            return;
        }
        setBulkAction(true);
        let lastError: string | null = null;
        try {
            for (const session of sortedSessions) {
                if (!session.name) {
                    continue;
                }
                try {
                    await RpcApi.TmuxKillSessionCommand(TabRpcClient, {
                        connname: connName,
                        sessionname: session.name,
                        logblockid: blockId,
                    });
                } catch (err) {
                    lastError = err instanceof Error ? err.message : String(err);
                }
            }
        } finally {
            setBulkAction(false);
        }
        if (lastError) {
            setError(lastError);
        }
        await refreshSessions();
    }, [blockId, connName, refreshSessions, sortedSessions, t]);

    const handleRename = React.useCallback((session: TmuxSessionInfo) => {
        const isManaged = session.name?.startsWith(TMUX_MANAGED_PREFIX);
        if (!isManaged) {
            return;
        }
        setRenameSessionName(session.name);
        setRenameValue((session.alias || "").trim());
    }, []);

    const handleRenameCancel = React.useCallback(() => {
        setRenameSessionName(null);
        setRenameValue("");
    }, []);

    const handleRenameSubmit = React.useCallback(
        async (session: TmuxSessionInfo) => {
            const isManaged = session.name?.startsWith(TMUX_MANAGED_PREFIX);
            if (!isManaged) {
                return;
            }
            const currentAlias = (session.alias || "").trim();
            const nextAlias = trimAliasInput(renameValue);
            if (nextAlias === currentAlias) {
                handleRenameCancel();
                return;
            }
            setActionSession(session.name);
            try {
                await RpcApi.TmuxSetSessionAliasCommand(TabRpcClient, {
                    connname: connName,
                    sessionname: session.name,
                    alias: nextAlias,
                    logblockid: blockId,
                });
                handleRenameCancel();
                await refreshSessions();
            } catch (err) {
                const message = err instanceof Error ? err.message : String(err);
                setError(message);
            } finally {
                setActionSession(null);
            }
        },
        [blockId, connName, handleRenameCancel, refreshSessions, renameValue]
    );

    const handleClearAlias = React.useCallback(
        async (session: TmuxSessionInfo) => {
            const isManaged = session.name?.startsWith(TMUX_MANAGED_PREFIX);
            if (!isManaged || !(session.alias || "").trim()) {
                return;
            }
            setActionSession(session.name);
            try {
                await RpcApi.TmuxSetSessionAliasCommand(TabRpcClient, {
                    connname: connName,
                    sessionname: session.name,
                    alias: "",
                    logblockid: blockId,
                });
                await refreshSessions();
            } catch (err) {
                const message = err instanceof Error ? err.message : String(err);
                setError(message);
            } finally {
                setActionSession(null);
            }
        },
        [blockId, connName, refreshSessions]
    );

    const renderBody = () => {
        if (loading) {
            return <div className="tmux-sessions-empty">{t("term.tmuxSessions.loading")}</div>;
        }
        if (error) {
            return <div className="tmux-sessions-empty error">{error}</div>;
        }
        if (sortedSessions.length === 0) {
            return <div className="tmux-sessions-empty">{t("term.tmuxSessions.empty")}</div>;
        }

        const actionsDisabled = actionSession !== null || bulkAction;

        return (
            <div className="tmux-sessions-list">
                <div className="tmux-session-row header">
                    <div>{t("term.tmuxSessions.columns.name")}</div>
                    <div>{t("term.tmuxSessions.columns.windows")}</div>
                    <div>{t("term.tmuxSessions.columns.attached")}</div>
                    <div>{t("term.tmuxSessions.columns.recent")}</div>
                    <div className="tmux-session-actions-header">{t("term.tmuxSessions.columns.actions")}</div>
                </div>
                {sortedSessions.map((session) => {
                    const isManaged = session.name?.startsWith(TMUX_MANAGED_PREFIX);
                    const isCurrent = currentSessionName !== "" && session.name === currentSessionName;
                    const attachedCount = session.attached ?? 0;
                    const alias = (session.alias || "").trim();
                    const displayName = alias || session.name;
                    const isEditing = renameSessionName === session.name;
                    const recent = formatRecentActivity(t, session.activity);
                    const activityTitle =
                        session.activity && session.activity > 0
                            ? new Date(session.activity * 1000).toLocaleString()
                            : t("term.tmuxSessions.activity.unknown");
                    return (
                        <div key={session.name} className={clsx("tmux-session-row", { managed: isManaged })}>
                            <div className="tmux-session-name">
                                <div className="tmux-session-name-main">
                                    <span className="ellipsis tmux-session-display-name" title={displayName}>
                                        {displayName}
                                    </span>
                                    {alias && (
                                        <span className="ellipsis tmux-session-id" title={session.name}>
                                            {session.name}
                                        </span>
                                    )}
                                </div>
                                {isCurrent && <span className="tmux-session-badge current">{t("term.tmuxSessions.current")}</span>}
                                {!isManaged && <span className="tmux-session-badge">{t("term.tmuxSessions.unmanaged")}</span>}
                            </div>
                            <div>{session.windows ?? 0}</div>
                            <div>
                                {attachedCount > 0 ? (
                                    <span className="tmux-session-attached">
                                        {t("term.tmuxSessions.attached")}
                                        {attachedCount > 1 ? ` x${attachedCount}` : ""}
                                    </span>
                                ) : (
                                    <span className="text-muted">{t("term.tmuxSessions.detached")}</span>
                                )}
                            </div>
                            <div title={activityTitle}>{recent}</div>
                            <div className="tmux-session-actions">
                                {isEditing ? (
                                    <>
                                        <input
                                            type="text"
                                            className="tmux-alias-input"
                                            value={renameValue}
                                            maxLength={MAX_ALIAS_LENGTH}
                                            onChange={(e) => setRenameValue(e.target.value)}
                                            onKeyDown={(e) => {
                                                if (e.key === "Enter") {
                                                    e.preventDefault();
                                                    void handleRenameSubmit(session);
                                                    return;
                                                }
                                                if (e.key === "Escape") {
                                                    e.preventDefault();
                                                    handleRenameCancel();
                                                }
                                            }}
                                            placeholder={t("term.tmuxSessions.prompt.alias")}
                                        />
                                        <Button className="green ghost" onClick={() => void handleRenameSubmit(session)} disabled={actionsDisabled}>
                                            {t("common.save")}
                                        </Button>
                                        <Button className="grey ghost" onClick={handleRenameCancel} disabled={actionsDisabled}>
                                            {t("common.cancel")}
                                        </Button>
                                    </>
                                ) : (
                                    <>
                                        <Button
                                            className="grey ghost"
                                            onClick={() => handleRename(session)}
                                            disabled={actionsDisabled || !isManaged}
                                        >
                                            {alias ? t("term.tmuxSessions.renameEdit") : t("term.tmuxSessions.rename")}
                                        </Button>
                                        <Button
                                            className="grey ghost"
                                            onClick={() => handleAttach(session, false)}
                                            disabled={actionsDisabled}
                                        >
                                            {t("term.tmuxSessions.attach")}
                                        </Button>
                                        <Button
                                            className="yellow ghost"
                                            onClick={() => handleAttach(session, true)}
                                            disabled={actionsDisabled}
                                        >
                                            {t("term.tmuxSessions.forceAttach")}
                                        </Button>
                                        <Button
                                            className="grey ghost"
                                            onClick={() => handleClearAlias(session)}
                                            disabled={actionsDisabled || !isManaged || !alias}
                                        >
                                            {t("term.tmuxSessions.clearName")}
                                        </Button>
                                        <Button
                                            className="red ghost"
                                            onClick={() => handleKill(session)}
                                            disabled={actionsDisabled}
                                        >
                                            {t("term.tmuxSessions.kill")}
                                        </Button>
                                    </>
                                )}
                            </div>
                        </div>
                    );
                })}
            </div>
        );
    };

    return (
        <Modal
            className="tmux-sessions-modal"
            onClose={() => modalsModel.popModal()}
            onClickBackdrop={() => modalsModel.popModal()}
        >
            <div className="tmux-sessions-header">
                <div className="tmux-sessions-title">{t("term.tmuxSessions.title")}</div>
                <div className="tmux-sessions-header-actions">
                    <Button className="grey ghost" onClick={refreshSessions} disabled={loading || bulkAction || actionSession !== null}>
                        <i className="fa-solid fa-rotate-right" />
                        <span>{t("term.tmuxSessions.refresh")}</span>
                    </Button>
                    <Button
                        className="red ghost"
                        onClick={handleKillAll}
                        disabled={loading || bulkAction || actionSession !== null || sortedSessions.length === 0}
                    >
                        <i className="fa-solid fa-trash" />
                        <span>{t("term.tmuxSessions.killAll")}</span>
                    </Button>
                </div>
            </div>
            <div className="tmux-sessions-subtitle">{connName}</div>
            {renderBody()}
        </Modal>
    );
};

TmuxSessionsModal.displayName = "TmuxSessionsModal";

export { TmuxSessionsModal };
