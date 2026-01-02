// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { Block, SubBlock } from "@/app/block/block";
import { Search, useSearch } from "@/app/element/search";
import { useT } from "@/app/i18n/i18n";
import { useTabModel } from "@/app/store/tab-model";
import { waveEventSubscribe } from "@/app/store/wps";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import type { TermViewModel } from "@/app/view/term/term-model";
import { atoms, getBlockComponentModel, getOverrideConfigAtom, getSettingsPrefixAtom, globalStore, WOS } from "@/store/global";
import { fireAndForget, useAtomValueSafe } from "@/util/util";
import { computeBgStyleFromMeta } from "@/util/waveutil";
import { ISearchOptions } from "@xterm/addon-search";
import clsx from "clsx";
import debug from "debug";
import * as jotai from "jotai";
import * as React from "react";
import { useDrop } from "react-dnd";
import { TermStickers } from "./termsticker";
import { TermThemeUpdater } from "./termtheme";
import { computeTheme } from "./termutil";
import { TermWrap } from "./termwrap";
import "./xterm.css";

const dlog = debug("wave:term");

interface TerminalViewProps {
    blockId: string;
    model: TermViewModel;
}

const TermMultiSessionKey_IsSession = "term:issession";
const TermMultiSessionKey_SessionIds = "term:sessionids";
const TermMultiSessionKey_ActiveSessionId = "term:activesessionid";
const TermMultiSessionKey_SessionListOpen = "term:sessionlistopen";
const TermMultiSessionKey_SessionListWidth = "term:sessionlistwidth";

function parseDraggedFileUri(uri: string): { connection: string | null; path: string } | null {
    if (!uri) {
        return null;
    }
    if (uri.startsWith("wsh://")) {
        // NOTE: We intentionally do not use `new URL()` here, because connection names can include
        // user/host/port patterns like `root@hk150:2222` which URL parsing would split into
        // username/hostname/port.
        const rest = uri.slice("wsh://".length);
        const slashIdx = rest.indexOf("/");
        if (slashIdx === -1) {
            return { connection: rest || null, path: "" };
        }
        const connection = rest.slice(0, slashIdx);
        let path = decodeURIComponent(rest.slice(slashIdx));
        if (path.startsWith("//")) {
            path = path.slice(1);
        }
        return { connection: connection || null, path };
    }
    const s3Marker = ":s3://";
    const s3Idx = uri.indexOf(s3Marker);
    if (s3Idx > 0) {
        return { connection: uri.slice(0, s3Idx), path: uri.slice(s3Idx + 1) };
    }
    return { connection: null, path: uri };
}

function getExtraSessionIds(blockData: Block | null, selfBlockId: string): string[] {
    const raw = blockData?.meta?.[TermMultiSessionKey_SessionIds];
    if (!Array.isArray(raw)) {
        return [];
    }
    return raw.filter((v) => typeof v === "string" && v && v !== selfBlockId) as string[];
}

function getActiveSessionId(blockData: Block | null, selfBlockId: string): string {
    const raw = blockData?.meta?.[TermMultiSessionKey_ActiveSessionId];
    if (typeof raw === "string" && raw) {
        return raw;
    }
    return selfBlockId;
}

function getSessionListOpen(blockData: Block | null, hasMultipleSessions: boolean, isNonMainActive: boolean): boolean {
    const raw = blockData?.meta?.[TermMultiSessionKey_SessionListOpen];
    if (typeof raw === "boolean") {
        return raw;
    }
    return hasMultipleSessions || isNonMainActive;
}

