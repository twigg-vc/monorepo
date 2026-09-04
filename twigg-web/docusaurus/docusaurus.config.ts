import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const SITE_URL = process.env.DOCUSAURUS_SITE_URL ?? 'http://localhost:9001';

// Kept in sync with handlers/landing/files/index.html.
const SITE_DESCRIPTION =
  "Twigg is the open source alternative to Critique, Google's internal code " +
  'collaboration platform. Stacked commits, code review, OWNERS, and CI/CD.';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)
const config: Config = {
  title: 'Twigg',
  tagline: 'Open Source Critique',
  favicon: 'img/twigg-circle-icon.png',

  // Future flags, see https://docusaurus.io/docs/api/docusaurus-config#future
  future: {
    v4: true, // Improve compatibility with the upcoming Docusaurus v4
  },

  // Set the production url of your site here
  url: SITE_URL,
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
    baseUrl: '/docs/v/2',

  onBrokenLinks: 'throw',
  onBrokenAnchors: 'throw',

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          routeBasePath: '/', // Serve the docs at the site's root
          sidebarPath: './sidebars.ts',
        },
        blog: {
          routeBasePath: '/blog',
          showReadingTime: true,
          blogTitle: 'Twigg Blog',
          blogDescription: SITE_DESCRIPTION,
          postsPerPage: 10,
          feedOptions: {
            type: ['rss', 'atom'],
            xslt: true,
          },
          // A post without a truncate marker lands whole on the index page.
          // Fail the build rather than ship that.
          onInlineTags: 'throw',
          onInlineAuthors: 'throw',
          onUntruncatedBlogPosts: 'throw',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  // Builds a static index at build time and ships it inside build/, so
  // search makes no network calls and needs no external service.
  themes: [
    [
      '@easyops-cn/docusaurus-search-local',
      {
        hashed: true,
        language: ['en'],
        indexDocs: true,
        indexBlog: true,
        docsRouteBasePath: '/',
        blogRouteBasePath: '/blog',
      },
    ],
  ],

  themeConfig: {
    image: 'img/twigg-og-card.png',
    metadata: [
      {name: 'description', content: SITE_DESCRIPTION},
      {property: 'og:description', content: SITE_DESCRIPTION},
      {name: 'twitter:card', content: 'summary_large_image'},
      {name: 'twitter:site', content: '@TwiggVc'},
    ],
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: "Twigg",
      logo: {
        alt: 'Twigg — Open Source Critique',
        src: 'img/twigg-circle-icon.png',
        href: 'https://twigg.vc/home',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'tutorialSidebar',
          position: 'left',
          label: 'Documentation',
        },
        {to: '/blog', label: 'Blog', position: 'left'},
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Product',
          items: [
            {
              label: 'Home',
              href: `${SITE_URL}/`,
            },
            {
                label: 'Get started',
                href: `${SITE_URL}/home`,
            },
            {
                label: 'Source',
                href: `https://github.com/twigg-vc/monorepo`,
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'Discord',
              href: 'https://discord.gg/udpz3faxwQ',
            },
            {
              label: 'X',
              href: 'https://x.com/TwiggVc',
            },
          ],
        },
        {
          title: 'Legal',
          items: [
            {
              label: 'Terms',
              href: `${SITE_URL}/terms`,
            },
            {
              label: 'Privacy',
              href: `${SITE_URL}/privacy`,
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'Blog',
              to: '/blog',
            },
          ],
        },
      ],
        copyright: `Copyright © ${new Date().getFullYear()} Twigg. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;