// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { Modal } from "@/app/modals/modal";
import { getAppLanguageFromSettings, t } from "@/app/i18n/i18n-core";
import { atoms, WOS } from "@/store/global";
import { modalsModel } from "@/store/modalmodel";
import * as services from "@/store/services";
import * as keyutil from "@/util/keyutil";
import { fireAndForget, useAtomValueSafe } from "@/util/util";
import { useAtomValue } from "jotai";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

const MetaKey_WindowTitleMode = "window:titlemode";
const MetaKey_WindowFixedTitle = "window:fixedtitle";

type RenameWindowModalProps = {
    windowid: string;
};

const RenameWindowModal = ({ windowid }: RenameWindowModalProps) => {
    const settings = useAtomValue(atoms.settingsAtom);
    const lang = getAppLanguageFromSettings(settings);
    const inputRef = useRef<HTMLInputElement>(null);

    const windowAtom = useMemo(() => (windowid ? WOS.getWaveObjectAtom<WaveWindow>(`window:${windowid}`) : null), [
        windowid,
    ]);
    const windowData = useAtomValueSafe(windowAtom);

    const initialValue = useMemo(() => {
        const mode = windowData?.meta?.[MetaKey_WindowTitleMode];
        if (mode === "fixed") {
            return String(windowData?.meta?.[MetaKey_WindowFixedTitle] ?? "");
        }
        return "";
    }, [windowData?.meta?.[MetaKey_WindowTitleMode], windowData?.meta?.[MetaKey_WindowFixedTitle]]);

    const [title, setTitle] = useState<string>(initialValue);
    const trimmedTitle = title.trim();

    useEffect(() => {
        setTitle(initialValue);
    }, [initialValue]);

    useEffect(() => {
        // Focus input after mount.
        const timeoutId = window.setTimeout(() => {
            inputRef.current?.focus();
            inputRef.current?.select();
        }, 0);
        return () => window.clearTimeout(timeoutId);
    }, []);

    const close = useCallback(() => {
        modalsModel.popModal();
    }, []);

    const save = useCallback(() => {
        const nextTitle = trimmedTitle;
        if (!windowid || nextTitle.length === 0) {
            return;
        }
        fireAndForget(async () => {
            await services.ObjectService.UpdateObjectMeta(WOS.makeORef("window", windowid), {
                "window:titlemode": "fixed",
                "window:fixedtitle": nextTitle,
            } as any);
            modalsModel.popModal();
        });
    }, [windowid, trimmedTitle]);

    const handleKeyDown = useCallback(
        (waveEvent: WaveKeyboardEvent): boolean => {
            if (keyutil.checkKeyPressed(waveEvent, "Escape")) {
                close();
                return true;
            }
            return false;
        },
        [close, save]
    );

    return (
        <Modal
            className="w-[520px]"
            onClose={close}
            onClickBackdrop={close}
            onCancel={close}
            cancelLabel={t(lang, "common.cancel")}
            onOk={save}
            okLabel={t(lang, "common.save")}
            okDisabled={trimmedTitle.length === 0}
        >
            <div className="flex flex-col gap-3">
                <div className="text-lg font-semibold">{t(lang, "windowtitle.rename.title")}</div>
                <input
                    ref={inputRef}
                    type="text"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder={t(lang, "windowtitle.rename.placeholder")}
                    maxLength={200}
                    className="bg-panel rounded-md border border-border py-2 px-3 text-inherit cursor-text focus:ring-2 focus:ring-accent focus:outline-none"
                    autoFocus={true}
                    onKeyDown={(e) => keyutil.keydownWrapper(handleKeyDown)(e)}
                />
            </div>
        </Modal>
    );
};

RenameWindowModal.displayName = "RenameWindowModal";

export { RenameWindowModal };