const TermResyncHandler = React.memo(({ blockId, model }: TerminalViewProps) => {
    const connStatus = jotai.useAtomValue(model.connStatus);
    const [lastConnStatus, setLastConnStatus] = React.useState<ConnStatus>(connStatus);

    React.useEffect(() => {
        if (!model.termRef.current?.hasResized) {
            return;
        }
        const isConnected = connStatus?.status == "connected";
        const wasConnected = lastConnStatus?.status == "connected";
        const curConnName = connStatus?.connection;
        const lastConnName = lastConnStatus?.connection;
        if (isConnected == wasConnected && curConnName == lastConnName) {
            return;
        }
        model.termRef.current?.resyncController("resync handler");
        setLastConnStatus(connStatus);
    }, [connStatus]);

    return null;
});

const TermSessionListItem = React.memo(
    ({
        sessionId,
        index,
        isActive,
        onSelect,
        onKill,
    }: {
        sessionId: string;
        index: number;
        isActive: boolean;
        onSelect: () => void;
        onKill: () => void;
    }) => {
        const t = useT();
        const [sessionBlock] = WOS.useWaveObjectValue<Block>(WOS.makeORef("block", sessionId));
        const [shellType, setShellType] = React.useState<string | null>(null);

        React.useEffect(() => {
            let cancelled = false;
            fireAndForget(async () => {
                const rtInfo = await RpcApi.GetRTInfoCommand(TabRpcClient, { oref: WOS.makeORef("block", sessionId) });
                if (cancelled) return;
                setShellType((rtInfo?.["shell:type"] as string) || null);
            });
            return () => {
                cancelled = true;
            };
        }, [sessionId, isActive]);

        const cwd = (sessionBlock?.meta?.["cmd:cwd"] as string) ?? "";
        const conn = sessionBlock?.meta?.connection ?? "local";
        const title = shellType || t("term.sessions.terminalWithIndex", { index: index + 1 });

        return (
            <div
                className={clsx("term-session-item", { active: isActive })}
                onClick={onSelect}
                title={cwd ? `${title}\n${cwd}` : title}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        onSelect();
                    }
                }}
            >
                <div className="term-session-item-top">
                    <div className="term-session-item-title ellipsis">
                        {title}
                        {conn !== "local" ? <span className="term-session-item-conn"> · {conn}</span> : null}
                    </div>
                    <button
                        className="term-session-item-kill"
                        title={t("term.sessions.kill")}
                        onClick={(e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            onKill();
                        }}
                    >
                        <i className="fa fa-solid fa-xmark" />
                    </button>
                </div>
                <div className="term-session-item-subtitle ellipsis">{cwd || "\u00A0"}</div>
            </div>
        );
    }
);

const TermVDomToolbarNode = ({ vdomBlockId, blockId, model }: TerminalViewProps & { vdomBlockId: string }) => {
    React.useEffect(() => {
        const unsub = waveEventSubscribe({
            eventType: "blockclose",
            scope: WOS.makeORef("block", vdomBlockId),
            handler: (event) => {
                RpcApi.SetMetaCommand(TabRpcClient, {
                    oref: WOS.makeORef("block", blockId),
                    meta: {
                        "term:mode": null,
                        "term:vdomtoolbarblockid": null,
                    },
                });
            },
        });
        return () => {
            unsub();
        };
    }, []);
    let vdomNodeModel = {
        blockId: vdomBlockId,
        isFocused: jotai.atom(false),
        focusNode: () => {},
        onClose: () => {
            if (vdomBlockId != null) {
                RpcApi.DeleteSubBlockCommand(TabRpcClient, { blockid: vdomBlockId });
            }
        },
    };
    const toolbarTarget = jotai.useAtomValue(model.vdomToolbarTarget);
    const heightStr = toolbarTarget?.height ?? "1.5em";
    return (
        <div key="vdomToolbar" className="term-toolbar" style={{ height: heightStr }}>
            <SubBlock key="vdom" nodeModel={vdomNodeModel} />
        </div>
    );
};

