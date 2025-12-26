// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { waveEventSubscribe } from "@/app/store/wps";
import { RpcApi } from "@/app/store/wshclientapi";
import { getAppLanguageFromSettings, t, type AppLanguage } from "@/app/i18n/i18n-core";
import * as electron from "electron";
import { fireAndForget } from "../frontend/util/util";
import { focusedBuilderWindow, getBuilderWindowById } from "./emain-builder";
import { openBuilderWindow } from "./emain-ipc";
import { isDev, unamePlatform } from "./emain-platform";
import { clearTabCache } from "./emain-tabview";
import { decreaseZoomLevel, increaseZoomLevel } from "./emain-util";
import {
    createNewWaveWindow,
    createWorkspace,
    focusedWaveWindow,
    getAllWaveWindows,
    getWaveWindowByWebContentsId,
    getWaveWindowByWorkspaceId,
    relaunchBrowserWindows,
    WaveBrowserWindow,
} from "./emain-window";
import { ElectronWshClient } from "./emain-wsh";
import { updater } from "./updater";

type AppMenuCallbacks = {
    createNewWaveWindow: () => Promise<void>;
    relaunchBrowserWindows: () => Promise<void>;
};

function getWindowWebContents(window: electron.BaseWindow): electron.WebContents {
    if (window == null) {
        return null;
    }
    // Check BrowserWindow first (for Tsunami Builder windows)
    if (window instanceof electron.BrowserWindow) {
        return window.webContents;
    }
    // Check WaveBrowserWindow (for main Wave windows with tab views)
    if (window instanceof WaveBrowserWindow) {
        if (window.activeTabView) {
            return window.activeTabView.webContents;
        }
        return null;
    }
    return null;
}

async function getWorkspaceMenu(lang: AppLanguage, ww?: WaveBrowserWindow): Promise<Electron.MenuItemConstructorOptions[]> {
    const workspaceList = await RpcApi.WorkspaceListCommand(ElectronWshClient);
    const workspaceMenu: Electron.MenuItemConstructorOptions[] = [
        {
            label: t(lang, "menu.createWorkspace"),
            click: (_, window) => fireAndForget(() => createWorkspace((window as WaveBrowserWindow) ?? ww)),
        },
    ];
    function getWorkspaceSwitchAccelerator(i: number): string {
        if (i < 9) {
            return unamePlatform == "darwin" ? `Command+Control+${i + 1}` : `Alt+Control+${i + 1}`;
        }
    }
    workspaceList?.length &&
        workspaceMenu.push(
            { type: "separator" },
            ...workspaceList.map<Electron.MenuItemConstructorOptions>((workspace, i) => {
                return {
                    label: `${workspace.workspacedata.name}`,
                    click: (_, window) => {
                        ((window as WaveBrowserWindow) ?? ww)?.switchWorkspace(workspace.workspacedata.oid);
                    },
                    accelerator: getWorkspaceSwitchAccelerator(i),
                };
            })
        );
    return workspaceMenu;
}

