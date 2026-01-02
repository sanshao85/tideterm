// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { atoms, globalStore } from "@/app/store/global";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { useAppLanguage, type AppLanguage, useT } from "@/app/i18n/i18n";
import type { WaveConfigViewModel } from "@/app/view/waveconfig/waveconfig-model";
import * as jotai from "jotai";
import { useState } from "react";

export function SettingsContent({ model }: { model: WaveConfigViewModel }) {
    const t = useT();
    const lang = useAppLanguage();
    const fullConfig = jotai.useAtomValue(atoms.fullConfigAtom);
    const [isUpdating, setIsUpdating] = useState(false);

    const remoteTmuxResumeEnabled = fullConfig?.settings?.["term:remotetmuxresume"] ?? true;

    const refreshConfigAndReloadSelectedFile = async () => {
        const refreshed = await RpcApi.GetFullConfigCommand(TabRpcClient);
        globalStore.set(atoms.fullConfigAtom, refreshed);
        const selectedFile = globalStore.get(model.selectedFileAtom);
        if (selectedFile) {
            await model.loadFile(selectedFile);
        }
    };

    const setLanguage = async (newLang: AppLanguage) => {
        if (newLang === lang || isUpdating) return;
        setIsUpdating(true);
        globalStore.set(model.errorMessageAtom, null);

        try {
            await RpcApi.SetConfigCommand(TabRpcClient, { "app:language": newLang });
            await refreshConfigAndReloadSelectedFile();
        } catch (e: any) {
            globalStore.set(model.errorMessageAtom, e?.message ? String(e.message) : String(e));
        } finally {
            setIsUpdating(false);
        }
    };

    const setRemoteTmuxResume = async (enabled: boolean) => {
        if (enabled === remoteTmuxResumeEnabled || isUpdating) return;
        setIsUpdating(true);
        globalStore.set(model.errorMessageAtom, null);

        try {
            await RpcApi.SetConfigCommand(TabRpcClient, { "term:remotetmuxresume": enabled });
            await refreshConfigAndReloadSelectedFile();
        } catch (e: any) {
            globalStore.set(model.errorMessageAtom, e?.message ? String(e.message) : String(e));
        } finally {
            setIsUpdating(false);
        }
    };

    return (
        <div className="flex flex-col gap-6 p-6 h-full overflow-auto">
            <div className="flex flex-col gap-1">
                <div className="text-lg font-semibold">{t("settings.language")}</div>
                <div className="text-sm text-muted-foreground">{t("settings.language.description")}</div>
            </div>

            <div className="flex flex-col gap-3">
                <label className="flex items-center gap-3 cursor-pointer">
                    <input
                        type="radio"
                        name="app-language"
                        checked={lang === "en"}
                        disabled={isUpdating}
                        onChange={() => setLanguage("en")}
                    />
                    <span className="text-sm">{t("settings.language.english")}</span>
                </label>

                <label className="flex items-center gap-3 cursor-pointer">
                    <input
                        type="radio"
                        name="app-language"
                        checked={lang === "zh-CN"}
                        disabled={isUpdating}
                        onChange={() => setLanguage("zh-CN")}
                    />
                    <span className="text-sm">{t("settings.language.chinese")}</span>
                </label>
            </div>

            <div className="flex flex-col gap-1">
                <div className="text-lg font-semibold">{t("settings.remoteTmuxResume")}</div>
                <div className="text-sm text-muted-foreground">{t("settings.remoteTmuxResume.description")}</div>
            </div>

            <div className="flex flex-col gap-3">
                <label className="flex items-center gap-3 cursor-pointer">
                    <input
                        type="checkbox"
                        checked={remoteTmuxResumeEnabled}
                        disabled={isUpdating}
                        onChange={(e) => setRemoteTmuxResume(e.target.checked)}
                    />
                    <span className="text-sm">{t("settings.remoteTmuxResume.toggle")}</span>
                </label>
            </div>
        </div>
    );
}
