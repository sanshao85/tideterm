// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as jotai from "jotai";

import { atoms, createBlock, globalStore, refocusNode, WOS } from "@/store/global";
import { uxCloseBlock } from "@/store/keymodel";
import { isBlank } from "@/util/util";

export type ProxyDockItem = {
    connection: string;
};

export const proxyDockItemsAtom = jotai.atom<ProxyDockItem[]>([]);

function normalizeConnection(connection?: string | null): string {
    const trimmed = (connection ?? "").trim();
    return trimmed ? trimmed : "local";
}

export function addProxyDockItem(connection?: string | null): void {
    const normalized = normalizeConnection(connection);
    globalStore.set(proxyDockItemsAtom, (prev) => {
        if (prev.some((item) => item.connection === normalized)) {
            return prev;
        }
        return [...prev, { connection: normalized }];
    });
}

export function removeProxyDockItem(connection?: string | null): void {
    const normalized = normalizeConnection(connection);
    globalStore.set(proxyDockItemsAtom, (prev) => prev.filter((item) => item.connection !== normalized));
}

function findExistingProxyBlockId(connection: string): string | null {
    const normalized = normalizeConnection(connection);
    const tabId = globalStore.get(atoms.staticTabId);
    if (isBlank(tabId)) {
        return null;
    }
    const tabAtom = WOS.getWaveObjectAtom<Tab>(WOS.makeORef("tab", tabId));
    const tabData = globalStore.get(tabAtom);
    const blockIds = tabData?.blockids ?? [];
    for (const blockId of blockIds) {
        const blockAtom = WOS.getWaveObjectAtom<Block>(WOS.makeORef("block", blockId));
        const blockData = globalStore.get(blockAtom);
        if (blockData?.meta?.view !== "proxy") {
            continue;
        }
        const conn = normalizeConnection(blockData?.meta?.connection);
        if (conn === normalized) {
            return blockId;
        }
    }
    return null;
}

export function minimizeProxyBlock(blockId: string): void {
    const tabId = globalStore.get(atoms.staticTabId);
    if (!isBlank(tabId)) {
        const ws = globalStore.get(atoms.workspace);
        const pinned = ws?.pinnedtabids?.includes(tabId) ?? false;
        if (pinned) {
            const tabAtom = WOS.getWaveObjectAtom<Tab>(WOS.makeORef("tab", tabId));
            const tabData = globalStore.get(tabAtom);
            const blockCount = tabData?.blockids?.length ?? 0;
            if (blockCount === 1) {
                uxCloseBlock(blockId);
                return;
            }
        }
    }
    const blockAtom = WOS.getWaveObjectAtom<Block>(WOS.makeORef("block", blockId));
    const blockData = globalStore.get(blockAtom);
    const connName = normalizeConnection(blockData?.meta?.connection);
    addProxyDockItem(connName);
    uxCloseBlock(blockId);
}

export async function restoreProxyFromDock(connection: string): Promise<void> {
    const normalized = normalizeConnection(connection);

    const existingBlockId = findExistingProxyBlockId(normalized);
    if (existingBlockId) {
        removeProxyDockItem(normalized);
        refocusNode(existingBlockId);
        return;
    }

    await createBlock({
        meta: {
            view: "proxy",
            connection: normalized,
        },
    });
    removeProxyDockItem(normalized);
}