function makeEditMenu(lang: AppLanguage, fullConfig?: FullConfigType): Electron.MenuItemConstructorOptions[] {
    let pasteAccelerator: string;
    if (unamePlatform === "darwin") {
        pasteAccelerator = "Command+V";
    } else {
        const ctrlVPaste = fullConfig?.settings?.["app:ctrlvpaste"];
        if (ctrlVPaste == null) {
            pasteAccelerator = unamePlatform === "win32" ? "Control+V" : "";
        } else if (ctrlVPaste) {
            pasteAccelerator = "Control+V";
        } else {
            pasteAccelerator = "";
        }
    }
    return [
        {
            role: "undo",
            label: t(lang, "menu.undo"),
            accelerator: unamePlatform === "darwin" ? "Command+Z" : "",
        },
        {
            role: "redo",
            label: t(lang, "menu.redo"),
            accelerator: unamePlatform === "darwin" ? "Command+Shift+Z" : "",
        },
        { type: "separator" },
        {
            role: "cut",
            label: t(lang, "menu.cut"),
            accelerator: unamePlatform === "darwin" ? "Command+X" : "",
        },
        {
            role: "copy",
            label: t(lang, "menu.copy"),
            accelerator: unamePlatform === "darwin" ? "Command+C" : "",
        },
        {
            role: "paste",
            label: t(lang, "menu.paste"),
            accelerator: pasteAccelerator,
        },
        {
            role: "pasteAndMatchStyle",
            label: t(lang, "menu.pasteAndMatchStyle"),
            accelerator: unamePlatform === "darwin" ? "Command+Shift+V" : "",
        },
        {
            role: "delete",
            label: t(lang, "menu.delete"),
        },
        {
            role: "selectAll",
            label: t(lang, "menu.selectAll"),
            accelerator: unamePlatform === "darwin" ? "Command+A" : "",
        },
    ];
}

function makeFileMenu(
    numWaveWindows: number,
    callbacks: AppMenuCallbacks,
    fullConfig: FullConfigType,
    lang: AppLanguage
): Electron.MenuItemConstructorOptions[] {
    const fileMenu: Electron.MenuItemConstructorOptions[] = [
        {
            label: t(lang, "menu.newWindow"),
            accelerator: "CommandOrControl+Shift+N",
            click: () => fireAndForget(callbacks.createNewWaveWindow),
        },
        {
            role: "close",
            label: t(lang, "menu.closeWindow"),
            accelerator: "",
            click: () => {
                focusedWaveWindow?.close();
            },
        },
    ];
    const featureWaveAppBuilder = fullConfig?.settings?.["feature:waveappbuilder"];
    if (isDev || featureWaveAppBuilder) {
        fileMenu.splice(1, 0, {
            label: t(lang, "menu.newWaveAppBuilderWindow"),
            accelerator: unamePlatform === "darwin" ? "Command+Shift+B" : "Alt+Shift+B",
            click: () => openBuilderWindow(""),
        });
    }
    if (numWaveWindows == 0) {
        fileMenu.push({
            label: `${t(lang, "menu.newWindow")} (hidden-1)`,
            accelerator: unamePlatform === "darwin" ? "Command+N" : "Alt+N",
            acceleratorWorksWhenHidden: true,
            visible: false,
            click: () => fireAndForget(callbacks.createNewWaveWindow),
        });
        fileMenu.push({
            label: `${t(lang, "menu.newWindow")} (hidden-2)`,
            accelerator: unamePlatform === "darwin" ? "Command+T" : "Alt+T",
            acceleratorWorksWhenHidden: true,
            visible: false,
            click: () => fireAndForget(callbacks.createNewWaveWindow),
        });
    }
    return fileMenu;
}

function makeAppMenuItems(webContents: electron.WebContents, lang: AppLanguage): Electron.MenuItemConstructorOptions[] {
    const appMenuItems: Electron.MenuItemConstructorOptions[] = [
        {
            label: t(lang, "menu.aboutWaveTerminal"),
            click: (_, window) => {
                (getWindowWebContents(window) ?? webContents)?.send("menu-item-about");
            },
        },
        {
            label: t(lang, "menu.checkForUpdates"),
            click: () => {
                fireAndForget(() => updater?.checkForUpdates(true));
            },
        },
        { type: "separator" },
    ];
    if (unamePlatform === "darwin") {
        appMenuItems.push(
            { role: "services", label: t(lang, "menu.services") },
            { type: "separator" },
            { role: "hide", label: t(lang, "menu.hide") },
            { role: "hideOthers", label: t(lang, "menu.hideOthers") },
            { type: "separator" }
        );
    }
    appMenuItems.push({ role: "quit", label: t(lang, "menu.quit") });
    return appMenuItems;
}

