import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  App,
  Button,
  Checkbox,
  Flex,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Select,
  Space,
  Tag,
  TreeSelect,
} from 'antd';
import { CheckCircleOutlined, StopOutlined } from '@ant-design/icons';
import type { TableColumnsType } from 'antd';
import { useSearchParams } from 'react-router-dom';
import { applicationApi, resourceApi } from '../../api/iam';
import { ListPageShell } from '../../components/ListPageShell';
import { useConfirmDelete } from '../../components/ConfirmDelete';
import { useApiErrorNotification } from '../../hooks/useApiError';
import { useSubmitLock } from '../../hooks/useSubmitLock';
import { useTableQuery } from '../../hooks/useTableQuery';
import { useApplicationContext } from '../../context/ApplicationContext';
import type {
  CreateResourceRequest,
  PermissionKind,
  Resource,
  ResourceTreeNode,
  UpdateResourceRequest,
} from '../../types';
import { PERMISSION_KIND_LABELS } from '../../types';
import { ACTIONS, buildSuccessMessage } from '../../types/copywriting';

const KIND_OPTIONS: { label: string; value: PermissionKind }[] = [
  { label: PERMISSION_KIND_LABELS.menu, value: 'menu' },
  { label: PERMISSION_KIND_LABELS.http_api, value: 'http_api' },
  { label: PERMISSION_KIND_LABELS.grpc, value: 'grpc' },
];

const KIND_TAG_COLORS: Record<PermissionKind, string> = {
  menu: 'blue',
  http_api: 'green',
  grpc: 'orange',
};

// 允许作为父级的类型映射
const ALLOWED_PARENT_KINDS: Record<PermissionKind, PermissionKind[]> = {
  menu: ['menu'],
  http_api: ['menu'],
  grpc: ['http_api'],
};

interface ParentOption {
  /** Tag + 名称（下拉项与选中态展示） */
  title: ReactNode;
  value: string;
  /** 纯文本名称，供搜索过滤（title 为 ReactNode 无法直接匹配） */
  name: string;
  children?: ParentOption[];
}

/**
 * 从资源树构建父级选择项：仅保留允许类型节点（类型不符但后代可选时提升后代）；
 * excludeId 及其整棵子树被排除（父级不能形成环）。
 */
function buildParentOptions(
  nodes: ResourceTreeNode[],
  allowedKinds: PermissionKind[],
  excludeId?: string,
): ParentOption[] {
  if (allowedKinds.length === 0) return [];
  const options: ParentOption[] = [];
  for (const node of nodes) {
    if (excludeId && node.resource.id === excludeId) continue;
    const children = buildParentOptions(node.children, allowedKinds, excludeId);
    if (!allowedKinds.includes(node.resource.kind)) {
      options.push(...children);
      continue;
    }
    const { kind, name } = node.resource;
    options.push({
      title: (
        <Space>
          <Tag color={KIND_TAG_COLORS[kind]}>{PERMISSION_KIND_LABELS[kind] ?? kind}</Tag>
          <span>{name}</span>
        </Space>
      ),
      value: node.resource.id,
      name,
      children: children.length > 0 ? children : undefined,
    });
  }
  return options;
}

/** 父级下拉搜索：按选项纯文本名称过滤 */
const filterParentOption = (input: string, node: unknown) => {
  const name = (node as { name?: string }).name ?? '';
  return name.toLowerCase().includes(input.trim().toLowerCase());
};

// TreeSelect 下拉框渲染在 Modal 内，避免被遮罩层遮挡
const getTreeSelectPopupContainer = (triggerNode: HTMLElement) =>
  triggerNode.parentElement?.parentElement?.parentElement || document.body;

interface CreateResourceFormValues {
  parentId?: string;
  kind: PermissionKind;
  key: string;
  name: string;
  rpcFullMethod?: string;
  displayOrder?: number;
}

interface CreateResourceModalProps {
  open: boolean;
  applicationId?: string;
  treeRoots: ResourceTreeNode[];
  onClose: () => void;
  onSuccess: () => void;
  onError: (error: unknown) => void;
}

