import type { Config } from "@docusaurus/types";
import rehypeHighlight from "rehype-highlight";

const baseUrl = process.env.EMBEDDED ? "/docsite/" : "/tideterm/";

const config: Config = {
    title: "TideTerm Documentation",
    tagline: "Level Up Your Terminal With Graphical Blocks",
    favicon: "img/logo/tideterm.png",

    // Set the production url of your site here
    url: "https://sanshao85.github.io",
    // Set the /<baseUrl>/ pathname under which your site is served
    // For GitHub pages deployment, it is often '/<projectName>/'
    baseUrl,

    // GitHub pages deployment config.
    // If you aren't using GitHub pages, you don't need these.
    organizationName: "sanshao85", // Usually your GitHub org/user name.
    projectName: "tideterm", // Usually your repo name.
    deploymentBranch: "main",

    onBrokenAnchors: "ignore",
    onBrokenLinks: "throw",
    onBrokenMarkdownLinks: "warn",
    trailingSlash: false,

    // Even if you don't use internationalization, you can use this field to set
    // useful metadata like html lang. For example, if your site is Chinese, you
    // may want to replace "en" with "zh-Hans".
    i18n: {
        defaultLocale: "en",
        locales: ["en"],
    },
    plugins: [
        [
            "content-docs",
            {
                path: "docs",
                routeBasePath: "/",
                exclude: ["features/**"],
                editUrl: !process.env.EMBEDDED ? "https://github.com/sanshao85/tideterm/edit/main/docs/" : undefined,
                rehypePlugins: [rehypeHighlight],
            } as import("@docusaurus/plugin-content-docs").Options,
        ],
        "ideal-image",
        [
            "@docusaurus/plugin-sitemap",
            {
                changefreq: "daily",
                filename: "sitemap.xml",
            },
        ],
        "docusaurus-plugin-sass",
        "@docusaurus/plugin-svgr",
    ].filter((v) => v),
    themes: [
        ["classic", { customCss: "src/css/custom.scss" }],
    ].filter((v) => v),
    themeConfig: {
        docs: {
            sidebar: {
                hideable: false,
                autoCollapseCategories: false,
            },
        },
        colorMode: {
            defaultMode: "light",
            disableSwitch: false,
            respectPrefersColorScheme: true,
        },
        navbar: {
            logo: {
                src: "img/logo/tideterm.png",
                srcDark: "img/logo/tideterm.png",
                href: "https://github.com/sanshao85/tideterm",
            },
            hideOnScroll: true,
            items: [
                {
                    type: "doc",
                    position: "left",
                    docId: "index",
                    label: "Docs",
                },
                !process.env.EMBEDDED
                    ? [
                          {
                              position: "right",
                              href: "https://github.com/sanshao85/tideterm/releases",
                              label: "Releases",
                          },
                          {
                              href: "https://github.com/sanshao85/tideterm",
                              position: "right",
                              className: "header-link-custom custom-icon-github",
                              "aria-label": "GitHub repository",
                          },
                      ]
                    : [],
            ].flat(),
        },
        metadata: [
            {
                name: "keywords",
                content:
                    "terminal, developer, development, command, line, tideterm, linux, macos, windows, connection, ssh, cli, documentation, docs, ai, graphical, blocks, widgets, remote, open, source, open-source, go, golang, react, typescript, javascript",
            },
            {
                name: "og:type",
                content: "website",
            },
            {
                name: "og:site_name",
                content: "TideTerm Documentation",
            },
            {
                name: "application-name",
                content: "TideTerm Documentation",
            },
            {
                name: "apple-mobile-web-app-title",
                content: "TideTerm Documentation",
            },
        ],
        footer: {
            copyright: `Copyright © ${new Date().getFullYear()} sanshao85. Built with Docusaurus.`,
        },
    },
    headTags: [
        {
            tagName: "link",
            attributes: {
                rel: "preload",
                as: "font",
                type: "font/woff2",
                "data-next-font": "size-adjust",
                href: `${baseUrl}fontawesome/webfonts/fa-sharp-regular-400.woff2`,
            },
        },
        {
            tagName: "link",
            attributes: {
                rel: "preload",
                as: "font",
                type: "font/woff2",
                "data-next-font": "size-adjust",
                href: `${baseUrl}fontawesome/webfonts/fa-sharp-solid-900.woff2`,
            },
        },
        {
            tagName: "link",
            attributes: {
                rel: "sitemap",
                type: "application/xml",
                title: "Sitemap",
                href: `${baseUrl}sitemap.xml`,
            },
        },
    ].filter((v) => v),
    stylesheets: [
        `${baseUrl}fontawesome/css/fontawesome.min.css`,
        `${baseUrl}fontawesome/css/sharp-regular.min.css`,
        `${baseUrl}fontawesome/css/sharp-solid.min.css`,
    ],
    staticDirectories: ["static", "storybook"],
};

export default config;