function makeViewMenu(
    webContents: electron.WebContents,
    callbacks: AppMenuCallbacks,
    isBuilderWindowFocused: boolean,
    fullscreenOnLaunch: boolean,
    lang: AppLanguage
): Electron.MenuItemConstructorOptions[] {
    const devToolsAccel = unamePlatform === "darwin" ? "Option+Command+I" : "Alt+Shift+I";
    return [
        {
            label: isBuilderWindowFocused ? t(lang, "menu.reloadWindow") : t(lang, "menu.reloadTab"),
            accelerator: "Shift+CommandOrControl+R",
            click: (_, window) => {
                (getWindowWebContents(window) ?? webContents)?.reloadIgnoringCache();
            },
        },
        {
            label: t(lang, "menu.relaunchAllWindows"),
            click: () => callbacks.relaunchBrowserWindows(),
        },
        {
            label: t(lang, "menu.clearTabCache"),
            click: () => clearTabCache(),
        },
        {
            label: t(lang, "menu.toggleDevTools"),
            accelerator: devToolsAccel,
            click: (_, window) => {
                let wc = getWindowWebContents(window) ?? webContents;
                wc?.toggleDevTools();
            },
        },
        { type: "separator" },
        {
            label: t(lang, "menu.resetZoom"),
            accelerator: "CommandOrControl+0",
            click: (_, window) => {
                const wc = getWindowWebContents(window) ?? webContents;
                if (wc) {
                    wc.setZoomFactor(1);
                    wc.send("zoom-factor-change", 1);
                }
            },
        },
        {
            label: t(lang, "menu.zoomIn"),
            accelerator: "CommandOrControl+=",
            click: (_, window) => {
                const wc = getWindowWebContents(window) ?? webContents;
                if (wc) {
                    increaseZoomLevel(wc);
                }
            },
        },
        {
            label: `${t(lang, "menu.zoomIn")} (hidden)`,
            accelerator: "CommandOrControl+Shift+=",
            click: (_, window) => {
                const wc = getWindowWebContents(window) ?? webContents;
                if (wc) {
                    increaseZoomLevel(wc);
                }
            },
            visible: false,
            acceleratorWorksWhenHidden: true,
        },
        {
            label: t(lang, "menu.zoomOut"),
            accelerator: "CommandOrControl+-",
            click: (_, window) => {
                const wc = getWindowWebContents(window) ?? webContents;
                if (wc) {
                    decreaseZoomLevel(wc);
                }
            },
        },
        {
            label: `${t(lang, "menu.zoomOut")} (hidden)`,
            accelerator: "CommandOrControl+Shift+-",
            click: (_, window) => {
                const wc = getWindowWebContents(window) ?? webContents;
                if (wc) {
                    decreaseZoomLevel(wc);
                }
            },
            visible: false,
            acceleratorWorksWhenHidden: true,
        },
        {
            label: t(lang, "menu.launchOnFullScreen"),
            submenu: [
                {
                    label: t(lang, "menu.on"),
                    type: "radio",
                    checked: fullscreenOnLaunch,
                    click: () => {
                        RpcApi.SetConfigCommand(ElectronWshClient, { "window:fullscreenonlaunch": true });
                    },
                },
                {
                    label: t(lang, "menu.off"),
                    type: "radio",
                    checked: !fullscreenOnLaunch,
                    click: () => {
                        RpcApi.SetConfigCommand(ElectronWshClient, { "window:fullscreenonlaunch": false });
                    },
                },
            ],
        },
        { type: "separator" },
        {
            role: "togglefullscreen",
            label: t(lang, "menu.fullscreen"),
        },
    ];
}