const TermVDomNodeSingleId = ({ vdomBlockId, blockId, model }: TerminalViewProps & { vdomBlockId: string }) => {
    React.useEffect(() => {
        const unsub = waveEventSubscribe({
            eventType: "blockclose",
            scope: WOS.makeORef("block", vdomBlockId),
            handler: (event) => {
                RpcApi.SetMetaCommand(TabRpcClient, {
                    oref: WOS.makeORef("block", blockId),
                    meta: {
                        "term:mode": null,
                        "term:vdomblockid": null,
                    },
                });
            },
        });
        return () => {
            unsub();
        };
    }, []);
    const isFocusedAtom = jotai.atom((get) => {
        return get(model.nodeModel.isFocused) && get(model.termMode) == "vdom";
    });
    let vdomNodeModel = {
        blockId: vdomBlockId,
        isFocused: isFocusedAtom,
        focusNode: () => {
            model.nodeModel.focusNode();
        },
        onClose: () => {
            if (vdomBlockId != null) {
                RpcApi.DeleteSubBlockCommand(TabRpcClient, { blockid: vdomBlockId });
            }
        },
    };
    return (
        <div key="htmlElem" className="term-htmlelem">
            <SubBlock key="vdom" nodeModel={vdomNodeModel} />
        </div>
    );
};

const TermVDomNode = ({ blockId, model }: TerminalViewProps) => {
    const vdomBlockId = jotai.useAtomValue(model.vdomBlockId);
    if (vdomBlockId == null) {
        return null;
    }
    return <TermVDomNodeSingleId key={vdomBlockId} vdomBlockId={vdomBlockId} blockId={blockId} model={model} />;
};

const TermToolbarVDomNode = ({ blockId, model }: TerminalViewProps) => {
    const vdomToolbarBlockId = jotai.useAtomValue(model.vdomToolbarBlockId);
    if (vdomToolbarBlockId == null) {
        return null;
    }
    return (
        <TermVDomToolbarNode
            key={vdomToolbarBlockId}
            vdomBlockId={vdomToolbarBlockId}
            blockId={blockId}
            model={model}
        />
    );
};

