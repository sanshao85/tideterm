import { createBlock, getApi } from "@/app/store/global";
import type { I18nKey } from "@/app/i18n/i18n-core";
import { isMacOS, isWindows } from "./platformutil";
import { fireAndForget } from "./util";
import { formatRemoteUri } from "./waveutil";

export type TranslateFn = (key: I18nKey, vars?: Record<string, string | number>) => string;

function getNativeLabel(isDirectory: boolean, t: TranslateFn): string {
    if (!isDirectory) {
        return t("filemenu.openFileInDefaultApplication");
    }
    if (isMacOS()) {
        return t("filemenu.revealInFinder");
    }
    if (isWindows()) {
        return t("filemenu.revealInExplorer");
    }
    return t("filemenu.revealInFileManager");
}

export function addOpenMenuItems(
    menu: ContextMenuItem[],
    conn: string,
    finfo: FileInfo,
    t: TranslateFn
): ContextMenuItem[] {
    if (!finfo) {
        return menu;
    }
    menu.push({
        type: "separator",
    });
    if (!conn) {
        // TODO:  resolve correct host path if connection is WSL
        // if the entry is a directory, reveal it in the file manager, if the entry is a file, reveal its parent directory
        menu.push({
            label: getNativeLabel(true, t),
            click: () => {
                getApi().openNativePath(finfo.isdir ? finfo.path : finfo.dir);
            },
        });
        // if the entry is a file, open it in the default application
        if (!finfo.isdir) {
            menu.push({
                label: getNativeLabel(false, t),
                click: () => {
                    getApi().openNativePath(finfo.path);
                },
            });
        }
    } else {
        menu.push({
            label: t("filemenu.downloadFile"),
            click: () => {
                const remoteUri = formatRemoteUri(finfo.path, conn);
                getApi().downloadFile(remoteUri);
            },
        });
    }
    menu.push({
        type: "separator",
    });
    if (!finfo.isdir) {
        menu.push({
            label: t("filemenu.openPreviewInNewBlock"),
            click: () =>
                fireAndForget(async () => {
                    const blockDef: BlockDef = {
                        meta: {
                            view: "preview",
                            file: finfo.path,
                            connection: conn,
                        },
                    };
                    await createBlock(blockDef);
                }),
        });
    }
    // TODO: improve behavior as we add more connection types
    if (!conn?.startsWith("aws:")) {
        menu.push({
            label: t("filemenu.openTerminalInNewBlock"),
            click: () => {
                const termBlockDef: BlockDef = {
                    meta: {
                        controller: "shell",
                        view: "term",
                        "cmd:cwd": finfo.isdir ? finfo.path : finfo.dir,
                        connection: conn,
                    },
                };
                fireAndForget(() => createBlock(termBlockDef));
            },
        });
    }
    return menu;
}
