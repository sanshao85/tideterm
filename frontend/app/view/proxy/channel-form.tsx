// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { useT } from "@/app/i18n/i18n";
import { openLink } from "@/app/store/global";
import { ChannelConfig, ChannelType, ProxyAPIKey } from "./proxy-model";

type ChannelFormProps = {
    channel: ChannelConfig | null;
    channelType: ChannelType;
    onSubmit: (channel: ChannelConfig) => void;
    onClose: () => void;
};

export const ChannelForm = React.memo(({ channel, channelType, onSubmit, onClose }: ChannelFormProps) => {
    const t = useT();
    const isEditing = channel !== null;
    const maxccBaseUrl = "https://www." + "maxcc.shop";
    const maxccDisplayUrl = "www." + "maxcc.shop";
    const codexCliWebsiteUrl = "https://www." + "codex-cli.top";
    const codexCliDisplayUrl = "www." + "codex-cli.top";
    const codexCliApiBaseUrl = "https://api." + "codex-cli.top/v1";
    const ccCxWebsiteUrl = "https://www." + "claude-code.top";
    const ccCxDisplayUrl = "www." + "claude-code.top";
    const ccCxTooltip = "www.claude-code.top 企业专用，官转max20，cx，gemini，可开票";
    const ccCxBaseUrlByServiceType = {
        claude: "https://api." + "claudecode.net.cn/api/claudecode",
        openai: "https://api." + "claudecode.net.cn/api/codex/backend-api/codex",
        gemini: "https://api." + "claudecode.net.cn/api/gemini",
    } as const;

    const defaultServiceTypeForChannelType = (tab: ChannelType) => {
        switch (tab) {
            case "responses":
                return "openai";
            case "gemini":
                return "gemini";
            case "messages":
            default:
                return "claude";
        }
    };

    const [formData, setFormData] = React.useState<ChannelConfig>({
        id: channel?.id || "",
        name: channel?.name || "",
        serviceType: channel?.serviceType || defaultServiceTypeForChannelType(channelType),
        baseUrl: channel?.baseUrl || "",
        baseUrls: channel?.baseUrls || [],
        apiKeys: channel?.apiKeys || [],
        authType: channel?.authType || "",
        priority: channel?.priority || 0,
        status: channel?.status || "active",
        promotionUntil: channel?.promotionUntil,
        modelMapping: channel?.modelMapping || {},
        lowQuality: channel?.lowQuality || false,
        description: channel?.description || "",
    });

    const [apiKeyInput, setApiKeyInput] = React.useState("");
    const [baseUrlInput, setBaseUrlInput] = React.useState("");
    const [mappingKey, setMappingKey] = React.useState("");
    const [mappingValue, setMappingValue] = React.useState("");

    type QuickPickServiceType = "claude" | "openai" | "gemini";
    type QuickPickConfig = {
        id: string;
        label: string;
        websiteUrl: string;
        displayUrl: string;
        tooltip: string;
        baseUrlByServiceType: Partial<Record<QuickPickServiceType, string>>;
    };

    const quickPicks: QuickPickConfig[] = [
        {
            id: "maxcc",
            label: "maxcc",
            websiteUrl: maxccBaseUrl,
            displayUrl: maxccDisplayUrl,
            tooltip: "www." + "maxcc.shop 纯官转pro，max20，满血不降智，原生体验！",
            baseUrlByServiceType: {
                claude: maxccBaseUrl,
            },
        },
        {
            id: "codex-cli",
            label: "codex-cli",
            websiteUrl: codexCliWebsiteUrl,
            displayUrl: codexCliDisplayUrl,
            tooltip: "www.codex-cli.top 专业codex通道，性价比超高！",
            baseUrlByServiceType: {
                openai: codexCliApiBaseUrl,
            },
        },
        {
            id: "cc-cx",
            label: "cc-cx",
            websiteUrl: ccCxWebsiteUrl,
            displayUrl: ccCxDisplayUrl,
            tooltip: ccCxTooltip,
            baseUrlByServiceType: { ...ccCxBaseUrlByServiceType },
        },
    ];

    const getQuickPickBaseUrl = (quickPick: QuickPickConfig, serviceType: string) => {
        const typedServiceType = serviceType as QuickPickServiceType;
        return quickPick.baseUrlByServiceType[typedServiceType] || "";
    };

    const isQuickPickEnabled = (quickPick: QuickPickConfig) => getQuickPickBaseUrl(quickPick, formData.serviceType) !== "";

    const authTypeOptions = React.useMemo(() => {
        const opts: Array<{ value: string; label: string }> = [];

        switch (formData.serviceType) {
            case "gemini":
                opts.push({ value: "x-goog-api-key", label: "x-goog-api-key" });
                break;
            case "openai":
                opts.push({ value: "", label: t("proxy.channel.authTypeDefault") });
                opts.push({ value: "bearer", label: t("proxy.channel.authTypeBearer") });
                opts.push({ value: "x-api-key", label: "x-api-key" });
                opts.push({ value: "both", label: t("proxy.channel.authTypeBoth") });
                break;
            case "claude":
            default:
                opts.push({ value: "", label: t("proxy.channel.authTypeDefault") });
                opts.push({ value: "both", label: t("proxy.channel.authTypeBoth") });
                opts.push({ value: "x-api-key", label: "x-api-key" });
                opts.push({ value: "bearer", label: t("proxy.channel.authTypeBearer") });
                break;
        }

        return opts;
    }, [formData.serviceType, t]);

    React.useEffect(() => {
        const allowed = new Set(authTypeOptions.map((o) => o.value));
        const current = formData.authType || "";
        if (!allowed.has(current)) {
            const fallback = authTypeOptions[0]?.value ?? "";
            setFormData((prev) => ({ ...prev, authType: fallback }));
        }
    }, [authTypeOptions, formData.authType]);

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => {
        const { name, value, type } = e.target;
        if (type === "checkbox") {
            setFormData((prev) => ({
                ...prev,
                [name]: (e.target as HTMLInputElement).checked,
            }));
        } else if (type === "number") {
            setFormData((prev) => ({
                ...prev,
                [name]: parseInt(value) || 0,
            }));
        } else {
            setFormData((prev) => ({
                ...prev,
                [name]: value,
            }));
        }
    };

    const handleAddApiKey = () => {
        const trimmed = apiKeyInput.trim();
        if (trimmed) {
            setFormData((prev) => ({
                ...prev,
                apiKeys: [...prev.apiKeys, { key: trimmed, enabled: true } satisfies ProxyAPIKey],
            }));
            setApiKeyInput("");
        }
    };

    const handleApiKeyInputKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === "Enter") {
            e.preventDefault();
            handleAddApiKey();
        }
    };

    const handleToggleApiKey = (index: number) => {
        setFormData((prev) => ({
            ...prev,
            apiKeys: prev.apiKeys.map((k, i) => (i === index ? { ...k, enabled: !k.enabled } : k)),
        }));
    };

    const handleRemoveApiKey = (index: number) => {
        setFormData((prev) => ({
            ...prev,
            apiKeys: prev.apiKeys.filter((_, i) => i !== index),
        }));
    };

    const handleAddBaseUrl = () => {
        if (baseUrlInput.trim()) {
            setFormData((prev) => ({
                ...prev,
                baseUrls: [...(prev.baseUrls || []), baseUrlInput.trim()],
            }));
            setBaseUrlInput("");
        }
    };

    const handleRemoveBaseUrl = (index: number) => {
        setFormData((prev) => ({
            ...prev,
            baseUrls: prev.baseUrls?.filter((_, i) => i !== index) || [],
        }));
    };

    const handleQuickPickClick = (quickPick: QuickPickConfig) => {
        const targetBaseUrl = getQuickPickBaseUrl(quickPick, formData.serviceType);
        if (!targetBaseUrl) return;
        setFormData((prev) => ({
            ...prev,
            baseUrl: targetBaseUrl,
        }));
    };

    const handleQuickPickContextMenu = (e: React.MouseEvent<HTMLButtonElement>, quickPick: QuickPickConfig) => {
        if (!isQuickPickEnabled(quickPick)) return;
        e.preventDefault();
        openLink(quickPick.websiteUrl);
    };

    const handleAddMapping = () => {
        if (mappingKey.trim() && mappingValue.trim()) {
            setFormData((prev) => ({
                ...prev,
                modelMapping: {
                    ...prev.modelMapping,
                    [mappingKey.trim()]: mappingValue.trim(),
                },
            }));
            setMappingKey("");
            setMappingValue("");
        }
    };

    const handleRemoveMapping = (key: string) => {
        setFormData((prev) => {
            const newMapping = { ...prev.modelMapping };
            delete newMapping[key];
            return {
                ...prev,
                modelMapping: newMapping,
            };
        });
    };

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        // Avoid dropping a typed key when the user presses Enter (default form submit)
        // without clicking "+" first.
        const trimmedKey = apiKeyInput.trim();
        if (trimmedKey && !formData.apiKeys.some((k) => k.key === trimmedKey)) {
            onSubmit({
                ...formData,
                apiKeys: [...formData.apiKeys, { key: trimmedKey, enabled: true } satisfies ProxyAPIKey],
            });
            return;
        }
        onSubmit(formData);
    };

    const maskApiKey = (key: string) => {
        const trimmed = key.trim();
        if (!trimmed || trimmed.length < 8) return "****";
        return trimmed.substring(0, 8) + "..." + trimmed.substring(trimmed.length - 4);
    };

    return (
        <div className="channel-form-overlay" onClick={onClose}>
            <div className="channel-form-modal" onClick={(e) => e.stopPropagation()}>
                <div className="form-header">
                    <h3>{isEditing ? t("proxy.editChannel") : t("proxy.addChannel")}</h3>
                    <button className="close-btn" onClick={onClose}>
                        <i className="fa fa-times" />
                    </button>
                </div>

                <form onSubmit={handleSubmit}>
                    <div className="form-body">
                        {/* Basic Info */}
                        <div className="form-section">
                            <h4>{t("proxy.form.basicInfo")}</h4>
                            <div className="quick-picks">
                                <span className="quick-picks-label">{t("proxy.form.quickPick")}</span>
                                <div className="quick-picks-tags">
                                    {quickPicks.map((quickPick) => {
                                        const enabled = isQuickPickEnabled(quickPick);
                                        return (
                                            <button
                                                key={quickPick.id}
                                                type="button"
                                                className="quick-pick-tag"
                                                data-url={quickPick.tooltip}
                                                onClick={() => handleQuickPickClick(quickPick)}
                                                onContextMenu={(e) => handleQuickPickContextMenu(e, quickPick)}
                                                disabled={!enabled}
                                                aria-disabled={!enabled}
                                            >
                                                {quickPick.label}
                                            </button>
                                        );
                                    })}
                                </div>
                            </div>
                            <div className="form-row">
                                <label>
                                    <span>{t("proxy.channel.name")}</span>
                                    <input
                                        type="text"
                                        name="name"
                                        value={formData.name}
                                        onChange={handleInputChange}
                                        placeholder={t("proxy.form.channelNamePlaceholder")}
                                        required
                                    />
                                </label>
                                <label>
                                    <span>{t("proxy.channel.serviceType")}</span>
                                    <select name="serviceType" value={formData.serviceType} onChange={handleInputChange}>
                                        <option value="claude">Claude</option>
                                        <option value="openai">OpenAI</option>
                                        <option value="gemini">Gemini</option>
                                    </select>
                                </label>
                            </div>

                            <div className="form-row">
                                <label>
                                    <span>{t("proxy.channel.baseUrl")}</span>
                                    <input
                                        type="url"
                                        name="baseUrl"
                                        value={formData.baseUrl}
                                        onChange={handleInputChange}
                                        placeholder="https://api.anthropic.com"
                                        required
                                    />
                                </label>
                            </div>

                            <div className="form-row">
                                <label>
                                    <span>{t("proxy.channel.priority")}</span>
                                    <input
                                        type="number"
                                        name="priority"
                                        value={formData.priority}
                                        onChange={handleInputChange}
                                        min="0"
                                    />
                                </label>
                                <label>
                                    <span>{t("proxy.channel.status")}</span>
                                    <select name="status" value={formData.status} onChange={handleInputChange}>
                                        <option value="active">{t("proxy.channel.active")}</option>
                                        <option value="suspended">{t("proxy.channel.suspended")}</option>
                                        <option value="disabled">{t("proxy.channel.disabled")}</option>
                                    </select>
                                </label>
                            </div>

                            <div className="form-row">
                                <label>
                                    <span>{t("proxy.channel.authType")}</span>
                                    <select name="authType" value={formData.authType || ""} onChange={handleInputChange}>
                                        {authTypeOptions.map((opt) => (
                                            <option key={opt.value || "default"} value={opt.value}>
                                                {opt.label}
                                            </option>
                                        ))}
                                    </select>
                                </label>
                            </div>
                        </div>

                        {/* API Keys */}
                        <div className="form-section">
                            <h4>{t("proxy.channel.apiKeys")}</h4>
                            <div className="list-input">
                                <input
                                    type="password"
                                    value={apiKeyInput}
                                    onChange={(e) => setApiKeyInput(e.target.value)}
                                    onKeyDown={handleApiKeyInputKeyDown}
                                    placeholder={t("proxy.form.apiKeyPlaceholder")}
                                />
                                <button type="button" className="add-btn" onClick={handleAddApiKey}>
                                    <i className="fa fa-plus" />
                                </button>
                            </div>
                            <div className="list-items">
                                {formData.apiKeys.map((k, index) => (
                                    <div key={index} className="list-item">
                                        <span className={!k.enabled ? "text-muted" : undefined}>{maskApiKey(k.key)}</span>
                                        <button
                                            type="button"
                                            onClick={() => handleToggleApiKey(index)}
                                            title={k.enabled ? t("proxy.form.apiKeyDisable") : t("proxy.form.apiKeyEnable")}
                                        >
                                            <i className={k.enabled ? "fa fa-pause" : "fa fa-play"} />
                                        </button>
                                        <button type="button" onClick={() => handleRemoveApiKey(index)}>
                                            <i className="fa fa-times" />
                                        </button>
                                    </div>
                                ))}
                            </div>
                        </div>

                        {/* Backup URLs */}
                        <div className="form-section">
                            <h4>
                                {t("proxy.channel.backupUrls")} {t("proxy.form.optional")}
                            </h4>
                            <div className="list-input">
                                <input
                                    type="url"
                                    value={baseUrlInput}
                                    onChange={(e) => setBaseUrlInput(e.target.value)}
                                    placeholder="https://backup-api.example.com"
                                />
                                <button type="button" className="add-btn" onClick={handleAddBaseUrl}>
                                    <i className="fa fa-plus" />
                                </button>
                            </div>
                            <div className="list-items">
                                {formData.baseUrls?.map((url, index) => (
                                    <div key={index} className="list-item">
                                        <span>{url}</span>
                                        <button type="button" onClick={() => handleRemoveBaseUrl(index)}>
                                            <i className="fa fa-times" />
                                        </button>
                                    </div>
                                ))}
                            </div>
                        </div>

                        {/* Model Mapping */}
                        <div className="form-section">
                            <h4>
                                {t("proxy.channel.modelMapping")} {t("proxy.form.optional")}
                            </h4>
                            <div className="mapping-input">
                                <input
                                    type="text"
                                    value={mappingKey}
                                    onChange={(e) => setMappingKey(e.target.value)}
                                    placeholder={t("proxy.form.sourceModel")}
                                />
                                <span className="arrow">→</span>
                                <input
                                    type="text"
                                    value={mappingValue}
                                    onChange={(e) => setMappingValue(e.target.value)}
                                    placeholder={t("proxy.form.targetModel")}
                                />
                                <button type="button" className="add-btn" onClick={handleAddMapping}>
                                    <i className="fa fa-plus" />
                                </button>
                            </div>
                            <div className="list-items">
                                {Object.entries(formData.modelMapping || {}).map(([key, value]) => (
                                    <div key={key} className="list-item mapping-item">
                                        <span>
                                            {key} → {value}
                                        </span>
                                        <button type="button" onClick={() => handleRemoveMapping(key)}>
                                            <i className="fa fa-times" />
                                        </button>
                                    </div>
                                ))}
                            </div>
                        </div>

                        {/* Additional Options */}
                        <div className="form-section">
                            <h4>{t("proxy.form.advanced")}</h4>
                            <div className="form-row">
                                <label className="checkbox-label">
                                    <input
                                        type="checkbox"
                                        name="lowQuality"
                                        checked={formData.lowQuality}
                                        onChange={handleInputChange}
                                    />
                                    <span>{t("proxy.channel.lowQuality")}</span>
                                </label>
                            </div>
                            <div className="form-row">
                                <label>
                                    <span>{t("proxy.channel.description")}</span>
                                    <textarea
                                        name="description"
                                        value={formData.description}
                                        onChange={handleInputChange}
                                        placeholder={t("proxy.form.descriptionPlaceholder")}
                                        rows={3}
                                    />
                                </label>
                            </div>
                        </div>
                    </div>

                    <div className="form-footer">
                        <button type="button" className="proxy-btn btn-secondary" onClick={onClose}>
                            {t("common.cancel")}
                        </button>
                        <button type="submit" className="proxy-btn btn-primary">
                            {isEditing ? t("proxy.form.saveChanges") : t("proxy.addChannel")}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
});
