// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import clsx from "clsx";
import { useAtomValue } from "jotai";
import * as React from "react";

import { useT } from "@/app/i18n/i18n";
import { getConnStatusAtom } from "@/store/global";
import { isBlank, isLocalConnName, makeConnRoute } from "@/util/util";

import { proxyDockItemsAtom, restoreProxyFromDock } from "./proxy-dock-model";
import { ProxyRpcApi } from "./proxy-rpc";
import "./proxy-dock.scss";

function getRouteForConn(connName: string): string | null {
    if (isBlank(connName) || isLocalConnName(connName) || connName.startsWith("aws:")) {
        return null;
    }
    return makeConnRoute(connName);
}

const ProxyDockItem = ({ connection }: { connection: string }) => {
    const t = useT();
    const connStatus = useAtomValue(getConnStatusAtom(connection));
    const [running, setRunning] = React.useState<boolean | null>(null);
    const [port, setPort] = React.useState<number | null>(null);

    React.useEffect(() => {
        let cancelled = false;
        let interval: number | null = null;

        async function refresh() {
            if (cancelled) {
                return;
            }
            if (connStatus?.status !== "connected") {
                setRunning(null);
                setPort(null);
                return;
            }
            try {
                const route = getRouteForConn(connection);
                const status = await ProxyRpcApi.proxyStatus(route);
                if (cancelled) {
                    return;
                }
                setRunning(status.running);
                setPort(status.port);
            } catch {
                if (cancelled) {
                    return;
                }
                setRunning(null);
                setPort(null);
            }
        }

        void refresh();
        if (connStatus?.status === "connected") {
            interval = window.setInterval(refresh, 5000);
        }

        return () => {
            cancelled = true;
            if (interval != null) {
                clearInterval(interval);
            }
        };
    }, [connection, connStatus?.status]);

    const disconnected = connStatus?.status !== "connected";
    const statusClass = disconnected ? "disconnected" : running == null ? "unknown" : running ? "running" : "stopped";
    const title = disconnected
        ? t("proxy.dock.disconnected", { connection })
        : running == null
          ? t("proxy.dock.unknown", { connection })
          : running
            ? t("proxy.dock.running", { connection, port: port ?? "" })
            : t("proxy.dock.stopped", { connection, port: port ?? "" });

    return (
        <button
            type="button"
            className={clsx("proxy-dock-item", statusClass)}
            onClick={() => void restoreProxyFromDock(connection)}
            title={title}
        >
            <i className="fa fa-solid fa-server" />
            <span className="proxy-dock-indicator" />
        </button>
    );
};

const ProxyDock = () => {
    const items = useAtomValue(proxyDockItemsAtom);
    if (!items.length) {
        return null;
    }
    return (
        <div className="proxy-dock" aria-label="proxy-dock">
            {items.map((item) => (
                <ProxyDockItem key={item.connection} connection={item.connection} />
            ))}
        </div>
    );
};

export { ProxyDock };
