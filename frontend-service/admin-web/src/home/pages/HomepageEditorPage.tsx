import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Alert,
  App,
  Button,
  Card,
  ColorPicker,
  Col,
  Divider,
  Empty,
  Flex,
  Input,
  Modal,
  Popconfirm,
  Result,
  Row,
  Segmented,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from 'antd';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  CloudUploadOutlined,
  DeleteOutlined,
  EyeOutlined,
  PlusOutlined,
  ReloadOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { ApiError } from '@/shared/api/request';
import { usePermissions } from '@/shared/hooks/permission-context';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import { homeApi } from '@/home/api/home';
import HomepagePreview from '@/home/pages/HomepagePreview';
import {
  parseHomepageJson,
  toVersionNumber,
  type HomepageBackground,
  type HomepageContent,
  type HomepageLayout,
  type HomepageRecord,
  type HomepageSection,
  type HomepageSectionLayout,
} from '@/home/types/homepage';

const { Text, Title } = Typography;

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div style={{ marginBottom: 16 }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'baseline',
          justifyContent: 'space-between',
          gap: 12,
          marginBottom: 6,
        }}
      >
        <span style={{ fontWeight: 600, fontSize: 13 }}>{label}</span>
        {hint && <span style={{ color: '#94a3b8', fontSize: 11 }}>{hint}</span>}
      </div>
      {children}
    </div>
  );
}