function CreateResourceModal({
  open,
  applicationId,
  treeRoots,
  onClose,
  onSuccess,
  onError,
}: CreateResourceModalProps) {
  const [form] = Form.useForm<CreateResourceFormValues>();
  const { isSubmitting, run } = useSubmitLock();
  const createKind = Form.useWatch('kind', form);

  const parentOptions = useMemo(
    () => buildParentOptions(treeRoots, ALLOWED_PARENT_KINDS[createKind as PermissionKind] ?? []),
    [treeRoots, createKind],
  );

  const handleFinish = (values: CreateResourceFormValues) => {
    if (!applicationId) return;
    run(async () => {
      await resourceApi.create({
        applicationId,
        kind: values.kind,
        key: values.key,
        name: values.name,
        rpcFullMethod: values.rpcFullMethod || undefined,
        parentId: values.parentId || undefined,
        displayOrder: values.displayOrder ?? undefined,
      } satisfies CreateResourceRequest);
      form.resetFields();
      onSuccess();
    }).catch(onError);
  };

  return (
    <Modal
      title="新增资源"
      open={open}
      onCancel={onClose}
      onOk={() => form.submit()}
      confirmLoading={isSubmitting}
      okText={ACTIONS.create}
      cancelText={ACTIONS.cancel}
      destroyOnClose
    >
      <Form form={form} layout="vertical" onFinish={handleFinish}>
        <Form.Item label="父级资源" name="parentId">
          <TreeSelect
            treeData={parentOptions}
            placeholder="留空为根节点"
            allowClear
            treeDefaultExpandAll
            showSearch
            filterTreeNode={filterParentOption}
            style={{ width: '100%' }}
            getPopupContainer={getTreeSelectPopupContainer}
          />
        </Form.Item>
        <Form.Item label="资源类型" name="kind" rules={[{ required: true, message: '请选择资源类型' }]}>
          <Radio.Group options={KIND_OPTIONS} optionType="button" buttonStyle="solid" />
        </Form.Item>
        <Form.Item label="资源键" name="key" rules={[{ required: true, message: '请输入资源键' }]}>
          <Input />
        </Form.Item>
        <Form.Item label="资源名称" name="name" rules={[{ required: true, message: '请输入资源名称' }]}>
          <Input />
        </Form.Item>
        <Form.Item label="RPC 目标方法" name="rpcFullMethod">
          <Input />
        </Form.Item>
        <Form.Item label="排序值" name="displayOrder" extra="数值越小排序越靠前">
          <InputNumber style={{ width: '100%' }} placeholder="0" />
        </Form.Item>
      </Form>
    </Modal>
  );
}

interface EditResourceFormValues {
  parentId?: string;
  name: string;
  rpcFullMethod?: string;
  displayOrder?: number;
  disabled?: boolean;
}

interface EditResourceModalProps {
  record: Resource | null;
  treeRoots: ResourceTreeNode[];
  onClose: () => void;
  onSuccess: () => void;
  onError: (error: unknown) => void;
}

