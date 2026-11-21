export type HomepageBackground = 'light' | 'dark' | 'gradient';
export type HomepageLayout = 'classic' | 'centered' | 'split';
export type HomepageSectionLayout = 'center' | 'left' | 'cards';

export interface HomepageTheme {
  primaryColor: string;
  background: HomepageBackground;
  layout: HomepageLayout;
}

export interface HomepageHero {
  eyebrow: string;
  title: string;
  subtitle: string;
  primaryCta: string;
  secondaryCta: string;
}

export interface HomepageStat {
  value: string;
  label: string;
}

export interface HomepageSectionItem {
  title: string;
  description: string;
}

export interface HomepageSection {
  id: string;
  title: string;
  body: string;
  layout: HomepageSectionLayout;
  visible: boolean;
  items: HomepageSectionItem[];
}

export interface HomepageContent {
  brandName: string;
  logoText: string;
  nav: string[];
  theme: HomepageTheme;
  hero: HomepageHero;
  stats: HomepageStat[];
  sections: HomepageSection[];
  footer: string;
}

/** GET /home/homepage 的响应；int64/timestamp 在 JSON 中可能是字符串。 */
export interface HomepageRecord {
  id: string;
  draftJson: string;
  publishedJson: string;
  publishedAt?: string;
  version: number | string;
  updatedBy?: string;
  updateTime?: string;
  application?: {
    id: string;
    code: string;
    name: string;
    status: string;
  };
}

export interface PublishedHomepageRecord {
  contentJson: string;
  publishedAt?: string;
  version: number | string;
}

const DEFAULT_HOMEPAGE: HomepageContent = {
  brandName: '星河科技',
  logoText: 'XH',
  nav: ['首页', '产品', '解决方案', '关于我们'],
  theme: { primaryColor: '#2563eb', background: 'light', layout: 'classic' },
  hero: {
    eyebrow: '为企业增长而生',
    title: '让复杂的业务，变得简单可增长',
    subtitle: '从数据到决策，从流程到协同，一套平台连接团队、客户与未来。',
    primaryCta: '预约演示',
    secondaryCta: '了解产品',
  },
  stats: [
    { value: '12 年', label: '行业积累' },
    { value: '3000+', label: '服务企业' },
    { value: '99.99%', label: '服务可用性' },
  ],
  sections: [
    {
      id: 'sec-1',
      title: '一套平台，覆盖全链路',
      body: '从线索到交付，把分散的系统连接成一条可观测的业务链路。',
      layout: 'cards',
      visible: true,
      items: [
        { title: '统一数据', description: '跨系统数据标准化，实时同步。' },
        { title: '流程协同', description: '任务、审批、消息一体化。' },
        { title: '智能决策', description: '指标看板与预警，异常先发现。' },
      ],
    },
    {
      id: 'sec-2',
      title: '为增长而设计的体验',
      body: '让每个团队都能在自己的节奏里协作，让每一次决策都有数据支撑。',
      layout: 'center',
      visible: true,
      items: [],
    },
    {
      id: 'sec-3',
      title: '从今天开始，连接你的团队',
      body: '预约一次演示，看看星河如何把复杂业务变简单。',
      layout: 'left',
      visible: true,
      items: [],
    },
  ],
  footer: '© 2026 星河科技. 保留所有权利。',
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function asString(value: unknown, fallback: string): string {
  return typeof value === 'string' ? value : fallback;
}

function asBackground(value: unknown): HomepageBackground {
  return value === 'dark' || value === 'gradient' ? value : 'light';
}

function asLayout(value: unknown): HomepageLayout {
  return value === 'centered' || value === 'split' ? value : 'classic';
}

function asSectionLayout(value: unknown): HomepageSectionLayout {
  return value === 'left' || value === 'cards' ? value : 'center';
}

export function createDefaultHomepage(): HomepageContent {
  return JSON.parse(JSON.stringify(DEFAULT_HOMEPAGE)) as HomepageContent;
}

/** 最小校验 + 缺省补齐：旧数据/手工改坏 JSON 时页面仍可编辑。 */
export function normalizeHomepageContent(raw: unknown): HomepageContent {
  if (!isRecord(raw)) return createDefaultHomepage();
  const theme = isRecord(raw.theme) ? raw.theme : {};
  const hero = isRecord(raw.hero) ? raw.hero : {};
  const nav = Array.isArray(raw.nav) ? raw.nav.filter((x): x is string => typeof x === 'string') : DEFAULT_HOMEPAGE.nav;
  const stats = Array.isArray(raw.stats)
    ? raw.stats.filter(isRecord).map((x) => ({
        value: asString(x.value, ''),
        label: asString(x.label, ''),
      }))
    : DEFAULT_HOMEPAGE.stats;
  const sections = Array.isArray(raw.sections)
    ? raw.sections.filter(isRecord).map((x, index) => ({
        id: asString(x.id, `sec-${index + 1}`),
        title: asString(x.title, ''),
        body: asString(x.body, ''),
        layout: asSectionLayout(x.layout),
        visible: x.visible !== false,
        items: Array.isArray(x.items)
          ? x.items.filter(isRecord).map((item) => ({
              title: asString(item.title, ''),
              description: asString(item.description, ''),
            }))
          : [],
      }))
    : DEFAULT_HOMEPAGE.sections;

  return {
    brandName: asString(raw.brandName, DEFAULT_HOMEPAGE.brandName),
    logoText: asString(raw.logoText, DEFAULT_HOMEPAGE.logoText),
    nav: nav.length > 0 ? nav : DEFAULT_HOMEPAGE.nav,
    theme: {
      primaryColor: asString(theme.primaryColor, DEFAULT_HOMEPAGE.theme.primaryColor),
      background: asBackground(theme.background),
      layout: asLayout(theme.layout),
    },
    hero: {
      eyebrow: asString(hero.eyebrow, DEFAULT_HOMEPAGE.hero.eyebrow),
      title: asString(hero.title, DEFAULT_HOMEPAGE.hero.title),
      subtitle: asString(hero.subtitle, DEFAULT_HOMEPAGE.hero.subtitle),
      primaryCta: asString(hero.primaryCta, DEFAULT_HOMEPAGE.hero.primaryCta),
      secondaryCta: asString(hero.secondaryCta, DEFAULT_HOMEPAGE.hero.secondaryCta),
    },
    stats,
    sections,
    footer: asString(raw.footer, DEFAULT_HOMEPAGE.footer),
  };
}

export function parseHomepageJson(json: string): HomepageContent {
  if (!json || !json.trim()) return createDefaultHomepage();
  try {
    return normalizeHomepageContent(JSON.parse(json));
  } catch {
    return createDefaultHomepage();
  }
}

export function toVersionNumber(value: number | string | undefined): number {
  const n = typeof value === 'string' ? Number(value) : value ?? 0;
  return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0;
}