async function makeFullAppMenu(callbacks: AppMenuCallbacks, workspaceOrBuilderId?: string): Promise<Electron.Menu> {
    const numWaveWindows = getAllWaveWindows().length;
    const webContents = workspaceOrBuilderId && getWebContentsByWorkspaceOrBuilderId(workspaceOrBuilderId);

    const isBuilderWindowFocused = focusedBuilderWindow != null;
    let fullscreenOnLaunch = false;
    let fullConfig: FullConfigType = null;
    try {
        fullConfig = await RpcApi.GetFullConfigCommand(ElectronWshClient);
        fullscreenOnLaunch = fullConfig?.settings["window:fullscreenonlaunch"];
    } catch (e) {
        console.error("Error fetching config:", e);
    }
    const lang = getAppLanguageFromSettings(fullConfig?.settings);
    const appMenuItems = makeAppMenuItems(webContents, lang);
    const editMenu = makeEditMenu(lang, fullConfig);
    const fileMenu = makeFileMenu(numWaveWindows, callbacks, fullConfig, lang);
    const viewMenu = makeViewMenu(webContents, callbacks, isBuilderWindowFocused, fullscreenOnLaunch, lang);
    let workspaceMenu: Electron.MenuItemConstructorOptions[] = null;
    try {
        workspaceMenu = await getWorkspaceMenu(lang);
    } catch (e) {
        console.error("getWorkspaceMenu error:", e);
    }
    const windowMenu: Electron.MenuItemConstructorOptions[] = [
        {
            label: t(lang, "menu.windowTitle"),
            submenu: [
                {
                    label: t(lang, "menu.renameWindow"),
                    click: (_, window) => {
                        // Prefer the window that Electron reports for the menu click; fall back to the captured
                        // webContents for pop-up menus (workspace/builder app menus).
                        const wc = getWindowWebContents(window) ?? webContents;
                        if (!wc) {
                            console.error("invalid window for window title rename click handler:", window);
                            return;
                        }
                        // Ensure the correct Wave window is focused before opening the modal.
                        getWaveWindowByWebContentsId(wc.id)?.focus();
                        wc.send("window-title-rename");
                    },
                },
                {
                    label: t(lang, "menu.restoreAutoWindowTitle"),
                    click: (_, window) => {
                        const wc = getWindowWebContents(window) ?? webContents;
                        if (!wc) {
                            console.error("invalid window for window title restore click handler:", window);
                            return;
                        }
                        getWaveWindowByWebContentsId(wc.id)?.focus();
                        wc.send("window-title-restore-auto");
                    },
                },
            ],
        },
        { type: "separator" },
        { role: "minimize", accelerator: "" },
        { role: "zoom" },
        { type: "separator" },
        { role: "front" },
    ];
    const menuTemplate: Electron.MenuItemConstructorOptions[] = [
        { role: "appMenu", submenu: appMenuItems },
        { role: "fileMenu", label: t(lang, "menu.file"), submenu: fileMenu },
        { role: "editMenu", label: t(lang, "menu.edit"), submenu: editMenu },
        { role: "viewMenu", label: t(lang, "menu.view"), submenu: viewMenu },
    ];
    if (workspaceMenu != null && !isBuilderWindowFocused) {
        menuTemplate.push({
            label: t(lang, "menu.workspace"),
            id: "workspace-menu",
            submenu: workspaceMenu,
        });
    }
    menuTemplate.push({
        role: "windowMenu",
        label: t(lang, "menu.window"),
        submenu: windowMenu,
    });
    return electron.Menu.buildFromTemplate(menuTemplate);
}

export function instantiateAppMenu(workspaceOrBuilderId?: string): Promise<electron.Menu> {
    return makeFullAppMenu(
        {
            createNewWaveWindow,
            relaunchBrowserWindows,
        },
        workspaceOrBuilderId
    );
}

// does not a set a menu on windows
export function makeAndSetAppMenu() {
    if (unamePlatform === "win32") {
        return;
    }
    fireAndForget(async () => {
        const menu = await instantiateAppMenu();
        electron.Menu.setApplicationMenu(menu);
        makeDockTaskbar();
    });
}

