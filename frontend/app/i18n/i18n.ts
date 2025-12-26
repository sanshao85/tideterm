// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { atoms } from "@/app/store/global";
import { useAtomValue } from "jotai";
import { useCallback } from "react";
import { getAppLanguageFromSettings, t } from "./i18n-core";
import type { AppLanguage, I18nKey } from "./i18n-core";

export type { AppLanguage, I18nKey } from "./i18n-core";
export { DefaultAppLanguage, getAppLanguageFromSettings, normalizeAppLanguage, t } from "./i18n-core";

export function useAppLanguage(): AppLanguage {
    const settings = useAtomValue(atoms.settingsAtom);
    return getAppLanguageFromSettings(settings);
}

export function useT(): (key: I18nKey, vars?: Record<string, string | number>) => string {
    const lang = useAppLanguage();
    return useCallback((key: I18nKey, vars?: Record<string, string | number>) => t(lang, key, vars), [lang]);
}