export default function HomepageEditorPage() {
  const { message } = App.useApp();
  const { hasPermission, loading: permissionLoading } = usePermissions();
  const { isSubmitting, run } = useSubmitLock();
  const [content, setContent] = useState<HomepageContent | null>(null);
  const [record, setRecord] = useState<HomepageRecord | null>(null);
  const [baseline, setBaseline] = useState('');
  const [previewOpen, setPreviewOpen] = useState(false);

  const query = useQuery({
    queryKey: ['home', 'homepage'],
    queryFn: homeApi.get,
  });

  const applyRecord = useCallback((next: HomepageRecord) => {
    const parsed = parseHomepageJson(next.draftJson);
    setRecord(next);
    setContent(parsed);
    setBaseline(JSON.stringify(parsed));
  }, []);

  useEffect(() => {
    if (query.data) applyRecord(query.data);
  }, [query.data, applyRecord]);

  const dirty = useMemo(
    () => Boolean(content) && JSON.stringify(content) !== baseline,
    [content, baseline],
  );
  const version = toVersionNumber(record?.version);
  const canUpdate = hasPermission('home.page.update');
  const canPublish = hasPermission('home.page.publish');

  const update = useCallback((patch: Partial<HomepageContent>) => {
    setContent((prev) => (prev ? { ...prev, ...patch } : prev));
  }, []);

  const updateTheme = useCallback((patch: Partial<HomepageContent['theme']>) => {
    setContent((prev) => (prev ? { ...prev, theme: { ...prev.theme, ...patch } } : prev));
  }, []);

  const updateHero = useCallback((patch: Partial<HomepageContent['hero']>) => {
    setContent((prev) => (prev ? { ...prev, hero: { ...prev.hero, ...patch } } : prev));
  }, []);

  const updateSection = useCallback((index: number, patch: Partial<HomepageSection>) => {
    setContent((prev) => {
      if (!prev) return prev;
      const sections = [...prev.sections];
      sections[index] = { ...sections[index], ...patch };
      return { ...prev, sections };
    });
  }, []);

  const moveSection = useCallback((index: number, offset: -1 | 1) => {
    setContent((prev) => {
      if (!prev) return prev;
      const target = index + offset;
      if (target < 0 || target >= prev.sections.length) return prev;
      const sections = [...prev.sections];
      [sections[index], sections[target]] = [sections[target], sections[index]];
      return { ...prev, sections };
    });
  }, []);

  const removeSection = useCallback((index: number) => {
    setContent((prev) => (prev ? { ...prev, sections: prev.sections.filter((_, i) => i !== index) } : prev));
  }, []);

  const addSection = useCallback(() => {
    setContent((prev) =>
      prev
        ? {
            ...prev,
            sections: [
              ...prev.sections,
              {
                id: `sec-${Date.now()}`,
                title: '新内容区块',
                body: '用一句话说明这个区块要讲什么。',
                layout: 'center',
                visible: true,
                items: [],
              },
            ],
          }
        : prev,
    );
  }, []);

  const updateSectionItem = useCallback(
    (sectionIndex: number, itemIndex: number, patch: { title?: string; description?: string }) => {
      setContent((prev) => {
        if (!prev) return prev;
        const sections = [...prev.sections];
        const items = [...sections[sectionIndex].items];
        items[itemIndex] = { ...items[itemIndex], ...patch };
        sections[sectionIndex] = { ...sections[sectionIndex], items };
        return { ...prev, sections };
      });
    },
    [],
  );

  const addSectionItem = useCallback((sectionIndex: number) => {
    setContent((prev) => {
      if (!prev) return prev;
      const sections = [...prev.sections];
      sections[sectionIndex] = {
        ...sections[sectionIndex],
        items: [
          ...sections[sectionIndex].items,
          { title: '新能力', description: '一句话描述这项能力。' },
        ],
      };
      return { ...prev, sections };
    });
  }, []);

  const removeSectionItem = useCallback((sectionIndex: number, itemIndex: number) => {
    setContent((prev) => {
      if (!prev) return prev;
      const sections = [...prev.sections];
      sections[sectionIndex] = {
        ...sections[sectionIndex],
        items: sections[sectionIndex].items.filter((_, i) => i !== itemIndex),
      };
      return { ...prev, sections };
    });
  }, []);

  const errorText = (error: unknown, fallback: string) => {
    if (error instanceof ApiError && (error.status === 409 || error.status === 412)) {
      return '内容已被其他人更新，请点「刷新」加载最新版本后再保存。';
    }
    if (error instanceof ApiError && error.status === 403) {
      return '没有该操作权限，请联系管理员分配 home.page 权限。';
    }
    return error instanceof Error && error.message ? error.message : fallback;
  };

  const handleSave = () =>
    run(async () => {
      if (!content) return;
      try {
        const saved = await homeApi.saveDraft(JSON.stringify(content), version);
        applyRecord(saved);
        message.success(`草稿已保存 v${toVersionNumber(saved.version)}`);
      } catch (error) {
        message.error(errorText(error, '保存草稿失败'));
      }
    });

  const handlePublish = () =>
    run(async () => {
      if (!content) return;
      try {
        let base = record;
        if (!base || base.id === '0' || dirty) {
          base = await homeApi.saveDraft(JSON.stringify(content), version);
          applyRecord(base);
        }
        const published = await homeApi.publish(toVersionNumber(base.version));
        applyRecord(published);
        message.success(`已发布 v${toVersionNumber(published.version)}，官网读取最新已发布内容`);
      } catch (error) {
        message.error(errorText(error, '发布失败'));
      }
    });

  const handleReload = () => {
    query.refetch();
  };

  if (permissionLoading || query.isPending) {
    return (
      <Flex justify="center" align="center" style={{ minHeight: 420 }}>
        <Spin size="large" tip="加载首页配置..." />
      </Flex>
    );
  }

  if (!hasPermission('home.page.get')) {
    return (
      <Result
        status="403"
        title="没有访问权限"
        subTitle="当前账号未持有 home.page.get，无法查看企业官网配置。"
      />
    );
  }

  if (query.isError || !content) {
    return (
      <Result
        status="error"
        title="首页配置加载失败"
        subTitle={query.error instanceof Error ? query.error.message : '请稍后重试'}
        extra={
          <Button type="primary" onClick={handleReload}>
            重新加载
          </Button>
        }
      />
    );
  }

  return (
    <div style={{ maxWidth: 1680, margin: '0 auto' }}>
      <Card
        style={{ marginBottom: 20 }}
        styles={{ body: { padding: '18px 22px' } }}
      >
        <Flex justify="space-between" align="center" gap={16} wrap="wrap">
          <div>
            <Title level={4} style={{ margin: 0 }}>
              企业官网首页
            </Title>
            <Space wrap size={8} style={{ marginTop: 10 }}>
              <Tag color={dirty ? 'orange' : 'green'}>{dirty ? '有未保存修改' : '已保存'}</Tag>
              <Tag>草稿 v{version}</Tag>
              {record?.publishedAt ? (
                <Tag color="blue">已发布 {dayjs(record.publishedAt).format('YYYY-MM-DD HH:mm')}</Tag>
              ) : (
                <Tag>尚未发布</Tag>
              )}
            </Space>
          </div>
          <Space wrap>
            <Button icon={<ReloadOutlined />} onClick={handleReload}>
              刷新
            </Button>
            <Button icon={<EyeOutlined />} onClick={() => setPreviewOpen(true)}>
              全屏预览
            </Button>
            {canUpdate && (
              <Button icon={<SaveOutlined />} loading={isSubmitting} onClick={handleSave}>
                保存草稿
              </Button>
            )}
            {canPublish && canUpdate && (
              <Button
                type="primary"
                icon={<CloudUploadOutlined />}
                loading={isSubmitting}
                onClick={handlePublish}
              >
                发布
              </Button>
            )}
          </Space>
        </Flex>
      </Card>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 20 }}
        message="所见即所得：左侧任意修改都会实时反映到右侧预览。保存只写草稿，点击发布后官网才读取新版本。"
      />

      <Row gutter={[20, 20]}>
        <Col xs={24} xl={11}>
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Card title="站点设置" styles={{ body: { padding: 20 } }}>
              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Field label="品牌名称" hint="导航与页脚展示">
                    <Input
                      value={content.brandName}
                      maxLength={40}
                      onChange={(e) => update({ brandName: e.target.value })}
                    />
                  </Field>
                </Col>
                <Col xs={24} md={12}>
                  <Field label="Logo 文字" hint="1-4 个字符">
                    <Input
                      value={content.logoText}
                      maxLength={4}
                      onChange={(e) => update({ logoText: e.target.value })}
                    />
                  </Field>
                </Col>
              </Row>
              <Field label="导航菜单" hint="输入后回车添加">
                <Select
                  mode="tags"
                  value={content.nav}
                  tokenSeparators={[',', '，']}
                  placeholder="首页、产品、解决方案"
                  onChange={(value: string[]) => update({ nav: value })}
                />
              </Field>
              <Field label="页脚版权" hint="展示在页面底部">
                <Input
                  value={content.footer}
                  maxLength={120}
                  onChange={(e) => update({ footer: e.target.value })}
                />
              </Field>
            </Card>

            <Card title="首屏文案" styles={{ body: { padding: 20 } }}>
              <Field label="眉标">
                <Input
                  value={content.hero.eyebrow}
                  maxLength={30}
                  onChange={(e) => updateHero({ eyebrow: e.target.value })}
                />
              </Field>
              <Field label="主标题" hint="建议 10-24 字">
                <Input.TextArea
                  value={content.hero.title}
                  autoSize={{ minRows: 2, maxRows: 3 }}
                  maxLength={80}
                  onChange={(e) => updateHero({ title: e.target.value })}
                />
              </Field>
              <Field label="副标题">
                <Input.TextArea
                  value={content.hero.subtitle}
                  autoSize={{ minRows: 2, maxRows: 4 }}
                  maxLength={160}
                  onChange={(e) => updateHero({ subtitle: e.target.value })}
                />
              </Field>
              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Field label="主按钮">
                    <Input
                      value={content.hero.primaryCta}
                      maxLength={20}
                      onChange={(e) => updateHero({ primaryCta: e.target.value })}
                    />
                  </Field>
                </Col>
                <Col xs={24} md={12}>
                  <Field label="次按钮">
                    <Input
                      value={content.hero.secondaryCta}
                      maxLength={20}
                      onChange={(e) => updateHero({ secondaryCta: e.target.value })}
                    />
                  </Field>
                </Col>
              </Row>
            </Card>

            <Card title="主题与布局" styles={{ body: { padding: 20 } }}>
              <Field label="主题色" hint="按钮、强调线、渐变都跟随">
                <ColorPicker
                  value={content.theme.primaryColor}
                  showText
                  onChange={(color) => updateTheme({ primaryColor: color.toHexString() })}
                />
              </Field>
              <Field label="背景风格">
                <Segmented
                  block
                  value={content.theme.background}
                  options={[
                    { label: '浅色', value: 'light' },
                    { label: '深色', value: 'dark' },
                    { label: '渐变', value: 'gradient' },
                  ]}
                  onChange={(value) => updateTheme({ background: value as HomepageBackground })}
                />
              </Field>
              <Field label="整页布局" hint="首屏文字与视觉卡片的排布">
                <Segmented
                  block
                  value={content.theme.layout}
                  options={[
                    { label: '经典', value: 'classic' },
                    { label: '居中', value: 'centered' },
                    { label: '分栏', value: 'split' },
                  ]}
                  onChange={(value) => updateTheme({ layout: value as HomepageLayout })}
                />
              </Field>
            </Card>

            <Card
              title="数据指标"
              extra={
                <Button
                  type="link"
                  icon={<PlusOutlined />}
                  onClick={() => update({ stats: [...content.stats, { value: '100+', label: '新指标' }] })}
                >
                  添加
                </Button>
              }
              styles={{ body: { padding: 20 } }}
            >
              {content.stats.length === 0 && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无指标" />}
              {content.stats.map((stat, index) => (
                <Row gutter={8} key={`${stat.label}-${index}`} style={{ marginBottom: 10 }}>
                  <Col span={9}>
                    <Input
                      value={stat.value}
                      placeholder="数值"
                      maxLength={20}
                      onChange={(e) => {
                        const stats = [...content.stats];
                        stats[index] = { ...stats[index], value: e.target.value };
                        update({ stats });
                      }}
                    />
                  </Col>
                  <Col span={13}>
                    <Input
                      value={stat.label}
                      placeholder="说明"
                      maxLength={20}
                      onChange={(e) => {
                        const stats = [...content.stats];
                        stats[index] = { ...stats[index], label: e.target.value };
                        update({ stats });
                      }}
                    />
                  </Col>
                  <Col span={2}>
                    <Button
                      type="text"
                      danger
                      icon={<DeleteOutlined />}
                      onClick={() => update({ stats: content.stats.filter((_, i) => i !== index) })}
                    />
                  </Col>
                </Row>
              ))}
            </Card>

            <Card
              title={`内容区块（${content.sections.length}）`}
              extra={
                <Button type="primary" ghost icon={<PlusOutlined />} onClick={addSection}>
                  新增区块
                </Button>
              }
              styles={{ body: { padding: 16 } }}
            >
              {content.sections.length === 0 && (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有区块，点击右上角新增" />
              )}
              <Space direction="vertical" size={14} style={{ width: '100%' }}>
                {content.sections.map((section, index) => (
                  <Card
                    key={section.id}
                    size="small"
                    title={<span style={{ fontSize: 13 }}>区块 {index + 1}</span>}
                    extra={
                      <Space size={2}>
                        <Button
                          type="text"
                          size="small"
                          icon={<ArrowUpOutlined />}
                          disabled={index === 0}
                          onClick={() => moveSection(index, -1)}
                        />
                        <Button
                          type="text"
                          size="small"
                          icon={<ArrowDownOutlined />}
                          disabled={index === content.sections.length - 1}
                          onClick={() => moveSection(index, 1)}
                        />
                        <Popconfirm
                          title="删除这个区块？"
                          description="删除后需要保存草稿才会生效。"
                          okText="删除"
                          cancelText="取消"
                          onConfirm={() => removeSection(index)}
                        >
                          <Button type="text" size="small" danger icon={<DeleteOutlined />} />
                        </Popconfirm>
                      </Space>
                    }
                    styles={{ body: { padding: 14 } }}
                  >
                    <Field label="区块标题">
                      <Input
                        value={section.title}
                        maxLength={60}
                        onChange={(e) => updateSection(index, { title: e.target.value })}
                      />
                    </Field>
                    <Field label="区块正文">
                      <Input.TextArea
                        value={section.body}
                        autoSize={{ minRows: 2, maxRows: 4 }}
                        maxLength={200}
                        onChange={(e) => updateSection(index, { body: e.target.value })}
                      />
                    </Field>
                    <Row gutter={12} align="middle">
                      <Col span={14}>
                        <Field label="区块布局">
                          <Select
                            value={section.layout}
                            options={[
                              { label: '居中', value: 'center' },
                              { label: '左对齐', value: 'left' },
                              { label: '卡片组', value: 'cards' },
                            ]}
                            onChange={(value: HomepageSectionLayout) =>
                              updateSection(index, { layout: value })
                            }
                          />
                        </Field>
                      </Col>
                      <Col span={10}>
                        <Field label="是否显示">
                          <Switch
                            checked={section.visible}
                            checkedChildren="显示"
                            unCheckedChildren="隐藏"
                            onChange={(checked) => updateSection(index, { visible: checked })}
                          />
                        </Field>
                      </Col>
                    </Row>

                    {section.layout === 'cards' && (
                      <div style={{ background: '#f8fafc', borderRadius: 10, padding: 12 }}>
                        <Flex justify="space-between" align="center" style={{ marginBottom: 10 }}>
                          <Text strong style={{ fontSize: 12 }}>
                            卡片条目（{section.items.length}）
                          </Text>
                          <Button
                            size="small"
                            type="link"
                            icon={<PlusOutlined />}
                            onClick={() => addSectionItem(index)}
                          >
                            添加卡片
                          </Button>
                        </Flex>
                        {section.items.map((item, itemIndex) => (
                          <div
                            key={`${section.id}-item-${itemIndex}`}
                            style={{
                              display: 'grid',
                              gridTemplateColumns: '1fr 1fr 28px',
                              gap: 6,
                              marginBottom: 8,
                            }}
                          >
                            <Input
                              size="small"
                              value={item.title}
                              placeholder="卡片标题"
                              maxLength={30}
                              onChange={(e) =>
                                updateSectionItem(index, itemIndex, { title: e.target.value })
                              }
                            />
                            <Input
                              size="small"
                              value={item.description}
                              placeholder="卡片描述"
                              maxLength={60}
                              onChange={(e) =>
                                updateSectionItem(index, itemIndex, { description: e.target.value })
                              }
                            />
                            <Button
                              size="small"
                              type="text"
                              danger
                              icon={<DeleteOutlined />}
                              onClick={() => removeSectionItem(index, itemIndex)}
                            />
                          </div>
                        ))}
                        {section.items.length === 0 && (
                          <Text type="secondary" style={{ fontSize: 11 }}>
                            卡片组布局建议至少添加 2 个条目。
                          </Text>
                        )}
                      </div>
                    )}
                  </Card>
                ))}
              </Space>
            </Card>
          </Space>
        </Col>

        <Col xs={24} xl={13}>
          <div style={{ position: 'sticky', top: 88 }}>
            <Card
              title="实时预览"
              extra={<Tag color="processing">未保存也可见</Tag>}
              styles={{ body: { padding: 0, background: '#eef2f7' } }}
            >
              <div style={{ maxHeight: 'calc(100vh - 250px)', overflow: 'auto' }}>
                <HomepagePreview content={content} compact />
              </div>
            </Card>
            <Text type="secondary" style={{ display: 'block', fontSize: 11, marginTop: 8 }}>
              预览按桌面宽度渲染；点击「全屏预览」可查看完整比例。
            </Text>
          </div>
        </Col>
      </Row>

      <Divider />

      <Modal
        open={previewOpen}
        onCancel={() => setPreviewOpen(false)}
        footer={null}
        width="96vw"
        title="官网首页预览（含未保存修改）"
        styles={{
          body: { padding: 0, height: '86vh', overflow: 'auto', background: '#eef2f7' },
        }}
      >
        <div style={{ maxWidth: 1200, margin: '0 auto', boxShadow: '0 20px 60px rgba(15, 23, 42, 0.18)' }}>
          <HomepagePreview content={content} />
        </div>
      </Modal>
    </div>
  );
}
