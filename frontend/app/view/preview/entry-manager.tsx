// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { Button } from "@/app/element/button";
import { Input } from "@/app/element/input";
import { useT } from "@/app/i18n/i18n";
import type { I18nKey } from "@/app/i18n/i18n-core";
import React, { memo, useState } from "react";

export enum EntryManagerType {
    NewFile = "newFile",
    NewDirectory = "newFolder",
    EditName = "rename",
}

const entryManagerTitleKeys: Record<EntryManagerType, I18nKey> = {
    [EntryManagerType.NewFile]: "filemenu.newFile",
    [EntryManagerType.NewDirectory]: "filemenu.newFolder",
    [EntryManagerType.EditName]: "filemenu.rename",
};

export type EntryManagerOverlayProps = {
    forwardRef?: React.Ref<HTMLDivElement>;
    entryManagerType: EntryManagerType;
    startingValue?: string;
    onSave: (newValue: string) => void;
    onCancel?: () => void;
    style?: React.CSSProperties;
    getReferenceProps?: () => any;
};

export const EntryManagerOverlay = memo(
    ({
        entryManagerType,
        startingValue,
        onSave,
        onCancel,
        forwardRef,
        style,
        getReferenceProps,
    }: EntryManagerOverlayProps) => {
        const t = useT();
        const [value, setValue] = useState(startingValue);
        return (
            <div className="entry-manager-overlay" ref={forwardRef} style={style} {...(getReferenceProps?.() ?? {})}>
                <div className="entry-manager-type">{t(entryManagerTitleKeys[entryManagerType])}</div>
                <div className="entry-manager-input">
                    <Input
                        value={value}
                        onChange={setValue}
                        autoFocus={true}
                        onKeyDown={(e) => {
                            if (e.key === "Enter") {
                                e.preventDefault();
                                e.stopPropagation();
                                onSave(value);
                            }
                        }}
                    />
                </div>
                <div className="entry-manager-buttons">
                    <Button className="py-[4px]" onClick={() => onSave(value)}>
                        {t("common.save")}
                    </Button>
                    <Button className="py-[4px] red outlined" onClick={onCancel}>
                        {t("common.cancel")}
                    </Button>
                </div>
            </div>
        );
    }
);

EntryManagerOverlay.displayName = "EntryManagerOverlay";