const SingleTerminalView = ({ blockId, model }: ViewComponentProps<TermViewModel>) => {
    const viewRef = React.useRef<HTMLDivElement>(null);
    const connectElemRef = React.useRef<HTMLDivElement>(null);
    const [blockData] = WOS.useWaveObjectValue<Block>(WOS.makeORef("block", blockId));
    const termSettingsAtom = getSettingsPrefixAtom("term");
    const termSettings = jotai.useAtomValue(termSettingsAtom);
    let termMode = blockData?.meta?.["term:mode"] ?? "term";
    if (termMode != "term" && termMode != "vdom") {
        termMode = "term";
    }
    const termModeRef = React.useRef(termMode);

    const tabModel = useTabModel();
    const termFontSize = jotai.useAtomValue(model.fontSizeAtom);
    const fullConfig = globalStore.get(atoms.fullConfigAtom);
    const connFontFamily = fullConfig.connections?.[blockData?.meta?.connection]?.["term:fontfamily"];
    const isFocused = jotai.useAtomValue(model.nodeModel.isFocused);
    const isMI = jotai.useAtomValue(tabModel.isTermMultiInput);
    const isBasicTerm = termMode != "vdom" && blockData?.meta?.controller != "cmd"; // needs to match isBasicTerm

    const terminalConnection = blockData?.meta?.connection ?? "local";
    const [, dropFileItemToTerm] = useDrop(
        () => ({
            accept: "FILE_ITEM",
            canDrop: (item: DraggedFile) => {
                if (!isBasicTerm) {
                    return false;
                }
                const parsed = parseDraggedFileUri(item?.uri);
                if (!parsed?.connection) {
                    return false;
                }
                return parsed.connection === terminalConnection;
            },
            drop: async (item: DraggedFile, monitor) => {
                if (monitor.didDrop()) {
                    return;
                }
                if (!isBasicTerm) {
                    return;
                }
                model.nodeModel.focusNode();
                model.giveFocus();

                let pathToPaste: string = null;
                try {
                    const fileInfo = await RpcApi.FileInfoCommand(TabRpcClient, { info: { path: item.uri } }, null);
                    pathToPaste = fileInfo?.path ?? null;
                } catch (_) {
                    // Fall back to best-effort parsing below.
                }
                if (!pathToPaste) {
                    const parsed = parseDraggedFileUri(item?.uri);
                    pathToPaste = parsed?.path ?? null;
                }
                if (!pathToPaste) {
                    return;
                }
                if (pathToPaste.startsWith("/~")) {
                    pathToPaste = pathToPaste.slice(1);
                }

                model.termRef.current?.pasteText(pathToPaste);
                model.giveFocus();
            },
        }),
        [isBasicTerm, model.termRef, terminalConnection]
    );

    React.useEffect(() => {
        if (connectElemRef.current == null) {
            return;
        }
        dropFileItemToTerm(connectElemRef.current);
    }, [dropFileItemToTerm]);

    // search
    const searchProps = useSearch({
        anchorRef: viewRef,
        viewModel: model,
        caseSensitive: false,
        wholeWord: false,
        regex: false,
    });
    const searchIsOpen = jotai.useAtomValue<boolean>(searchProps.isOpen);
    const caseSensitive = useAtomValueSafe<boolean>(searchProps.caseSensitive);
    const wholeWord = useAtomValueSafe<boolean>(searchProps.wholeWord);
    const regex = useAtomValueSafe<boolean>(searchProps.regex);
    const searchVal = jotai.useAtomValue<string>(searchProps.searchValue);
    const searchDecorations = React.useMemo(
        () => ({
            matchOverviewRuler: "#000000",
            activeMatchColorOverviewRuler: "#000000",
            activeMatchBorder: "#FF9632",
            matchBorder: "#FFFF00",
        }),
        []
    );
    const searchOpts = React.useMemo<ISearchOptions>(
        () => ({
            regex,
            wholeWord,
            caseSensitive,
            decorations: searchDecorations,
        }),
        [regex, wholeWord, caseSensitive]
    );
    const handleSearchError = React.useCallback((e: Error) => {
        console.warn("search error:", e);
    }, []);
    const executeSearch = React.useCallback(
        (searchText: string, direction: "next" | "previous") => {
            if (searchText === "") {
                model.termRef.current?.searchAddon.clearDecorations();
                return;
            }
            try {
                model.termRef.current?.searchAddon[direction === "next" ? "findNext" : "findPrevious"](
                    searchText,
                    searchOpts
                );
            } catch (e) {
                handleSearchError(e);
            }
        },
        [searchOpts, handleSearchError]
    );
    searchProps.onSearch = React.useCallback(
        (searchText: string) => executeSearch(searchText, "previous"),
        [executeSearch]
    );
    searchProps.onPrev = React.useCallback(() => executeSearch(searchVal, "previous"), [executeSearch, searchVal]);
    searchProps.onNext = React.useCallback(() => executeSearch(searchVal, "next"), [executeSearch, searchVal]);
    // Return input focus to the terminal when the search is closed
    React.useEffect(() => {
        if (!searchIsOpen) {
            model.giveFocus();
        }
    }, [searchIsOpen]);
    // rerun search when the searchOpts change
    React.useEffect(() => {
        model.termRef.current?.searchAddon.clearDecorations();
        searchProps.onSearch(searchVal);
    }, [searchOpts]);
    // end search

    React.useEffect(() => {
        const fullConfig = globalStore.get(atoms.fullConfigAtom);
        const termThemeName = globalStore.get(model.termThemeNameAtom);
        const termTransparency = globalStore.get(model.termTransparencyAtom);
        const termMacOptionIsMetaAtom = getOverrideConfigAtom(blockId, "term:macoptionismeta");
        const [termTheme, _] = computeTheme(fullConfig, termThemeName, termTransparency);
        let termScrollback = 2000;
        if (termSettings?.["term:scrollback"]) {
            termScrollback = Math.floor(termSettings["term:scrollback"]);
        }
        if (blockData?.meta?.["term:scrollback"]) {
            termScrollback = Math.floor(blockData.meta["term:scrollback"]);
        }
        if (termScrollback < 0) {
            termScrollback = 0;
        }
        if (termScrollback > 50000) {
            termScrollback = 50000;
        }
        const termAllowBPM = globalStore.get(model.termBPMAtom) ?? true;
        const termMacOptionIsMeta = globalStore.get(termMacOptionIsMetaAtom) ?? false;
        const wasFocused = model.termRef.current != null && globalStore.get(model.nodeModel.isFocused);
        const termWrap = new TermWrap(
            tabModel.tabId,
            blockId,
            connectElemRef.current,
            {
                theme: termTheme,
                fontSize: termFontSize,
                fontFamily: termSettings?.["term:fontfamily"] ?? connFontFamily ?? "Hack",
                drawBoldTextInBrightColors: false,
                fontWeight: "normal",
                fontWeightBold: "bold",
                allowTransparency: true,
                scrollback: termScrollback,
                allowProposedApi: true, // Required by @xterm/addon-search to enable search functionality and decorations
                ignoreBracketedPasteMode: !termAllowBPM,
                macOptionIsMeta: termMacOptionIsMeta,
            },
            {
                keydownHandler: model.handleTerminalKeydown.bind(model),
                useWebGl: !termSettings?.["term:disablewebgl"],
                sendDataHandler: model.sendDataToController.bind(model),
            }
        );
        (window as any).term = termWrap;
        model.termRef.current = termWrap;
        const rszObs = new ResizeObserver(() => {
            termWrap.handleResize_debounced();
        });
        rszObs.observe(connectElemRef.current);
        termWrap.onSearchResultsDidChange = (results) => {
            globalStore.set(searchProps.resultsIndex, results.resultIndex);
            globalStore.set(searchProps.resultsCount, results.resultCount);
        };
        fireAndForget(termWrap.initTerminal.bind(termWrap));
        if (wasFocused) {
            setTimeout(() => {
                model.giveFocus();
            }, 10);
        }
        return () => {
            termWrap.dispose();
            rszObs.disconnect();
        };
    }, [blockId, termSettings, termFontSize, connFontFamily]);

    React.useEffect(() => {
        if (termModeRef.current == "vdom" && termMode == "term") {
            // focus the terminal
            model.giveFocus();
        }
        termModeRef.current = termMode;
    }, [termMode]);

    React.useEffect(() => {
        if (isMI && isBasicTerm && isFocused && model.termRef.current != null) {
            model.termRef.current.multiInputCallback = (data: string) => {
                model.multiInputHandler(data);
            };
        } else {
            if (model.termRef.current != null) {
                model.termRef.current.multiInputCallback = null;
            }
        }
    }, [isMI, isBasicTerm, isFocused]);

    const scrollbarHideObserverRef = React.useRef<HTMLDivElement>(null);
    const onScrollbarShowObserver = React.useCallback(() => {
        const termViewport = viewRef.current.getElementsByClassName("xterm-viewport")[0] as HTMLDivElement;
        termViewport.style.zIndex = "var(--zindex-xterm-viewport-overlay)";
        scrollbarHideObserverRef.current.style.display = "block";
    }, []);
    const onScrollbarHideObserver = React.useCallback(() => {
        const termViewport = viewRef.current.getElementsByClassName("xterm-viewport")[0] as HTMLDivElement;
        termViewport.style.zIndex = "auto";
        scrollbarHideObserverRef.current.style.display = "none";
    }, []);

    const stickerConfig = {
        charWidth: 8,
        charHeight: 16,
        rows: model.termRef.current?.terminal.rows ?? 24,
        cols: model.termRef.current?.terminal.cols ?? 80,
        blockId: blockId,
    };

    const termBg = computeBgStyleFromMeta(blockData?.meta);

    return (
        <div className={clsx("view-term", "term-mode-" + termMode)} ref={viewRef}>
            {termBg && <div className="absolute inset-0 z-0 pointer-events-none" style={termBg} />}
            <TermResyncHandler blockId={blockId} model={model} />
            <TermThemeUpdater blockId={blockId} model={model} termRef={model.termRef} />
            <TermStickers config={stickerConfig} />
            <TermToolbarVDomNode key="vdom-toolbar" blockId={blockId} model={model} />
            <TermVDomNode key="vdom" blockId={blockId} model={model} />
            <div key="conntectElem" className="term-connectelem" ref={connectElemRef}>
                <div className="term-scrollbar-show-observer" onPointerOver={onScrollbarShowObserver} />
                <div
                    ref={scrollbarHideObserverRef}
                    className="term-scrollbar-hide-observer"
                    onPointerOver={onScrollbarHideObserver}
                />
            </div>
            <Search {...searchProps} />
        </div>
    );
};

