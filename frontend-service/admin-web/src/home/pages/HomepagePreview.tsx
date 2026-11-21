import type { CSSProperties } from 'react';
import type { HomepageContent, HomepageSection } from '@/home/types/homepage';

interface HomepagePreviewProps {
  content: HomepageContent;
  /** 右栏缩略预览：整体收紧字号与留白；全屏预览用 false。 */
  compact?: boolean;
}

function hexToRgba(hex: string, alpha: number): string {
  const normalized = hex.replace('#', '').trim();
  const full = normalized.length === 3
    ? normalized.split('').map((c) => c + c).join('')
    : normalized;
  if (!/^[0-9a-fA-F]{6}$/.test(full)) return `rgba(37, 99, 235, ${alpha})`;
  const r = parseInt(full.slice(0, 2), 16);
  const g = parseInt(full.slice(2, 4), 16);
  const b = parseInt(full.slice(4, 6), 16);
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

export default function HomepagePreview({ content, compact = false }: HomepagePreviewProps) {
  const dark = content.theme.background === 'dark';
  const gradient = content.theme.background === 'gradient';
  const primary = content.theme.primaryColor || '#2563eb';
  const pageBg = dark
    ? '#0b1220'
    : gradient
      ? `linear-gradient(160deg, ${hexToRgba(primary, 0.14)} 0%, #f8fafc 42%, #ffffff 100%)`
      : '#f6f8fb';
  const surface = dark ? 'rgba(255,255,255,0.06)' : '#ffffff';
  const text = dark ? '#f8fafc' : '#0f172a';
  const muted = dark ? 'rgba(248,250,252,0.68)' : '#475569';
  const border = dark ? 'rgba(255,255,255,0.14)' : '#e2e8f0';
  const softPrimary = hexToRgba(primary, dark ? 0.24 : 0.1);

  const baseFont = compact ? 12 : 15;
  const heroTitleSize = compact ? 26 : 44;
  const sectionTitleSize = compact ? 20 : 28;
  const pagePadding = compact ? 18 : 32;
  const maxWidth = compact ? 880 : 1120;

  const sectionBlocks = content.sections.filter((section) => section.visible);

  return (
    <div
      style={{
        background: pageBg,
        color: text,
        fontFamily:
          '"Inter", "PingFang SC", "Microsoft YaHei", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
        fontSize: baseFont,
        lineHeight: 1.65,
        minHeight: compact ? 620 : 780,
        overflow: 'hidden',
      }}
    >
      {/* 顶部导航 */}
      <header
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 16,
          padding: `${compact ? 14 : 20}px ${pagePadding}px`,
          borderBottom: `1px solid ${border}`,
          background: dark ? 'rgba(11,18,32,0.7)' : 'rgba(255,255,255,0.72)',
          backdropFilter: 'blur(12px)',
          position: 'sticky',
          top: 0,
          zIndex: 2,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
          <div
            style={{
              width: compact ? 30 : 38,
              height: compact ? 30 : 38,
              borderRadius: 10,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontWeight: 700,
              fontSize: compact ? 13 : 16,
              color: '#fff',
              background: `linear-gradient(135deg, ${primary}, ${hexToRgba(primary, 0.7)})`,
              boxShadow: `0 6px 16px ${hexToRgba(primary, 0.25)}`,
              flex: 'none',
            }}
          >
            {content.logoText || 'XH'}
          </div>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontWeight: 700, fontSize: compact ? 14 : 17, whiteSpace: 'nowrap' }}>
              {content.brandName || '企业名称'}
            </div>
            {!compact && (
              <div style={{ color: muted, fontSize: 11, letterSpacing: 1.2 }}>ENTERPRISE SOLUTIONS</div>
            )}
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: compact ? 12 : 22, flexWrap: 'wrap' }}>
          {content.nav.map((item) => (
            <span key={item} style={{ color: muted, fontSize: compact ? 12 : 14, whiteSpace: 'nowrap' }}>
              {item}
            </span>
          ))}
          <div
            style={{
              padding: `${compact ? 6 : 8}px ${compact ? 12 : 18}px`,
              borderRadius: 999,
              background: primary,
              color: '#fff',
              fontWeight: 600,
              fontSize: compact ? 11 : 13,
              whiteSpace: 'nowrap',
              boxShadow: `0 6px 16px ${hexToRgba(primary, 0.28)}`,
            }}
          >
            联系我们
          </div>
        </div>
      </header>

      {/* 首屏 */}
      <section style={{ padding: `${compact ? 34 : 64}px ${pagePadding}px ${compact ? 28 : 54}px` }}>
        <div
          style={{
            maxWidth,
            margin: '0 auto',
            display: 'grid',
            gap: compact ? 20 : 44,
            gridTemplateColumns:
              content.theme.layout === 'classic'
                ? compact
                  ? '1.15fr 0.85fr'
                  : '1.1fr 0.9fr'
                : content.theme.layout === 'split'
                  ? '1fr 1fr'
                  : '1fr',
            alignItems: 'center',
            textAlign: content.theme.layout === 'centered' ? 'center' : 'left',
          }}
        >
          <div>
            <div
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 8,
                padding: `${compact ? 4 : 6}px ${compact ? 9 : 12}px`,
                borderRadius: 999,
                background: softPrimary,
                color: primary,
                fontWeight: 600,
                fontSize: compact ? 10 : 12,
                letterSpacing: 0.4,
                marginBottom: compact ? 14 : 22,
              }}
            >
              <span
                style={{
                  width: 6,
                  height: 6,
                  borderRadius: '50%',
                  background: primary,
                  display: 'inline-block',
                }}
              />
              {content.hero.eyebrow || '为企业增长而生'}
            </div>
            <div
              style={{
                fontSize: heroTitleSize,
                lineHeight: 1.16,
                fontWeight: 800,
                letterSpacing: compact ? -0.4 : -1.2,
                marginBottom: compact ? 12 : 18,
                background: dark
                  ? `linear-gradient(120deg, #ffffff 20%, ${hexToRgba(primary, 0.95)} 100%)`
                  : `linear-gradient(120deg, ${text} 25%, ${primary} 100%)`,
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
              }}
            >
              {content.hero.title || '让复杂的业务，变得简单可增长'}
            </div>
            <p style={{ color: muted, fontSize: compact ? 12 : 16, maxWidth: compact ? 430 : 560, margin: 0 }}>
              {content.hero.subtitle}
            </p>
            <div style={{ display: 'flex', gap: 12, marginTop: compact ? 20 : 30, flexWrap: 'wrap' }}>
              <div
                style={{
                  padding: `${compact ? 8 : 11}px ${compact ? 16 : 24}px`,
                  borderRadius: compact ? 9 : 12,
                  background: primary,
                  color: '#fff',
                  fontWeight: 700,
                  fontSize: compact ? 12 : 14,
                  boxShadow: `0 10px 24px ${hexToRgba(primary, 0.3)}`,
                }}
              >
                {content.hero.primaryCta || '预约演示'}
              </div>
              <div
                style={{
                  padding: `${compact ? 8 : 11}px ${compact ? 16 : 24}px`,
                  borderRadius: compact ? 9 : 12,
                  background: dark ? 'rgba(255,255,255,0.08)' : '#fff',
                  border: `1px solid ${border}`,
                  color: text,
                  fontWeight: 600,
                  fontSize: compact ? 12 : 14,
                }}
              >
                {content.hero.secondaryCta || '了解产品'}
              </div>
            </div>
          </div>

          {content.theme.layout !== 'centered' && (
            <div
              style={{
                borderRadius: compact ? 14 : 22,
                padding: compact ? 14 : 22,
                background: dark
                  ? `linear-gradient(145deg, ${hexToRgba(primary, 0.3)}, rgba(255,255,255,0.04))`
                  : `linear-gradient(145deg, ${hexToRgba(primary, 0.22)}, rgba(255,255,255,0.9))`,
                border: `1px solid ${border}`,
                boxShadow: `0 24px 60px ${hexToRgba(primary, dark ? 0.2 : 0.12)}`,
                minHeight: compact ? 260 : 360,
                position: 'relative',
                overflow: 'hidden',
              }}
            >
              <div
                style={{
                  position: 'absolute',
                  width: 220,
                  height: 220,
                  borderRadius: '50%',
                  background: hexToRgba(primary, 0.22),
                  top: -80,
                  right: -60,
                  filter: 'blur(6px)',
                }}
              />
              <div
                style={{
                  position: 'relative',
                  background: surface,
                  borderRadius: compact ? 10 : 16,
                  padding: compact ? 14 : 20,
                  border: `1px solid ${border}`,
                  boxShadow: `0 12px 30px ${hexToRgba(primary, 0.08)}`,
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <div>
                    <div style={{ color: muted, fontSize: compact ? 9 : 11, letterSpacing: 0.8 }}>
                      GROWTH OVERVIEW
                    </div>
                    <div style={{ fontWeight: 800, fontSize: compact ? 17 : 22, marginTop: 2 }}>
                      +128.6%
                    </div>
                  </div>
                  <div
                    style={{
                      padding: `${compact ? 3 : 5}px ${compact ? 7 : 10}px`,
                      borderRadius: 8,
                      background: softPrimary,
                      color: primary,
                      fontSize: compact ? 9 : 11,
                      fontWeight: 700,
                    }}
                  >
                    同比
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'flex-end', gap: compact ? 5 : 8, height: compact ? 92 : 130, marginTop: compact ? 14 : 22 }}>
                  {[38, 52, 44, 68, 58, 82, 72, 96].map((h, index) => (
                    <div
                      key={index}
                      style={{
                        flex: 1,
                        height: `${h}%`,
                        borderRadius: compact ? 4 : 6,
                        background:
                          index >= 5
                            ? `linear-gradient(180deg, ${primary}, ${hexToRgba(primary, 0.55)})`
                            : hexToRgba(primary, dark ? 0.3 : 0.18),
                      }}
                    />
                  ))}
                </div>
                <div style={{ display: 'flex', gap: compact ? 8 : 14, marginTop: compact ? 12 : 18 }}>
                  {content.stats.slice(0, 3).map((stat) => (
                    <div
                      key={stat.label}
                      style={{
                        flex: 1,
                        padding: `${compact ? 6 : 9}px ${compact ? 7 : 10}px`,
                        borderRadius: 8,
                        background: dark ? 'rgba(255,255,255,0.05)' : '#f8fafc',
                        border: `1px solid ${border}`,
                      }}
                    >
                      <div style={{ fontWeight: 700, fontSize: compact ? 11 : 14 }}>{stat.value}</div>
                      <div style={{ color: muted, fontSize: compact ? 8 : 10 }}>{stat.label}</div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>

        {content.theme.layout === 'centered' && content.stats.length > 0 && (
          <div
            style={{
              maxWidth,
              margin: `${compact ? 22 : 38}px auto 0`,
              display: 'flex',
              gap: compact ? 10 : 20,
              flexWrap: 'wrap',
              justifyContent: 'center',
            }}
          >
            {content.stats.map((stat) => (
              <div
                key={stat.label}
                style={{
                  minWidth: compact ? 110 : 150,
                  padding: `${compact ? 10 : 14}px ${compact ? 12 : 18}px`,
                  borderRadius: 12,
                  background: surface,
                  border: `1px solid ${border}`,
                  textAlign: 'center',
                }}
              >
                <div style={{ fontWeight: 800, fontSize: compact ? 16 : 22, color: primary }}>{stat.value}</div>
                <div style={{ color: muted, fontSize: compact ? 9 : 11, marginTop: 2 }}>{stat.label}</div>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* 区块 */}
      {sectionBlocks.map((section, index) => (
        <SectionBlock
          key={section.id}
          section={section}
          index={index}
          compact={compact}
          primary={primary}
          muted={muted}
          surface={surface}
          border={border}
          softPrimary={softPrimary}
          sectionTitleSize={sectionTitleSize}
          maxWidth={maxWidth}
          pagePadding={pagePadding}
        />
      ))}

      {/* 页脚 */}
      <footer
        style={{
          borderTop: `1px solid ${border}`,
          padding: `${compact ? 18 : 26}px ${pagePadding}px`,
          marginTop: compact ? 6 : 14,
          background: dark ? 'rgba(255,255,255,0.03)' : 'rgba(255,255,255,0.7)',
        }}
      >
        <div
          style={{
            maxWidth,
            margin: '0 auto',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            gap: 12,
            flexWrap: 'wrap',
          }}
        >
          <div style={{ fontWeight: 700, fontSize: compact ? 12 : 14 }}>
            {content.brandName}
            <span style={{ color: muted, fontWeight: 400, marginLeft: 8, fontSize: compact ? 10 : 12 }}>
              {content.footer}
            </span>
          </div>
          <div style={{ display: 'flex', gap: compact ? 10 : 18, color: muted, fontSize: compact ? 10 : 12 }}>
            {content.nav.map((item) => (
              <span key={item}>{item}</span>
            ))}
          </div>
        </div>
      </footer>
    </div>
  );
}

interface SectionBlockProps {
  section: HomepageSection;
  index: number;
  compact: boolean;
  primary: string;
  muted: string;
  surface: string;
  border: string;
  softPrimary: string;
  sectionTitleSize: number;
  maxWidth: number;
  pagePadding: number;
}

function SectionBlock({
  section,
  index,
  compact,
  primary,
  muted,
  surface,
  border,
  softPrimary,
  sectionTitleSize,
  maxWidth,
  pagePadding,
}: SectionBlockProps) {
  const sectionStyle: CSSProperties = {
    padding: `${compact ? 26 : 44}px ${pagePadding}px`,
    borderTop: index === 0 ? `1px solid ${border}` : undefined,
  };
  const contentStyle: CSSProperties = {
    maxWidth,
    margin: '0 auto',
    textAlign: section.layout === 'center' ? 'center' : 'left',
  };

  return (
    <section style={sectionStyle}>
      <div style={contentStyle}>
        <div
          style={{
            display: 'inline-block',
            padding: `${compact ? 3 : 5}px ${compact ? 8 : 11}px`,
            borderRadius: 999,
            background: softPrimary,
            color: primary,
            fontWeight: 700,
            fontSize: compact ? 9 : 11,
            letterSpacing: 0.6,
            marginBottom: compact ? 10 : 16,
          }}
        >
          0{index + 1} / HIGHLIGHT
        </div>
        <div
          style={{
            fontSize: sectionTitleSize,
            fontWeight: 800,
            letterSpacing: compact ? -0.2 : -0.6,
            lineHeight: 1.25,
            marginBottom: compact ? 8 : 12,
          }}
        >
          {section.title || '内容区块'}
        </div>
        <p
          style={{
            color: muted,
            fontSize: compact ? 12 : 15,
            maxWidth: section.layout === 'center' ? 640 : 720,
            margin: section.layout === 'center' ? '0 auto' : 0,
          }}
        >
          {section.body}
        </p>

        {section.layout === 'cards' && section.items.length > 0 && (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: compact
                ? 'repeat(auto-fit, minmax(150px, 1fr))'
                : 'repeat(auto-fit, minmax(220px, 1fr))',
              gap: compact ? 10 : 18,
              marginTop: compact ? 18 : 28,
              textAlign: 'left',
            }}
          >
            {section.items.map((item) => (
              <div
                key={`${section.id}-${item.title}`}
                style={{
                  padding: compact ? 12 : 20,
                  borderRadius: compact ? 10 : 16,
                  background: surface,
                  border: `1px solid ${border}`,
                  boxShadow: `0 10px 26px ${hexToRgba(primary, 0.06)}`,
                  position: 'relative',
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: 36,
                    height: 3,
                    background: primary,
                    borderRadius: 3,
                  }}
                />
                <div style={{ fontWeight: 700, fontSize: compact ? 12 : 15, marginBottom: compact ? 4 : 7 }}>
                  {item.title || '能力'}
                </div>
                <div style={{ color: muted, fontSize: compact ? 10 : 12.5 }}>{item.description}</div>
              </div>
            ))}
          </div>
        )}

        {section.layout === 'left' && (
          <div
            style={{
              marginTop: compact ? 14 : 22,
              paddingLeft: compact ? 10 : 16,
              borderLeft: `3px solid ${primary}`,
              color: muted,
              fontSize: compact ? 10 : 12.5,
            }}
          >
            了解更多 → 与我们一起，把复杂业务变成可增长的确定性。
          </div>
        )}
      </div>
    </section>
  );
}
