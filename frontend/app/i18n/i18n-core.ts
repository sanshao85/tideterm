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
    "common.default": "Default",
    "common.defaultWithValue": "Default ({value})",
    "common.delete": "Delete",
    "common.deleting": "Deleting...",
    "common.edit": "Edit",
    "common.loading": "Loading...",
    "common.reset": "Reset",
    "common.save": "Save",
    "common.saving": "Saving...",
    "common.openDevTools": "Open DevTools",
    "common.closeDevTools": "Close DevTools",

    // Settings
    "settings.language": "Language",
    "settings.language.description": "Changes take effect immediately.",
    "settings.language.english": "English",
    "settings.language.chinese": "中文",

    // Workspace / widgets
    "workspace.localWaveApps": "Local Apps",
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

    // App context menu
    "contextmenu.openClipboardUrl": "Open Clipboard URL ({host})",

    // Block header context menu
    "blockmenu.magnifyBlock": "Magnify Block",
    "blockmenu.unmagnifyBlock": "Un-Magnify Block",
    "blockmenu.copyBlockId": "Copy BlockId",
    "blockmenu.closeBlock": "Close Block",

    // Tab context menu
    "tabmenu.pinTab": "Pin Tab",
    "tabmenu.unpinTab": "Unpin Tab",
    "tabmenu.renameTab": "Rename Tab",
    "tabmenu.copyTabId": "Copy TabId",
    "tabmenu.backgrounds": "Backgrounds",
    "tabmenu.closeTab": "Close Tab",

    // Terminal settings menu
    "termmenu.themes": "Themes",
    "termmenu.fontSize": "Font Size",
    "termmenu.transparency": "Transparency",
    "termmenu.transparentBackground": "Transparent Background",
    "termmenu.noTransparency": "No Transparency",
    "termmenu.allowBracketedPasteMode": "Allow Bracketed Paste Mode",
    "termmenu.forceRestartController": "Force Restart Controller",
    "termmenu.clearOutputOnRestart": "Clear Output On Restart",
    "termmenu.runOnStartup": "Run On Startup",
    "termmenu.closeToolbar": "Close Toolbar",
    "termmenu.debugConnection": "Debug Connection",
    "termmenu.debugConnectionInfo": "Info",
    "termmenu.debugConnectionVerbose": "Verbose",

    // Web view settings menu
    "webviewmenu.copyUrlToClipboard": "Copy URL to Clipboard",
    "webviewmenu.setBlockHomepage": "Set Block Homepage",
    "webviewmenu.setDefaultHomepage": "Set Default Homepage",
    "webviewmenu.userAgentType": "User Agent Type",
    "webviewmenu.userAgentDefault": "Default",
    "webviewmenu.userAgentMobileIphone": "Mobile: iPhone",
    "webviewmenu.userAgentMobileAndroid": "Mobile: Android",
    "webviewmenu.hideNavigation": "Hide Navigation",
    "webviewmenu.unhideNavigation": "Un-Hide Navigation",
    "webviewmenu.setZoomFactor": "Set Zoom Factor",
    "webviewmenu.clearHistory": "Clear History",
    "webviewmenu.clearCookiesAndStorage": "Clear Cookies and Storage (All Web Widgets)",

    // Preview settings menu
    "previewmenu.goToBookmark": "Go to {label} ({path})",
    "previewmenu.copyFullPath": "Copy Full Path",
    "previewmenu.editorFontSize": "Editor Font Size",
    "previewmenu.saveFile": "Save File",
    "previewmenu.revertFile": "Revert File",
    "previewmenu.wordWrap": "Word Wrap",

    // Sysinfo settings menu
    "sysinfomenu.plotType": "Plot Type",

    // Tsunami settings menu
    "tsunamimenu.stopWaveApp": "Stop WaveApp",
    "tsunamimenu.restartWaveApp": "Restart WaveApp",
    "tsunamimenu.restartWaveAppForceRebuild": "Restart WaveApp and Force Rebuild",
    "tsunamimenu.remixWaveAppInBuilder": "Remix WaveApp in Builder",

    // WaveAI (AI panel) context menu
    "aipanelmenu.newChat": "New Chat",
    "aipanelmenu.maxOutputTokens": "Max Output Tokens",
    "aipanelmenu.configureModes": "Configure Modes",
    "aipanelmenu.hideTideTermAI": "Hide TideTerm AI",
    "aipanelmenu.tokens.1kDevTesting": "1k (Dev Testing)",
    "aipanelmenu.tokens.4k": "4k",
    "aipanelmenu.tokens.16kPro": "16k (Pro)",
    "aipanelmenu.tokens.24k": "24k",
    "aipanelmenu.tokens.64kPro": "64k (Pro)",

    // Workspace widgets context menu
    "workspace.menu.editWidgetsJson": "Edit widgets.json",

    // Builder context menu
    "buildermenu.addToContext": "Add to Context",
    "buildermenu.publishApp": "Publish App",
    "buildermenu.switchApp": "Switch App",
    "buildermenu.renameFile": "Rename File",
    "buildermenu.deleteFile": "Delete File",

    // WaveConfig
    "waveconfig.configFiles": "Config Files",
    "waveconfig.deprecated": "deprecated",
    "waveconfig.file.general": "General",
    "waveconfig.file.connections": "Connections",
    "waveconfig.file.sidebarWidgets": "Sidebar Widgets",
    "waveconfig.file.waveAiModes": "AI Modes",
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

    // MCP Servers
    "waveconfig.file.mcpServers": "MCP Servers",
    "waveconfig.mcpServers.description": "Manage MCP servers for AI tools",
    "mcp.noServers": "No MCP Servers",
    "mcp.addToGetStarted": "Add an MCP server or import from installed AI applications",
    "mcp.addServer": "Add Server",
    "mcp.editServer": "Edit Server",
    "mcp.importFromApps": "Import from Apps",
    "mcp.loading": "Loading MCP configuration...",
    "mcp.serverCount": "{count} server(s)",
    "mcp.syncAll": "Sync All",
    "mcp.syncAll.tooltip": "Sync all servers to their enabled apps",
    "mcp.import": "Import",
    "mcp.importSuccess": "Successfully imported {count} server(s)",
    "mcp.importNoServers": "No new servers to import",
    "mcp.syncSuccess": "All servers synced successfully",
    "mcp.deleteConfirm": "Are you sure you want to delete this server?",
    "mcp.serverId": "Server ID",
    "mcp.serverId.help": "Unique identifier for this server (e.g., my-mcp-server)",
    "mcp.serverName": "Display Name",
    "mcp.transportType": "Transport Type",
    "mcp.command": "Command",
    "mcp.args": "Arguments",
    "mcp.args.help": "Space-separated arguments for the command",
    "mcp.env": "Environment Variables",
    "mcp.env.help": "One variable per line in KEY=value format",
    "mcp.url": "URL",
    "mcp.description": "Description",
    "mcp.description.placeholder": "Brief description of this server",
    "mcp.enabledApps": "Enabled Apps",
    "mcp.enabledApps.help": "Select which apps should use this MCP server",
    "mcp.installedApps": "Installed Apps",
    "mcp.warning": "Important: Please clear all existing MCP servers in the AI apps first, then configure them here. Otherwise synchronization may fail.",

    // App menu (Electron)
    "menu.workspace": "Workspace",
    "menu.createWorkspace": "Create Workspace",
    "menu.newWindow": "New Window",
    "menu.newWaveAppBuilderWindow": "New App Builder Window",
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
    "menu.aboutWaveTerminal": "About TideTerm",
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
    "common.default": "默认",
    "common.defaultWithValue": "默认（{value}）",
    "common.delete": "删除",
    "common.deleting": "正在删除…",
    "common.edit": "编辑",
    "common.loading": "加载中…",
    "common.reset": "重置",
    "common.save": "保存",
    "common.saving": "正在保存…",
    "common.openDevTools": "打开开发者工具",
    "common.closeDevTools": "关闭开发者工具",

    // Settings
    "settings.language": "语言",
    "settings.language.description": "切换后立即生效，无需重启。",
    "settings.language.english": "English",
    "settings.language.chinese": "中文",

    // Workspace / widgets
    "workspace.localWaveApps": "本地应用",
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

    // App context menu
    "contextmenu.openClipboardUrl": "打开剪贴板 URL（{host}）",

    // Block header context menu
    "blockmenu.magnifyBlock": "放大块",
    "blockmenu.unmagnifyBlock": "取消放大块",
    "blockmenu.copyBlockId": "复制 BlockId",
    "blockmenu.closeBlock": "关闭块",

    // Tab context menu
    "tabmenu.pinTab": "固定标签页",
    "tabmenu.unpinTab": "取消固定标签页",
    "tabmenu.renameTab": "重命名标签页",
    "tabmenu.copyTabId": "复制 TabId",
    "tabmenu.backgrounds": "背景",
    "tabmenu.closeTab": "关闭标签页",

    // Terminal settings menu
    "termmenu.themes": "主题",
    "termmenu.fontSize": "字体大小",
    "termmenu.transparency": "透明度",
    "termmenu.transparentBackground": "透明背景",
    "termmenu.noTransparency": "无透明",
    "termmenu.allowBracketedPasteMode": "允许括号粘贴模式",
    "termmenu.forceRestartController": "强制重启控制器",
    "termmenu.clearOutputOnRestart": "重启时清空输出",
    "termmenu.runOnStartup": "启动时运行",
    "termmenu.closeToolbar": "关闭工具栏",
    "termmenu.debugConnection": "连接调试",
    "termmenu.debugConnectionInfo": "信息",
    "termmenu.debugConnectionVerbose": "详细",

    // Web view settings menu
    "webviewmenu.copyUrlToClipboard": "复制 URL 到剪贴板",
    "webviewmenu.setBlockHomepage": "设置块主页",
    "webviewmenu.setDefaultHomepage": "设置默认主页",
    "webviewmenu.userAgentType": "User Agent 类型",
    "webviewmenu.userAgentDefault": "默认",
    "webviewmenu.userAgentMobileIphone": "移动端：iPhone",
    "webviewmenu.userAgentMobileAndroid": "移动端：Android",
    "webviewmenu.hideNavigation": "隐藏导航",
    "webviewmenu.unhideNavigation": "显示导航",
    "webviewmenu.setZoomFactor": "设置缩放比例",
    "webviewmenu.clearHistory": "清除历史记录",
    "webviewmenu.clearCookiesAndStorage": "清除 Cookie 和存储（所有 Web 小组件）",

    // Preview settings menu
    "previewmenu.goToBookmark": "前往 {label}（{path}）",
    "previewmenu.copyFullPath": "复制完整路径",
    "previewmenu.editorFontSize": "编辑器字体大小",
    "previewmenu.saveFile": "保存文件",
    "previewmenu.revertFile": "还原文件",
    "previewmenu.wordWrap": "自动换行",

    // Sysinfo settings menu
    "sysinfomenu.plotType": "图表类型",

    // Tsunami settings menu
    "tsunamimenu.stopWaveApp": "停止 WaveApp",
    "tsunamimenu.restartWaveApp": "重启 WaveApp",
    "tsunamimenu.restartWaveAppForceRebuild": "重启 WaveApp 并强制重建",
    "tsunamimenu.remixWaveAppInBuilder": "在 Builder 中 Remix WaveApp",

    // WaveAI (AI panel) context menu
    "aipanelmenu.newChat": "新建对话",
    "aipanelmenu.maxOutputTokens": "最大输出 Token",
    "aipanelmenu.configureModes": "配置模式",
    "aipanelmenu.hideTideTermAI": "隐藏 TideTerm AI",
    "aipanelmenu.tokens.1kDevTesting": "1k（开发测试）",
    "aipanelmenu.tokens.4k": "4k",
    "aipanelmenu.tokens.16kPro": "16k（Pro）",
    "aipanelmenu.tokens.24k": "24k",
    "aipanelmenu.tokens.64kPro": "64k（Pro）",

    // Workspace widgets context menu
    "workspace.menu.editWidgetsJson": "编辑 widgets.json",

    // Builder context menu
    "buildermenu.addToContext": "添加到上下文",
    "buildermenu.publishApp": "发布应用",
    "buildermenu.switchApp": "切换应用",
    "buildermenu.renameFile": "重命名文件",
    "buildermenu.deleteFile": "删除文件",

    // WaveConfig
    "waveconfig.configFiles": "配置文件",
    "waveconfig.deprecated": "已弃用",
    "waveconfig.file.general": "通用",
    "waveconfig.file.connections": "连接",
    "waveconfig.file.sidebarWidgets": "侧边栏小部件",
    "waveconfig.file.waveAiModes": "AI 模式",
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

    // MCP Servers
    "waveconfig.file.mcpServers": "MCP 服务器",
    "waveconfig.mcpServers.description": "管理 AI 工具的 MCP 服务器",
    "mcp.noServers": "暂无 MCP 服务器",
    "mcp.addToGetStarted": "添加 MCP 服务器或从已安装的 AI 应用导入",
    "mcp.addServer": "添加服务器",
    "mcp.editServer": "编辑服务器",
    "mcp.importFromApps": "从应用导入",
    "mcp.loading": "正在加载 MCP 配置…",
    "mcp.serverCount": "{count} 个服务器",
    "mcp.syncAll": "同步全部",
    "mcp.syncAll.tooltip": "将所有服务器同步到启用的应用",
    "mcp.import": "导入",
    "mcp.importSuccess": "成功导入 {count} 个服务器",
    "mcp.importNoServers": "没有新服务器可导入",
    "mcp.syncSuccess": "所有服务器同步成功",
    "mcp.deleteConfirm": "确定要删除此服务器吗？",
    "mcp.serverId": "服务器 ID",
    "mcp.serverId.help": "此服务器的唯一标识符（例如：my-mcp-server）",
    "mcp.serverName": "显示名称",
    "mcp.transportType": "传输类型",
    "mcp.command": "命令",
    "mcp.args": "参数",
    "mcp.args.help": "命令参数，用空格分隔",
    "mcp.env": "环境变量",
    "mcp.env.help": "每行一个变量，格式为 KEY=value",
    "mcp.url": "URL",
    "mcp.description": "描述",
    "mcp.description.placeholder": "此服务器的简要描述",
    "mcp.enabledApps": "启用的应用",
    "mcp.enabledApps.help": "选择哪些应用应使用此 MCP 服务器",
    "mcp.installedApps": "已安装的应用",
    "mcp.warning": "重要提示：请先清空各 AI 应用中已有的 MCP 服务器配置，然后在此处进行设置，否则同步可能会失败。",

    // App menu (Electron)
    "menu.workspace": "工作区",
    "menu.createWorkspace": "新建工作区",
    "menu.newWindow": "新建窗口",
    "menu.newWaveAppBuilderWindow": "新建 App Builder 窗口",
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
    "menu.aboutWaveTerminal": "关于 TideTerm",
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