/** 编辑弹窗：携带 version 乐观锁；parentCleared 标记"原 有父级、现清空"的场景 */
function EditResourceModal({ record, treeRoots, onClose, onSuccess, onError }: EditResourceModalProps) {
  const [form] = Form.useForm<EditResourceFormValues>();
  const { isSubmitting, run } = useSubmitLock();

  const parentOptions = useMemo(
    () => (record ? buildParentOptions(treeRoots, ALLOWED_PARENT_KINDS[record.kind], record.id) : []),
    [treeRoots, record],
  );

  useEffect(() => {
    if (!record) return;
    form.setFieldsValue({
      parentId: record.parentId,
      name: record.name,
      rpcFullMethod: record.rpcFullMethod,
      displayOrder: record.displayOrder,
      disabled: record.enabledStatus !== 'enabled',
    });
  }, [record, form]);

  const handleFinish = (values: EditResourceFormValues) => {
    if (!record) return;
    run(async () => {
      const parentId = values.parentId || undefined;
      await resourceApi.update(record.id, {
        parentId,
        parentCleared: parentId === undefined && record.parentId !== undefined,
        name: values.name,
        rpcFullMethod: values.rpcFullMethod || undefined,
        displayOrder: values.displayOrder ?? undefined,
        disabled: values.disabled ?? false,
        version: record.version,
      } satisfies UpdateResourceRequest);
      form.resetFields();
      onSuccess();
    }).catch(onError);
  };

  return (
    <Modal
      title="编辑资源"
      open={record !== null}
      onCancel={onClose}
      onOk={() => form.submit()}
      confirmLoading={isSubmitting}
      okText={ACTIONS.save}
      cancelText={ACTIONS.cancel}
      destroyOnClose
    >
      {record ? (
        <Form form={form} layout="vertical" onFinish={handleFinish}>
          <Form.Item label="资源类型" extra="类型创建后不可变">
            <Tag color={KIND_TAG_COLORS[record.kind]}>{PERMISSION_KIND_LABELS[record.kind] ?? record.kind}</Tag>
          </Form.Item>
          <Form.Item label="父级资源" name="parentId">
            <TreeSelect
              treeData={parentOptions}
              placeholder="留空为根节点"
              allowClear
              treeDefaultExpandAll
              showSearch
              filterTreeNode={filterParentOption}
              style={{ width: '100%' }}
              getPopupContainer={getTreeSelectPopupContainer}
            />
          </Form.Item>
          <Form.Item label="资源名称" name="name" rules={[{ required: true, message: '请输入资源名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item label="RPC 目标方法" name="rpcFullMethod">
            <Input />
          </Form.Item>
          <Form.Item label="排序值" name="displayOrder" extra="数值越小排序越靠前">
            <InputNumber style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="禁用" name="disabled" valuePropName="checked">
            <Checkbox />
          </Form.Item>
        </Form>
      ) : null}
    </Modal>
  );
}

export default function ResourceManagementPage() {
  const { message } = App.useApp();
  const showNotification = useApiErrorNotification();
  const confirmDelete = useConfirmDelete();
  const { currentApplicationId } = useApplicationContext();
  const [searchParams, setSearchParams] = useSearchParams();
  const [applications, setApplications] = useState<{ id: string; name: string }[]>([]);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [editingRecord, setEditingRecord] = useState<Resource | null>(null);
  const [treeRoots, setTreeRoots] = useState<ResourceTreeNode[]>([]);
  const [treeVersion, setTreeVersion] = useState(0);

  const applicationId = searchParams.get('application_id') ?? undefined;
  const kindParam = searchParams.get('kind');
  const kindFilter: PermissionKind | undefined =
    kindParam === 'menu' || kindParam === 'http_api' || kindParam === 'grpc' ? kindParam : undefined;

  const tableQuery = useTableQuery({
    domain: 'resources',
    filterKeys: ['application_id', 'kind'],
    fetcher: (currentPage, pageSize, keyword, filters) => {
      const listFilters: Record<string, string> = { ...filters };
      if (keyword.trim()) listFilters.keyword = keyword.trim();
      return resourceApi.list(currentPage, pageSize, listFilters);
    },
    getItems: (response) => response.resources,
  });

  /** 自定义筛选写入 URL（翻页保留），并回到第一页 */
  const setFilter = useCallback(
    (key: string, value?: string) => {
      const nextParams = new URLSearchParams(searchParams);
      if (!value) nextParams.delete(key);
      else nextParams.set(key, value);
      nextParams.set('current_page', '1');
      setSearchParams(nextParams, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  useEffect(() => {
    applicationApi.list(1, 1000).then((response) => {
      setApplications(response.applications.map((app) => ({ id: String(app.id), name: app.name })));
    });
  }, []);

  // 未选择应用时回退到当前上下文应用或第一个应用
  useEffect(() => {
    if (applicationId || applications.length === 0) return;
    const fallback =
      applications.find((app) => app.id === currentApplicationId)?.id ?? applications[0].id;
    setFilter('application_id', fallback);
  }, [applicationId, applications, currentApplicationId, setFilter]);

  // 父级选项与父级名称回显数据源：应用维度的全量资源树（增删改后随 treeVersion 刷新）
  useEffect(() => {
    if (!applicationId) {
      setTreeRoots([]);
      return;
    }
    let cancelled = false;
    resourceApi
      .listTree(applicationId)
      .then((response) => {
        if (!cancelled) setTreeRoots(response.roots);
      })
      .catch(() => {
        // 树加载失败不阻断列表展示，父级选项提交时服务端校验兜底
      });
    return () => {
      cancelled = true;
    };
  }, [applicationId, treeVersion]);

  const reloadTree = useCallback(() => setTreeVersion((version) => version + 1), []);

  const resourceById = useMemo(() => {
    const map: Record<string, Resource> = {};
    const walk = (nodes: ResourceTreeNode[]) => {
      for (const node of nodes) {
        map[node.resource.id] = node.resource;
        walk(node.children);
      }
    };
    walk(treeRoots);
    return map;
  }, [treeRoots]);

  const handleDelete = useCallback(
    (record: Resource) => {
      confirmDelete({
        entityType: '资源',
        entityName: record.name,
        onDelete: async () => {
          await resourceApi.delete(record.id);
          await tableQuery.refresh();
          reloadTree();
          message.success(buildSuccessMessage(ACTIONS.delete));
        },
        onError: (error) => showNotification(error, '删除资源失败'),
      });
    },
    [confirmDelete, tableQuery, reloadTree, message, showNotification],
  );

  const handleCreateSuccess = useCallback(() => {
    setCreateModalOpen(false);
    message.success(buildSuccessMessage(ACTIONS.create));
    void tableQuery.refresh();
    reloadTree();
  }, [message, tableQuery, reloadTree]);

  const handleEditSuccess = useCallback(() => {
    setEditingRecord(null);
    message.success(buildSuccessMessage(ACTIONS.save));
    void tableQuery.refresh();
    reloadTree();
  }, [message, tableQuery, reloadTree]);

  const columns = useMemo<TableColumnsType<Resource>>(() => [
    {
      title: '类型',
      dataIndex: 'kind',
      key: 'kind',
      width: 110,
      render: (kind: PermissionKind) => (
        <Tag color={KIND_TAG_COLORS[kind]}>{PERMISSION_KIND_LABELS[kind] ?? kind}</Tag>
      ),
    },
    { title: '资源键', dataIndex: 'key', key: 'key', ellipsis: true },
    { title: '资源名称', dataIndex: 'name', key: 'name', ellipsis: true },
    {
      title: '父资源',
      dataIndex: 'parentId',
      key: 'parentId',
      width: 200,
      ellipsis: true,
      render: (parentId: string | undefined) => {
        if (!parentId) return '—';
        const parent = resourceById[parentId];
        if (!parent) return parentId;
        return (
          <Space>
            <Tag color={KIND_TAG_COLORS[parent.kind]}>{PERMISSION_KIND_LABELS[parent.kind] ?? parent.kind}</Tag>
            <span>{parent.name}</span>
          </Space>
        );
      },
    },
    { title: '排序值', dataIndex: 'displayOrder', key: 'displayOrder', width: 90 },
    {
      title: '状态',
      dataIndex: 'enabledStatus',
      key: 'enabledStatus',
      width: 90,
      render: (status: string) => status === 'enabled'
        ? <Tag icon={<CheckCircleOutlined />}>启用</Tag>
        : <Tag icon={<StopOutlined />}>禁用</Tag>,
    },
    { title: '更新时间', dataIndex: 'updateTime', key: 'updateTime', width: 170 },
    {
      title: '操作',
      key: 'action',
      width: 140,
      render: (_: unknown, record: Resource) => (
        <Flex gap={0}>
          <Button type="link" onClick={() => setEditingRecord(record)}>
            {ACTIONS.edit}
          </Button>
          <Button type="link" danger onClick={() => handleDelete(record)}>
            {ACTIONS.delete}
          </Button>
        </Flex>
      ),
    },
  ], [resourceById, handleDelete]);

  return (
    <>
      <ListPageShell<Resource>
        columns={columns}
        tableQuery={tableQuery}
        searchPlaceholder="搜索资源键或名称"
        keyword={searchParams.get('keyword') ?? ''}
        onSearch={tableQuery.setKeyword}
        emptyDescription="暂无资源"
        errorDescription="获取资源列表失败"
        createLabel={applicationId ? `${ACTIONS.create}资源` : undefined}
        onCreate={() => setCreateModalOpen(true)}
        extraActions={(
          <Flex gap={16}>
            <Select
              placeholder="选择应用"
              allowClear
              showSearch
              optionFilterProp="label"
              style={{ width: 220 }}
              value={applicationId}
              onChange={(value) => setFilter('application_id', value ?? undefined)}
              options={applications.map((app) => ({ label: app.name, value: app.id }))}
            />
            <Select
              placeholder="类型"
              allowClear
              style={{ width: 140 }}
              value={kindFilter}
              onChange={(value) => setFilter('kind', value)}
              options={KIND_OPTIONS}
            />
          </Flex>
        )}
      />
      <CreateResourceModal
        open={createModalOpen}
        applicationId={applicationId}
        treeRoots={treeRoots}
        onClose={() => setCreateModalOpen(false)}
        onSuccess={handleCreateSuccess}
        onError={(error) => showNotification(error, '创建资源失败')}
      />
      <EditResourceModal
        record={editingRecord}
        treeRoots={treeRoots}
        onClose={() => setEditingRecord(null)}
        onSuccess={handleEditSuccess}
        onError={(error) => showNotification(error, '保存资源失败')}
      />
    </>
  );
}