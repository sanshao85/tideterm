// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { fireAndForget } from "@/util/util";
import { app, dialog, ipcMain, shell } from "electron";
import envPaths from "env-paths";
import { existsSync, mkdirSync } from "fs";
import os from "os";
import path from "path";
import { WaveDevVarName, WaveDevViteVarName } from "../frontend/util/isdev";
import * as keyutil from "../frontend/util/keyutil";

// This is a little trick to ensure that Electron puts all its runtime data into a subdirectory to avoid conflicts with our own data.
// On macOS, it will store to ~/Library/Application \Support/tideterm/electron
// On Linux, it will store to ~/.config/tideterm/electron
// On Windows, it will store to %LOCALAPPDATA%/tideterm/electron
app.setName("tideterm/electron");

const isDev = !app.isPackaged;
const isDevVite = isDev && process.env.ELECTRON_RENDERER_URL;
console.log(`Running in ${isDev ? "development" : "production"} mode`);
if (isDev) {
    process.env[WaveDevVarName] = "1";
}
if (isDevVite) {
    process.env[WaveDevViteVarName] = "1";
}

const waveDirNamePrefix = "tideterm";
const waveDirNameSuffix = isDev ? "dev" : "";
const waveDirName = `${waveDirNamePrefix}${waveDirNameSuffix ? `-${waveDirNameSuffix}` : ""}`;

const paths = envPaths("tideterm", { suffix: waveDirNameSuffix });

app.setName(isDev ? "TideTerm (Dev)" : "TideTerm");
const unamePlatform = process.platform;
const unameArch: string = process.arch;
keyutil.setKeyUtilPlatform(unamePlatform);

const WaveConfigHomeVarName = "TIDETERM_CONFIG_HOME";
const WaveDataHomeVarName = "TIDETERM_DATA_HOME";
const WaveHomeVarName = "TIDETERM_HOME";

export function checkIfRunningUnderARM64Translation(fullConfig: FullConfigType) {
    if (!fullConfig.settings["app:dismissarchitecturewarning"] && app.runningUnderARM64Translation) {
        console.log("Running under ARM64 translation, alerting user");
        const dialogOpts: Electron.MessageBoxOptions = {
            type: "warning",
            buttons: ["Dismiss", "Learn More"],
            title: "TideTerm has detected a performance issue",
            message: `TideTerm is running in ARM64 translation mode which may impact performance.\n\nRecommendation: Use a native build for optimal performance.`,
        };

        const choice = dialog.showMessageBoxSync(null, dialogOpts);
        if (choice === 1) {
            // Open the documentation URL
            console.log("User chose to learn more");
            fireAndForget(() =>
                shell.openExternal(
                    "https://github.com/sanshao85/tideterm#readme"
                )
            );
            throw new Error("User redirected to docsite to learn more about ARM64 translation, exiting");
        } else {
            console.log("User dismissed the dialog");
        }
    }
}

/**
 * Gets the path to the old TideTerm home directory (defaults to `~/.tideterm`).
 * @returns The path to the directory if it exists and contains valid data for the current app, otherwise null.
 */
function getWaveHomeDir(): string {
    let home = process.env[WaveHomeVarName];
    if (!home) {
        const homeDir = app.getPath("home");
        if (homeDir) {
            home = path.join(homeDir, `.${waveDirName}`);
        }
    }
    // If home exists and it has `tideterm.lock` in it, we know it has valid data for the current app.
    if (home && existsSync(home) && existsSync(path.join(home, "tideterm.lock"))) {
        return home;
    }
    return null;
}

/**
 * Ensure the given path exists, creating it recursively if it doesn't.
 * @param path The path to ensure.
 * @returns The same path, for chaining.
 */
function ensurePathExists(path: string): string {
    if (!existsSync(path)) {
        mkdirSync(path, { recursive: true });
    }
    return path;
}

/**
 * Gets the path to the directory where TideTerm configurations are stored. Creates the directory if it does not exist.
 * Handles backwards compatibility with the old TideTerm Home directory model, where configurations and data were stored together.
 * @returns The path where configurations should be stored.
 */
function getWaveConfigDir(): string {
    // If wave home dir exists, use it for backwards compatibility
    const waveHomeDir = getWaveHomeDir();
    if (waveHomeDir) {
        return path.join(waveHomeDir, "config");
    }

    const override = process.env[WaveConfigHomeVarName];
    const xdgConfigHome = process.env.XDG_CONFIG_HOME;
    let retVal: string;
    if (override) {
        retVal = override;
    } else if (xdgConfigHome) {
        retVal = path.join(xdgConfigHome, waveDirName);
    } else {
        retVal = path.join(app.getPath("home"), ".config", waveDirName);
    }
    return ensurePathExists(retVal);
}

/**
 * Gets the path to the directory where TideTerm data is stored. Creates the directory if it does not exist.
 * Handles backwards compatibility with the old TideTerm Home directory model, where configurations and data were stored together.
 * @returns The path where data should be stored.
 */
function getWaveDataDir(): string {
    // If wave home dir exists, use it for backwards compatibility
    const waveHomeDir = getWaveHomeDir();
    if (waveHomeDir) {
        return waveHomeDir;
    }

    const override = process.env[WaveDataHomeVarName];
    const xdgDataHome = process.env.XDG_DATA_HOME;
    let retVal: string;
    if (override) {
        retVal = override;
    } else if (xdgDataHome) {
        retVal = path.join(xdgDataHome, waveDirName);
    } else {
        retVal = paths.data;
    }
    return ensurePathExists(retVal);
}

