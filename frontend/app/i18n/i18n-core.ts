// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

export type AppLanguage = "en" | "zh-CN";

export const DefaultAppLanguage: AppLanguage = "en";

export function normalizeAppLanguage(lang?: string | null): AppLanguage {
    if (!lang) return DefaultAppLanguage;
    const normalized = String(lang).trim();
    if (!normalized) return DefaultAppLanguage;

    const lower = normalized.toLowerCase();
    if (lower === "zh-cn" || lower === "zh" || lower.startsWith("zh-")) {
        return "zh-CN";
    }
    return "en";
}

export function getAppLanguageFromSettings(settings?: SettingsType | null): AppLanguage {
    return normalizeAppLanguage(settings?.["app:language"]);
}

const en = {
    // Common
    "common.adding": "Adding...",
    "common.cancel": "Cancel",
    "common.close": "Close",
    "common.delete": "Delete",
    "common.deleting": "Deleting...",
    "common.loading": "Loading...",
    "common.save": "Save",
    "common.saving": "Saving...",

    // Settings
    "settings.language": "Language",
    "settings.language.description": "Changes take effect immediately.",
    "settings.language.english": "English",
    "settings.language.chinese": "中文",

    // Workspace / widgets
    "workspace.localWaveApps": "Local WaveApps",
    "workspace.settingsAndHelp": "Settings & Help",
    "workspace.appsLabel": "apps",
    "workspace.settingsLabel": "settings",
    "workspace.menu.settings": "Settings",
    "workspace.menu.tips": "Tips",
    "workspace.menu.secrets": "Secrets",
    "workspace.menu.help": "Help",

    // File / directory context menu
    "filemenu.newFile": "New File",
    "filemenu.newFolder": "New Folder",
    "filemenu.rename": "Rename",
    "filemenu.copyFileName": "Copy File Name",
    "filemenu.copyFullFileName": "Copy Full File Name",
    "filemenu.copyFileNameShellQuoted": "Copy File Name (Shell Quoted)",
    "filemenu.copyFullFileNameShellQuoted": "Copy Full File Name (Shell Quoted)",
    "filemenu.openFileInDefaultApplication": "Open File in Default Application",
    "filemenu.revealInFinder": "Reveal in Finder",
    "filemenu.revealInExplorer": "Reveal in Explorer",
    "filemenu.revealInFileManager": "Reveal in File Manager",
    "filemenu.downloadFile": "Download File",
    "filemenu.openPreviewInNewBlock": "Open Preview in New Block",
    "filemenu.openTerminalInNewBlock": "Open Terminal in New Block",

    // WaveConfig
    "waveconfig.configFiles": "Config Files",
    "waveconfig.deprecated": "deprecated",
    "waveconfig.file.general": "General",
    "waveconfig.file.connections": "Connections",
    "waveconfig.file.sidebarWidgets": "Sidebar Widgets",
    "waveconfig.file.waveAiModes": "Wave AI Modes",
    "waveconfig.file.tabBackgrounds": "Tab Backgrounds",
    "waveconfig.file.secrets": "Secrets",
    "waveconfig.file.presets": "Presets",
    "waveconfig.file.aiPresets": "AI Presets",
    "waveconfig.unsavedChanges": "Unsaved changes",
    "waveconfig.visual": "Visual",
    "waveconfig.rawJson": "Raw JSON",
    "waveconfig.viewDocumentation": "View documentation",
    "waveconfig.saveWithShortcut": "Save ({shortcut})",
    "waveconfig.connections.description.sshHosts": "SSH hosts",
    "waveconfig.connections.description.sshHostsAndWslDistros": "SSH hosts and WSL distros",
    "waveconfig.waveAiModes.description": "Local models and BYOK",

    // Secrets
    "secrets.noSecrets": "No Secrets",
    "secrets.addToGetStarted": "Add a secret to get started",
    "secrets.addNewSecret": "Add New Secret",
    "secrets.cliAccess": "CLI Access",
    "secrets.addNewSecret.title": "Add New Secret",
    "secrets.secretName": "Secret Name",
    "secrets.secretName.help":
        "Must start with a letter and contain only letters, numbers, and underscores.",
    "secrets.secretName.placeholder": "MY_SECRET_NAME",
    "secrets.secretValue": "Secret Value",
    "secrets.secretValue.placeholder": "Enter secret value...",
    "secrets.secretValue.placeholderNew": "Enter new secret value...",
    "secrets.addSecret": "Add Secret",
    "secrets.currentValueNotShown":
        "The current secret value is not shown by default for security purposes.",
    "secrets.show": "Show",
    "secrets.showSecret": "Show Secret",
    "secrets.loadingSecrets": "Loading secrets...",
    "secrets.deleteThisSecret": "Delete this secret",

    // App menu (Electron)
    "menu.workspace": "Workspace",
    "menu.createWorkspace": "Create Workspace",
    "menu.newWindow": "New Window",
    "menu.newWaveAppBuilderWindow": "New WaveApp Builder Window",
    "menu.closeWindow": "Close",
    "menu.reloadWindow": "Reload Window",
    "menu.reloadTab": "Reload Tab",
    "menu.relaunchAllWindows": "Relaunch All Windows",
    "menu.clearTabCache": "Clear Tab Cache",
    "menu.toggleDevTools": "Toggle DevTools",
    "menu.resetZoom": "Reset Zoom",
    "menu.zoomIn": "Zoom In",
    "menu.zoomOut": "Zoom Out",
    "menu.launchOnFullScreen": "Launch On Full Screen",
    "menu.on": "On",
    "menu.off": "Off",
    "menu.aboutWaveTerminal": "About Wave Terminal",
    "menu.checkForUpdates": "Check for Updates",
    "menu.undo": "Undo",
    "menu.redo": "Redo",
    "menu.cut": "Cut",
    "menu.copy": "Copy",
    "menu.paste": "Paste",
    "menu.pasteAndMatchStyle": "Paste and Match Style",
    "menu.delete": "Delete",
    "menu.selectAll": "Select All",
    "menu.services": "Services",
    "menu.hide": "Hide",
    "menu.hideOthers": "Hide Others",
    "menu.quit": "Quit",
    "menu.file": "File",
    "menu.edit": "Edit",
    "menu.view": "View",
    "menu.window": "Window",
    "menu.windowTitle": "Window Title",
    "menu.showWindow": "Show Window",
    "menu.renameWindow": "Rename Window…",
    "menu.restoreAutoWindowTitle": "Use Automatic Title",
    "menu.fullscreen": "Full Screen",

    // Window title
    "windowtitle.rename.title": "Rename Window",
    "windowtitle.rename.placeholder": "Enter window name…",
} as const;