waveEventSubscribe({
    eventType: "workspace:update",
    handler: makeAndSetAppMenu,
});
waveEventSubscribe({
    eventType: "config",
    handler: makeAndSetAppMenu,
});

function getWebContentsByWorkspaceOrBuilderId(workspaceOrBuilderId: string): electron.WebContents {
    const ww = getWaveWindowByWorkspaceId(workspaceOrBuilderId);
    if (ww) {
        return ww.activeTabView?.webContents;
    }

    const bw = getBuilderWindowById(workspaceOrBuilderId);
    if (bw) {
        return bw.webContents;
    }

    return null;
}

function convertMenuDefArrToMenu(
    webContents: electron.WebContents,
    menuDefArr: ElectronContextMenuItem[]
): electron.Menu {
    const menuItems: electron.MenuItem[] = [];
    for (const menuDef of menuDefArr) {
        const menuItemTemplate: electron.MenuItemConstructorOptions = {
            role: menuDef.role as any,
            label: menuDef.label,
            type: menuDef.type,
            click: (_, window) => {
                const wc = getWindowWebContents(window) ?? webContents;
                if (!wc) {
                    console.error("invalid window for context menu click handler:", window);
                    return;
                }
                wc.send("contextmenu-click", menuDef.id);
            },
            checked: menuDef.checked,
            enabled: menuDef.enabled,
        };
        if (menuDef.submenu != null) {
            menuItemTemplate.submenu = convertMenuDefArrToMenu(webContents, menuDef.submenu);
        }
        const menuItem = new electron.MenuItem(menuItemTemplate);
        menuItems.push(menuItem);
    }
    return electron.Menu.buildFromTemplate(menuItems);
}

electron.ipcMain.on(
    "contextmenu-show",
    (event, workspaceOrBuilderId: string, menuDefArr: ElectronContextMenuItem[]) => {
        if (menuDefArr.length === 0) {
            event.returnValue = true;
            return;
        }
        fireAndForget(async () => {
            const webContents = getWebContentsByWorkspaceOrBuilderId(workspaceOrBuilderId);
            if (!webContents) {
                console.error("invalid window for context menu:", workspaceOrBuilderId);
                return;
            }

            const menu = convertMenuDefArrToMenu(webContents, menuDefArr);
            menu.popup();
        });
        event.returnValue = true;
    }
);

electron.ipcMain.on("workspace-appmenu-show", (event, workspaceId: string) => {
    fireAndForget(async () => {
        const webContents = getWebContentsByWorkspaceOrBuilderId(workspaceId);
        if (!webContents) {
            console.error("invalid window for workspace app menu:", workspaceId);
            return;
        }
        const menu = await instantiateAppMenu(workspaceId);
        menu.popup();
    });
    event.returnValue = true;
});

electron.ipcMain.on("builder-appmenu-show", (event, builderId: string) => {
    fireAndForget(async () => {
        const webContents = getWebContentsByWorkspaceOrBuilderId(builderId);
        if (!webContents) {
            console.error("invalid window for builder app menu:", builderId);
            return;
        }
        const menu = await instantiateAppMenu(builderId);
        menu.popup();
    });
    event.returnValue = true;
});

function makeDockTaskbar() {
    if (unamePlatform !== "darwin") return;
    fireAndForget(async () => {
        let fullConfig: FullConfigType = null;
        try {
            fullConfig = await RpcApi.GetFullConfigCommand(ElectronWshClient);
        } catch (e) {
            console.error("Error fetching config for dock menu:", e);
        }
        const lang = getAppLanguageFromSettings(fullConfig?.settings);
        const dockMenu = electron.Menu.buildFromTemplate([
            {
                label: t(lang, "menu.newWindow"),
                click() {
                    fireAndForget(createNewWaveWindow);
                },
            },
        ]);
        electron.app.dock.setMenu(dockMenu);
    });
}

export { makeDockTaskbar };
