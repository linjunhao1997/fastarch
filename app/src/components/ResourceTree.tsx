import { useEffect, useState, useCallback } from 'react';
import { App, Tree, Button, Space, Form, Input, Radio, TreeSelect, Tag, Modal, Popconfirm } from 'antd';
import type { TreeDataNode } from 'antd';
import { resourceApi } from '../api/iam';
import type { CreateResourceRequest, PermissionKind, Resource, UpdateResourceRequest } from '../types';
import { PERMISSION_KIND_LABELS } from '../types';

const KIND_OPTIONS: { label: string; value: PermissionKind }[] = [
  { label: PERMISSION_KIND_LABELS.menu, value: 'menu' },
  { label: PERMISSION_KIND_LABELS.http_api, value: 'http_api' },
  { label: PERMISSION_KIND_LABELS.grpc, value: 'grpc' },
];

const KIND_COLORS: Record<PermissionKind, string> = {
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

interface ResourceTreeProps {
  applicationId: string;
}

// Session-persistent expand state (resets on new session per spec clarification)
function useSessionExpandKeys(key: string) {
  const getInitial = (): string[] => {
    try {
      return JSON.parse(sessionStorage.getItem(key) || '[]');
    } catch {
      return [];
    }
  };
  const [expandedKeys, setExpandedKeysState] = useState<string[]>(getInitial);
  const setExpandedKeys = useCallback((keys: React.Key[]) => {
    const strKeys = keys.map(String);
    setExpandedKeysState(strKeys);
    sessionStorage.setItem(key, JSON.stringify(strKeys));
  }, [key]);
  return [expandedKeys, setExpandedKeys] as const;
}

export default function ResourceTree({ applicationId }: ResourceTreeProps) {
  // 静态 message 在 React 19 下不渲染（ReactDOM.render 已移除），统一用 App 上下文实例
  const { message } = App.useApp();
  const [treeData, setTreeData] = useState<TreeDataNode[]>([]);
  const [allNodes, setAllNodes] = useState<Resource[]>([]);
  const [expandedKeys, setExpandedKeys] = useSessionExpandKeys(`resource-tree-expand-${applicationId}`);
  const [selectedId, setSelectedId] = useState<string | undefined>();
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [editingResource, setEditingResource] = useState<Resource | undefined>();
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();
  const createKind = Form.useWatch('kind', createForm);

  const loadTree = () => {
    if (!applicationId) {
      setTreeData([]);
      setAllNodes([]);
      return;
    }
    resourceApi.listTree(applicationId).then((response) => {
      const nodes: Resource[] = [];
      const collectNodes = (roots: typeof response.roots) => {
        for (const node of roots) {
          nodes.push(node.resource);
          collectNodes(node.children);
        }
      };
      collectNodes(response.roots);
      setAllNodes(nodes);

      const ids: React.Key[] = [];
      const convert = (roots: typeof response.roots): TreeDataNode[] =>
        roots.map((node) => {
          ids.push(node.resource.id);
          return {
            key: node.resource.id,
            title: node.resource.name,
            children: convert(node.children),
          };
        });
      setTreeData(convert(response.roots));
      // Auto-expand all on first load
      if (ids.length > 0 && expandedKeys.length === 0) {
        setExpandedKeys(ids);
      }
    });
  };

  useEffect(() => {
    loadTree();
    // Reset expand state when application changes
    setExpandedKeys([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [applicationId]);


  const handleCreate = async (values: CreateResourceRequest & { parentId?: string }) => {
    await resourceApi.create({
      ...values,
      applicationId: applicationId,
      parentId: values.parentId || undefined,
    });
    message.success('创建资源成功');
    setIsCreateModalOpen(false);
    createForm.resetFields();
    loadTree();
  };

  const handleEdit = async (values: UpdateResourceRequest & { parentId?: string }) => {
    if (!editingResource) return;
    await resourceApi.update(String(editingResource.id), {
      ...values,
      parentId: values.parentId || undefined,
      parentCleared: values.parentId === undefined && editingResource.parentId !== undefined,
      version: editingResource.version,
    });
    message.success('更新资源成功');
    setIsEditModalOpen(false);
    setEditingResource(undefined);
    editForm.resetFields();
    loadTree();
  };

  const handleDelete = async (resource: Resource) => {
    try {
      await resourceApi.delete(String(resource.id));
      message.success('删除资源成功');
      if (selectedId === String(resource.id)) setSelectedId(undefined);
      loadTree();
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '删除资源失败');
    }
  };

  const openEdit = async () => {
    if (!selectedId) return;
    const resource = await resourceApi.get(String(selectedId));
    setEditingResource(resource);
    editForm.setFieldsValue({
      name: resource.name,
      rpcFullMethod: resource.rpcFullMethod,
      disabled: resource.enabledStatus === 'disabled',
      parentId: resource.parentId ? String(resource.parentId) : undefined,
      displayOrder: resource.displayOrder,
    });
    setIsEditModalOpen(true);
  };

  // 为树节点添加颜色标签
  const addColorTag = (nodes: TreeDataNode[]): TreeDataNode[] => {
    return nodes.map((node) => {
      const nodeKey = String(node.key ?? '');
      const resource = allNodes.find((n) => n.id === nodeKey);
      const baseTitle = typeof node.title === 'function' ? nodeKey : node.title;
      return {
        ...node,
        title: resource ? (
          <Space>
            {baseTitle}
            <Tag color={KIND_COLORS[resource.kind]}>{PERMISSION_KIND_LABELS[resource.kind]}</Tag>
            {resource.enabledStatus === 'disabled' && <Tag color="red">禁用</Tag>}
            <Popconfirm
              title="删除资源"
              description="确认删除该资源吗？存在子资源或被权限引用时将无法删除。"
              okText="确认"
              cancelText="取消"
              onConfirm={() => handleDelete(resource)}
            >
              {/* 阻止冒泡：点击删除不触发树节点选中 */}
              <Button type="link" danger size="small" style={{ padding: 0 }} onClick={(e) => e.stopPropagation()}>
                删除
              </Button>
            </Popconfirm>
          </Space>
        ) : baseTitle,
        children: node.children ? addColorTag(node.children as TreeDataNode[]) : undefined,
      };
    });
  };

  // 过滤树中符合给定类型的节点（用于父级选择）
  const filterTreeWithFlatten = (nodes: TreeDataNode[], allowedKinds: PermissionKind[]): TreeDataNode[] => {
    if (allowedKinds.length === 0) return [];
    const result: TreeDataNode[] = [];
    for (const node of nodes) {
      const resource = allNodes.find((n) => n.id === String(node.key));
      if (resource && allowedKinds.includes(resource.kind)) {
        result.push({
          ...node,
          title: resource.name,
          children: node.children ? filterTreeWithFlatten(node.children as TreeDataNode[], allowedKinds) : undefined,
        });
      } else if (node.children) {
        // 不符合类型但有子节点，将子节点提升到当前位置
        result.push(...filterTreeWithFlatten(node.children as TreeDataNode[], allowedKinds));
      }
    }
    return result;
  };

  // 创建窗口的父级选项
  const createParentTreeData = createKind
    ? filterTreeWithFlatten(treeData, ALLOWED_PARENT_KINDS[createKind as PermissionKind] || [])
    : treeData;

  // 编辑窗口的父级选项
  const editParentTreeData = editingResource
    ? filterTreeWithFlatten(treeData, ALLOWED_PARENT_KINDS[editingResource.kind] || [])
    : treeData;

  // 主树展示（带颜色标签）
  const treeDataWithTags = addColorTag(treeData);

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setIsCreateModalOpen(true)}>新增资源</Button>
        <Button onClick={openEdit} disabled={!selectedId}>编辑</Button>
      </Space>
      <Tree
        treeData={treeDataWithTags}
        expandedKeys={expandedKeys}
        onExpand={setExpandedKeys}
        onSelect={([key]) => {
          const nodeKey = String(key);
          const resource = allNodes.find((n) => n.id === nodeKey);
          if (!resource) return;
          // 仅记录选中项（启用工具栏 编辑/删除 按钮），编辑弹窗由"编辑"按钮触发
          setSelectedId(nodeKey);
        }}
        selectedKeys={selectedId ? [selectedId] : []}
      />
      <Modal
        title="新增资源"
        open={isCreateModalOpen}
        onCancel={() => { setIsCreateModalOpen(false); createForm.resetFields(); }}
        onOk={() => createForm.submit()}
      >
        <Form form={createForm} layout="vertical" onFinish={handleCreate}>
          <Form.Item label="父级资源" name="parentId">
            <TreeSelect
              treeData={createParentTreeData}
              placeholder="留空为根节点"
              allowClear
              treeDefaultExpandAll
              showSearch
              treeNodeFilterProp="label"
              style={{ width: '100%' }}
            />
          </Form.Item>
          <Form.Item label="资源类型" name="kind" rules={[{ required: true }]}>
            <Radio.Group options={KIND_OPTIONS} optionType="button" buttonStyle="solid" />
          </Form.Item>
          <Form.Item label="资源键" name="key" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item label="资源名称" name="name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item label="RPC 目标方法" name="rpcFullMethod">
            <Input />
          </Form.Item>
          <Form.Item label="排序值" name="displayOrder" extra="数值越小排序越靠前">
            <Input type="number" placeholder="0" />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title="编辑资源"
        open={isEditModalOpen}
        onCancel={() => { setIsEditModalOpen(false); setEditingResource(undefined); editForm.resetFields(); }}
        onOk={() => editForm.submit()}
      >
        <Form form={editForm} layout="vertical" onFinish={handleEdit}>
          <Form.Item label="父级资源" name="parentId">
            <TreeSelect
              treeData={editParentTreeData}
              placeholder="留空为根节点"
              allowClear
              treeDefaultExpandAll
              showSearch
              treeNodeFilterProp="label"
              style={{ width: '100%' }}
            />
          </Form.Item>
          <Form.Item label="资源名称" name="name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item label="RPC 目标方法" name="rpcFullMethod">
            <Input />
          </Form.Item>
          <Form.Item label="排序值" name="displayOrder" extra="数值越小排序越靠前">
            <Input type="number" />
          </Form.Item>
          <Form.Item label="禁用" name="disabled" valuePropName="checked">
            <input type="checkbox" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}