export type I18nKey = keyof typeof en;

const zhCN: Record<I18nKey, string> = {
    // Common
    "common.adding": "正在添加…",
    "common.cancel": "取消",
    "common.close": "关闭",
    "common.delete": "删除",
    "common.deleting": "正在删除…",
    "common.loading": "加载中…",
    "common.save": "保存",
    "common.saving": "正在保存…",

    // Settings
    "settings.language": "语言",
    "settings.language.description": "切换后立即生效，无需重启。",
    "settings.language.english": "English",
    "settings.language.chinese": "中文",

    // Workspace / widgets
    "workspace.localWaveApps": "本地 WaveApps",
    "workspace.settingsAndHelp": "设置与帮助",
    "workspace.appsLabel": "应用",
    "workspace.settingsLabel": "设置",
    "workspace.menu.settings": "设置",
    "workspace.menu.tips": "提示",
    "workspace.menu.secrets": "密钥",
    "workspace.menu.help": "帮助",

    // File / directory context menu
    "filemenu.newFile": "新建文件",
    "filemenu.newFolder": "新建文件夹",
    "filemenu.rename": "重命名",
    "filemenu.copyFileName": "复制文件名",
    "filemenu.copyFullFileName": "复制完整路径",
    "filemenu.copyFileNameShellQuoted": "复制文件名（Shell 引号）",
    "filemenu.copyFullFileNameShellQuoted": "复制完整路径（Shell 引号）",
    "filemenu.openFileInDefaultApplication": "用默认应用打开文件",
    "filemenu.revealInFinder": "在访达中显示",
    "filemenu.revealInExplorer": "在资源管理器中显示",
    "filemenu.revealInFileManager": "在文件管理器中显示",
    "filemenu.downloadFile": "下载文件",
    "filemenu.openPreviewInNewBlock": "在新块中打开预览",
    "filemenu.openTerminalInNewBlock": "在新块中打开终端",

    // WaveConfig
    "waveconfig.configFiles": "配置文件",
    "waveconfig.deprecated": "已弃用",
    "waveconfig.file.general": "通用",
    "waveconfig.file.connections": "连接",
    "waveconfig.file.sidebarWidgets": "侧边栏小部件",
    "waveconfig.file.waveAiModes": "Wave AI 模式",
    "waveconfig.file.tabBackgrounds": "标签页背景",
    "waveconfig.file.secrets": "密钥",
    "waveconfig.file.presets": "预设",
    "waveconfig.file.aiPresets": "AI 预设",
    "waveconfig.unsavedChanges": "未保存的更改",
    "waveconfig.visual": "可视化",
    "waveconfig.rawJson": "原始 JSON",
    "waveconfig.viewDocumentation": "查看文档",
    "waveconfig.saveWithShortcut": "保存（{shortcut}）",
    "waveconfig.connections.description.sshHosts": "SSH 主机",
    "waveconfig.connections.description.sshHostsAndWslDistros": "SSH 主机和 WSL 发行版",
    "waveconfig.waveAiModes.description": "本地模型和 BYOK",

    // Secrets
    "secrets.noSecrets": "暂无密钥",
    "secrets.addToGetStarted": "添加一个密钥以开始使用",
    "secrets.addNewSecret": "添加新密钥",
    "secrets.cliAccess": "命令行访问",
    "secrets.addNewSecret.title": "添加新密钥",
    "secrets.secretName": "密钥名称",
    "secrets.secretName.help": "必须以字母开头，只能包含字母、数字和下划线。",
    "secrets.secretName.placeholder": "MY_SECRET_NAME",
    "secrets.secretValue": "密钥内容",
    "secrets.secretValue.placeholder": "输入密钥内容…",
    "secrets.secretValue.placeholderNew": "输入新的密钥内容…",
    "secrets.addSecret": "添加密钥",
    "secrets.currentValueNotShown": "出于安全考虑，默认不显示当前密钥内容。",
    "secrets.show": "显示",
    "secrets.showSecret": "显示密钥",
    "secrets.loadingSecrets": "正在加载密钥…",
    "secrets.deleteThisSecret": "删除此密钥",

    // App menu (Electron)
    "menu.workspace": "工作区",
    "menu.createWorkspace": "新建工作区",
    "menu.newWindow": "新建窗口",
    "menu.newWaveAppBuilderWindow": "新建 WaveApp Builder 窗口",
    "menu.closeWindow": "关闭",
    "menu.reloadWindow": "重新加载窗口",
    "menu.reloadTab": "重新加载标签页",
    "menu.relaunchAllWindows": "重新启动所有窗口",
    "menu.clearTabCache": "清除标签页缓存",
    "menu.toggleDevTools": "切换开发者工具",
    "menu.resetZoom": "重置缩放",
    "menu.zoomIn": "放大",
    "menu.zoomOut": "缩小",
    "menu.launchOnFullScreen": "启动时全屏",
    "menu.on": "开",
    "menu.off": "关",
    "menu.aboutWaveTerminal": "关于 Wave Terminal",
    "menu.checkForUpdates": "检查更新",
    "menu.undo": "撤销",
    "menu.redo": "重做",
    "menu.cut": "剪切",
    "menu.copy": "复制",
    "menu.paste": "粘贴",
    "menu.pasteAndMatchStyle": "粘贴并匹配样式",
    "menu.delete": "删除",
    "menu.selectAll": "全选",
    "menu.services": "服务",
    "menu.hide": "隐藏",
    "menu.hideOthers": "隐藏其他",
    "menu.quit": "退出",
    "menu.file": "文件",
    "menu.edit": "编辑",
    "menu.view": "视图",
    "menu.window": "窗口",
    "menu.windowTitle": "窗口标题",
    "menu.showWindow": "显示窗口",
    "menu.renameWindow": "重命名窗口…",
    "menu.restoreAutoWindowTitle": "恢复自动标题",
    "menu.fullscreen": "全屏",

    // Window title
    "windowtitle.rename.title": "重命名窗口",
    "windowtitle.rename.placeholder": "输入窗口名称…",
};

function formatMessage(template: string, vars?: Record<string, string | number>): string {
    if (!vars) return template;
    return template.replace(/\{(\w+)\}/g, (_, k: string) => {
        const v = vars[k];
        return v == null ? `{${k}}` : String(v);
    });
}

export function t(lang: AppLanguage, key: I18nKey, vars?: Record<string, string | number>): string {
    const message = (lang === "zh-CN" ? zhCN[key] : en[key]) ?? en[key] ?? key;
    return formatMessage(message, vars);
}