function getElectronAppBasePath(): string {
    // import.meta.dirname in dev points to waveterm/dist/main
    return path.dirname(import.meta.dirname);
}

function getElectronAppUnpackedBasePath(): string {
    return getElectronAppBasePath().replace("app.asar", "app.asar.unpacked");
}

function getElectronAppResourcesPath(): string {
    if (isDev) {
        // import.meta.dirname in dev points to waveterm/dist/main
        return path.dirname(import.meta.dirname);
    }
    return process.resourcesPath;
}

const wavesrvBinName = `wavesrv.${unameArch}`;

function getWaveSrvPath(): string {
    if (process.platform === "win32") {
        const winBinName = `${wavesrvBinName}.exe`;
        const appPath = path.join(getElectronAppUnpackedBasePath(), "bin", winBinName);
        return `${appPath}`;
    }
    return path.join(getElectronAppUnpackedBasePath(), "bin", wavesrvBinName);
}

function getWaveSrvCwd(): string {
    return getWaveDataDir();
}

ipcMain.on("get-is-dev", (event) => {
    event.returnValue = isDev;
});
ipcMain.on("get-platform", (event, url) => {
    event.returnValue = unamePlatform;
});
ipcMain.on("get-user-name", (event) => {
    const userInfo = os.userInfo();
    event.returnValue = userInfo.username;
});
ipcMain.on("get-host-name", (event) => {
    event.returnValue = os.hostname();
});
ipcMain.on("get-webview-preload", (event) => {
    event.returnValue = path.join(getElectronAppBasePath(), "preload", "preload-webview.cjs");
});
ipcMain.on("get-data-dir", (event) => {
    event.returnValue = getWaveDataDir();
});
ipcMain.on("get-config-dir", (event) => {
    event.returnValue = getWaveConfigDir();
});
ipcMain.on("get-home-dir", (event) => {
    event.returnValue = app.getPath("home");
});

/**
 * Gets the value of the XDG_CURRENT_DESKTOP environment variable. If ORIGINAL_XDG_CURRENT_DESKTOP is set, it will be returned instead.
 * This corrects for a strange behavior in Electron, where it sets its own value for XDG_CURRENT_DESKTOP to improve Chromium compatibility.
 * @see https://www.electronjs.org/docs/latest/api/environment-variables#original_xdg_current_desktop
 * @returns The value of the XDG_CURRENT_DESKTOP environment variable, or ORIGINAL_XDG_CURRENT_DESKTOP if set, or undefined if neither are set.
 */
function getXdgCurrentDesktop(): string {
    if (process.env.ORIGINAL_XDG_CURRENT_DESKTOP) {
        return process.env.ORIGINAL_XDG_CURRENT_DESKTOP;
    } else if (process.env.XDG_CURRENT_DESKTOP) {
        return process.env.XDG_CURRENT_DESKTOP;
    } else {
        return undefined;
    }
}

/**
 * Calls the given callback with the value of the XDG_CURRENT_DESKTOP environment variable set to ORIGINAL_XDG_CURRENT_DESKTOP if it is set.
 * @see https://www.electronjs.org/docs/latest/api/environment-variables#original_xdg_current_desktop
 * @param callback The callback to call.
 */
function callWithOriginalXdgCurrentDesktop(callback: () => void) {
    const currXdgCurrentDesktopDefined = "XDG_CURRENT_DESKTOP" in process.env;
    const currXdgCurrentDesktop = process.env.XDG_CURRENT_DESKTOP;
    const originalXdgCurrentDesktop = getXdgCurrentDesktop();
    if (originalXdgCurrentDesktop) {
        process.env.XDG_CURRENT_DESKTOP = originalXdgCurrentDesktop;
    }
    callback();
    if (originalXdgCurrentDesktop) {
        if (currXdgCurrentDesktopDefined) {
            process.env.XDG_CURRENT_DESKTOP = currXdgCurrentDesktop;
        } else {
            delete process.env.XDG_CURRENT_DESKTOP;
        }
    }
}

/**
 * Calls the given async callback with the value of the XDG_CURRENT_DESKTOP environment variable set to ORIGINAL_XDG_CURRENT_DESKTOP if it is set.
 * @see https://www.electronjs.org/docs/latest/api/environment-variables#original_xdg_current_desktop
 * @param callback The async callback to call.
 */
async function callWithOriginalXdgCurrentDesktopAsync(callback: () => Promise<void>) {
    const currXdgCurrentDesktopDefined = "XDG_CURRENT_DESKTOP" in process.env;
    const currXdgCurrentDesktop = process.env.XDG_CURRENT_DESKTOP;
    const originalXdgCurrentDesktop = getXdgCurrentDesktop();
    if (originalXdgCurrentDesktop) {
        process.env.XDG_CURRENT_DESKTOP = originalXdgCurrentDesktop;
    }
    await callback();
    if (originalXdgCurrentDesktop) {
        if (currXdgCurrentDesktopDefined) {
            process.env.XDG_CURRENT_DESKTOP = currXdgCurrentDesktop;
        } else {
            delete process.env.XDG_CURRENT_DESKTOP;
        }
    }
}

export {
    callWithOriginalXdgCurrentDesktop,
    callWithOriginalXdgCurrentDesktopAsync,
    getElectronAppBasePath,
    getElectronAppResourcesPath,
    getElectronAppUnpackedBasePath,
    getWaveConfigDir,
    getWaveDataDir,
    getWaveSrvCwd,
    getWaveSrvPath,
    getXdgCurrentDesktop,
    isDev,
    isDevVite,
    unameArch,
    unamePlatform,
    WaveConfigHomeVarName,
    WaveDataHomeVarName,
};
