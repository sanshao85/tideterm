// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { GlobalModel } from "@/app/store/global-model";
import { useTabModel } from "@/app/store/tab-model";
import { getLayoutModelForStaticTab } from "@/layout/index";
import { atoms, getApi, WOS } from "@/store/global";
import { modalsModel } from "@/store/modalmodel";
import * as services from "@/store/services";
import { fireAndForget, useAtomValueSafe } from "@/util/util";
import { useAtomValue } from "jotai";
import { useEffect, useMemo } from "react";

const MetaKey_WindowTitleMode = "window:titlemode";
const MetaKey_WindowFixedTitle = "window:fixedtitle";

type WindowTitleMode = "auto" | "fixed";

function normalizeTitleMode(mode: unknown): WindowTitleMode {
    return mode === "fixed" ? "fixed" : "auto";
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

function joinConnectionAndPath(connection: string | null | undefined, path: string | null | undefined): string | null {
    const normalizedPath = (path ?? "").trim();
    if (normalizedPath.length === 0) {
        return null;
    }
    if (!isLocalConnectionName(connection)) {
        return `${connection} — ${normalizedPath}`;
    }
    return normalizedPath;
}

function expandHomeIfLocal(connection: string | null | undefined, path: string): string {
    if (!isLocalConnectionName(connection)) {
        return path;
    }
    if (!path) {
        return path;
    }
    if (path === "~") {
        return getApi().getHomeDir();
    }
    if (path.startsWith("~/")) {
        return getApi().getHomeDir() + path.slice(1);
    }
    return path;
}

export const WindowTitleManager = () => {
    const tabModel = useTabModel();
    const tabData = useAtomValue(tabModel.tabAtom);
    const windowData = useAtomValue(GlobalModel.getInstance().windowDataAtom);
    const windowId = windowData?.oid;

    // Avoid creating new atoms/layout models on every render.
    const staticTabId = useAtomValue(atoms.staticTabId);
    const layoutModel = useMemo(() => getLayoutModelForStaticTab(), [staticTabId]);
    const focusedNode = useAtomValueSafe(layoutModel?.focusedNode);
    const focusedBlockId = focusedNode?.data?.blockId ?? null;
    const focusedBlockAtom = useMemo(
        () => (focusedBlockId ? WOS.getWaveObjectAtom<Block>(`block:${focusedBlockId}`) : null),
        [focusedBlockId]
    );
    const focusedBlock = useAtomValueSafe(focusedBlockAtom);

    const titleMode = normalizeTitleMode(windowData?.meta?.[MetaKey_WindowTitleMode]);
    const fixedTitle = (windowData?.meta?.[MetaKey_WindowFixedTitle] as string | null)?.trim() ?? "";

    useEffect(() => {
        if (!windowId) {
            return;
        }
        const cleanupRename = getApi().onWindowTitleRename(() => {
            if (modalsModel.isModalOpen("RenameWindowModal")) {
                return;
            }
            modalsModel.pushModal("RenameWindowModal", { windowid: windowId });
        });
        const cleanupRestoreAuto = getApi().onWindowTitleRestoreAuto(() => {
            fireAndForget(() =>
                services.ObjectService.UpdateObjectMeta(WOS.makeORef("window", windowId), {
                    "window:titlemode": "auto",
                    "window:fixedtitle": null,
                } as any)
            );
        });
        return () => {
            cleanupRename();
            cleanupRestoreAuto();
        };
    }, [windowId]);

    useEffect(() => {
        let canceled = false;

        const setTitle = (title: string | null) => {
            if (canceled) {
                return;
            }
            const nextTitle = (title ?? "").trim();
            if (nextTitle.length === 0) {
                return;
            }
            if (document.title !== nextTitle) {
                document.title = nextTitle;
            }
        };

        const computeTitle = async () => {
            if (!windowId) {
                return;
            }

            if (titleMode === "fixed" && fixedTitle.length > 0) {
                setTitle(fixedTitle);
                return;
            }

            const focusedView = focusedBlock?.meta?.view ?? "";
            if (focusedView === "term") {
                const conn = focusedBlock?.meta?.connection as string | null;
                const cwdRaw = (focusedBlock?.meta?.["cmd:cwd"] as string | null) ?? "";
                const cwd = expandHomeIfLocal(conn, cwdRaw.trim());
                const title = joinConnectionAndPath(conn, cwd);
                if (title) {
                    setTitle(title);
                    return;
                }
            }

            if (focusedView === "preview") {
                const conn = focusedBlock?.meta?.connection as string | null;
                const file = (focusedBlock?.meta?.file as string | null) ?? "";
                if (file.trim().length > 0) {
                    const path = expandHomeIfLocal(conn, file.trim());
                    const title = joinConnectionAndPath(conn, path);
                    if (title) {
                        setTitle(title);
                        return;
                    }
                }
            }

            // Fallback: tab name (for non-term/non-filepanel focus)
            const tabName = (tabData?.name ?? "").trim();
            if (tabName.length > 0) {
                setTitle(tabName);
            }
        };

        computeTitle();
        return () => {
            canceled = true;
        };
    }, [
        windowId,
        titleMode,
        fixedTitle,
        tabData?.name,
        focusedBlockId,
        focusedBlock?.meta?.view,
        focusedBlock?.meta?.connection,
        focusedBlock?.meta?.["cmd:cwd"],
        focusedBlock?.meta?.file,
    ]);

    return null;
};