const TerminalView = ({ blockId, model }: ViewComponentProps<TermViewModel>) => {
    const t = useT();
    const [blockData] = WOS.useWaveObjectValue<Block>(WOS.makeORef("block", blockId));
    const isSession = !!blockData?.meta?.[TermMultiSessionKey_IsSession];
    if (isSession) {
        return <SingleTerminalView blockId={blockId} model={model} />;
    }

    const extraSessionIds = React.useMemo(() => getExtraSessionIds(blockData, blockId), [blockData, blockId]);
    const activeSessionId = React.useMemo(() => getActiveSessionId(blockData, blockId), [blockData, blockId]);
    const hasMultiple = extraSessionIds.length > 0;
    const isNonMainActive = activeSessionId !== blockId;
    const listOpen = getSessionListOpen(blockData, hasMultiple, isNonMainActive);
    const shouldRenderMulti = listOpen || isNonMainActive;
    const rootRef = React.useRef<HTMLDivElement>(null);
    const savedSidebarWidth = React.useMemo(() => {
        const raw = blockData?.meta?.[TermMultiSessionKey_SessionListWidth];
        if (typeof raw === "number" && isFinite(raw) && raw > 0) {
            return raw;
        }
        return 260;
    }, [blockData]);
    const [sidebarWidth, setSidebarWidth] = React.useState(savedSidebarWidth);
    const sidebarWidthRef = React.useRef(sidebarWidth);
    const [isDraggingSidebar, setIsDraggingSidebar] = React.useState(false);

    React.useEffect(() => {
        sidebarWidthRef.current = sidebarWidth;
    }, [sidebarWidth]);

    React.useEffect(() => {
        if (!isDraggingSidebar) {
            setSidebarWidth(savedSidebarWidth);
        }
    }, [savedSidebarWidth, isDraggingSidebar]);

    const startSidebarResize = React.useCallback(
        (e: React.PointerEvent<HTMLDivElement>) => {
            if (!listOpen) {
                return;
            }
            e.preventDefault();
            e.stopPropagation();
            setIsDraggingSidebar(true);

            const minSidebarWidth = 180;
            const maxSidebarWidth = 900;
            const minMainWidth = 240;

            const startX = e.clientX;
            const startWidth = sidebarWidthRef.current;
            const rootWidth = rootRef.current?.getBoundingClientRect().width ?? null;

            const clampWidth = (width: number): number => {
                const maxFromRoot =
                    rootWidth != null ? Math.max(minSidebarWidth, Math.floor(rootWidth - minMainWidth)) : maxSidebarWidth;
                const maxWidth = Math.min(maxSidebarWidth, maxFromRoot);
                return Math.max(minSidebarWidth, Math.min(maxWidth, Math.round(width)));
            };

            const prevCursor = document.body.style.cursor;
            const prevUserSelect = document.body.style.userSelect;
            document.body.style.cursor = "col-resize";
            document.body.style.userSelect = "none";

            const onMove = (ev: PointerEvent) => {
                const dx = ev.clientX - startX;
                const nextWidth = clampWidth(startWidth - dx);
                sidebarWidthRef.current = nextWidth;
                setSidebarWidth(nextWidth);
            };
            const onUp = () => {
                window.removeEventListener("pointermove", onMove);
                window.removeEventListener("pointerup", onUp, true);
                window.removeEventListener("pointercancel", onUp, true);
                document.body.style.cursor = prevCursor;
                document.body.style.userSelect = prevUserSelect;
                setIsDraggingSidebar(false);
                void model.setTermSessionListWidth(sidebarWidthRef.current);
            };

            window.addEventListener("pointermove", onMove);
            window.addEventListener("pointerup", onUp, true);
            window.addEventListener("pointercancel", onUp, true);
        },
        [listOpen, model]
    );

    React.useEffect(() => {
        if (!shouldRenderMulti) {
            return;
        }
        const t = setTimeout(() => {
            if (activeSessionId === blockId) {
                model.giveFocus();
                return;
            }
            const bcm = getBlockComponentModel(activeSessionId);
            const vm = bcm?.viewModel as TermViewModel | undefined;
            vm?.giveFocus?.();
        }, 30);
        return () => clearTimeout(t);
    }, [activeSessionId, shouldRenderMulti, blockId]);

    if (!shouldRenderMulti) {
        return <SingleTerminalView blockId={blockId} model={model} />;
    }

    const sessionIds = [blockId, ...extraSessionIds];
    const termBg = computeBgStyleFromMeta(blockData?.meta);

    return (
        <div className="term-multi-root" ref={rootRef}>
            {termBg && <div className="absolute inset-0 z-0 pointer-events-none" style={termBg} />}
            <div className="term-multi-main">
                <div className={clsx("term-multi-session", { active: activeSessionId === blockId })}>
                    <SingleTerminalView blockId={blockId} model={model} />
                </div>
                {extraSessionIds.map((sessionId) => {
                    const isFocusedAtom = jotai.atom((get) => {
                        return get(model.nodeModel.isFocused) && activeSessionId === sessionId;
                    });
                    const sessionNodeModel = {
                        blockId: sessionId,
                        isFocused: isFocusedAtom,
                        focusNode: () => model.nodeModel.focusNode(),
                        onClose: () => {
                            void model.killTerminalSession(sessionId);
                        },
                    };
                    return (
                        <div key={sessionId} className={clsx("term-multi-session", { active: activeSessionId === sessionId })}>
                            <SubBlock nodeModel={sessionNodeModel} />
                        </div>
                    );
                })}
            </div>
            {listOpen ? (
                <div
                    className={clsx("term-multi-resizer", { dragging: isDraggingSidebar })}
                    onPointerDown={startSidebarResize}
                    role="separator"
                    aria-orientation="vertical"
                    aria-label={t("term.sessions.listTitle")}
                />
            ) : null}
            {listOpen ? (
                <div className="term-multi-sidebar" style={{ width: `${sidebarWidth}px` }}>
                    <div className="term-multi-sidebar-header">{t("term.sessions.listTitle")}</div>
                    <div className="term-multi-sidebar-list">
                        {sessionIds.map((sessionId, idx) => {
                            return (
                                <TermSessionListItem
                                    key={sessionId}
                                    sessionId={sessionId}
                                    index={idx}
                                    isActive={activeSessionId === sessionId}
                                    onSelect={() => {
                                        void model.setActiveTermSessionId(sessionId);
                                    }}
                                    onKill={() => {
                                        void model.killTerminalSession(sessionId);
                                    }}
                                />
                            );
                        })}
                    </div>
                </div>
            ) : null}
        </div>
    );
};

export { TerminalView };
