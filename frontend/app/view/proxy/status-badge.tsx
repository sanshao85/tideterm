// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import clsx from "clsx";
import * as React from "react";

import { useT } from "@/app/i18n/i18n";

type StatusBadgeProps = {
    running: boolean;
    size?: "small" | "medium" | "large";
};

export const StatusBadge = React.memo(({ running, size = "medium" }: StatusBadgeProps) => {
    const t = useT();
    return (
        <span className={clsx("status-badge-indicator", size, running ? "running" : "stopped")}>
            <span className="status-dot" />
            <span className="status-text">{running ? t("proxy.status.running") : t("proxy.status.stopped")}</span>
        </span>
    );
});
