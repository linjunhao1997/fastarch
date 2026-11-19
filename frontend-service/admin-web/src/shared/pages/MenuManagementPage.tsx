import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Button, Card, Flex, Form, Input, InputNumber, Modal, Radio, Select, Space, Table, Tag, message,
} from 'antd';
import type { TableColumnsType } from 'antd';
import { CheckCircleOutlined, PlusOutlined, StopOutlined } from '@ant-design/icons';
import { menuApi } from '@/shared/api/menu-dict-api';
import { getConsole } from '@/shared/api/app-context';
import { permissionApi } from '@/iam/api/iam';
import { useConfirmDelete } from '@/shared/components/ConfirmDelete';
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { usePermissions } from '@/shared/hooks/permission-context';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import { MENU_ICONS } from '@/app/router';
import type { Menu, MenuTreeNode } from '@/shared/types';
import { ACTIONS, buildSuccessMessage } from '@/shared/types/copywriting';
import { MENU_MODULE_CODES } from '@/shared/types/permissions';

const ICON_OPTIONS = Object.keys(MENU_ICONS).map((name) => ({
  value: name,
  label: <Space>{MENU_ICONS[name]}<span>{name}</span></Space>,
}));

interface MenuFormValues {
  parentId: string;
  name: string;
  path?: string;
  icon?: string;
  sort?: number;
  permissionCode?: string;
  status: string;
}

/** 树节点 → Table 行（children 供展开渲染） */
interface MenuRow extends Menu {
  children?: MenuRow[];
}

const toMenuRows = (nodes: MenuTreeNode[]): MenuRow[] =>
  nodes.map((node) => ({
    ...node.menu,
    children: node.children.length > 0 ? toMenuRows(node.children) : undefined,
  }));

interface EditingState {
  mode: 'create' | 'edit';
  /** create 时为父节点 id（'0' = 顶级）；edit 时为被编辑菜单自身 */
  parent?: Menu;
  menu?: Menu;
}

export default function MenuManagementPage() {
  const showNotification = useApiErrorNotification();
  const queryClient = useQueryClient();
  const confirmDelete = useConfirmDelete();
  const { hasPermission } = usePermissions();
  // 菜单码按控制台选择：iam 控制台用 iam.menu.*，sender 控制台用 sender.menu.*
  const menuCodes = MENU_MODULE_CODES[getConsole()];
  const canCreate = hasPermission(menuCodes.create);
  const canUpdate = hasPermission(menuCodes.update);
  const canDelete = hasPermission(menuCodes.delete);
  const [form] = Form.useForm<MenuFormValues>();
  const [editing, setEditing] = useState<EditingState | null>(null);
  const { isSubmitting, run } = useSubmitLock();

  const treeQuery = useQuery({
    // 缓存按控制台隔离（#9 + ADR 0007）：服务端按登录锁定应用隔离数据
    queryKey: ['menu-tree', getConsole()],
    queryFn: () => menuApi.listTree(),
  });
  const permissionQuery = useQuery({
    queryKey: ['permissions', 'all', getConsole()],
    queryFn: () => permissionApi.list(1, 500),
  });

  const permissionCodeOptions = useMemo(
    () => (permissionQuery.data?.permissions ?? []).map((p) => ({ value: p.permissionCode })),
    [permissionQuery.data],
  );

  /** 树展平：id → menu（父节点校验、回编辑缓存用） */
  const menuById = useMemo(() => {
    const map = new Map<string, Menu>();
    const walk = (nodes: MenuTreeNode[]) => {
      for (const n of nodes) {
        map.set(n.menu.id, n.menu);
        walk(n.children);
      }
    };
    walk(treeQuery.data?.roots ?? []);
    return map;
  }, [treeQuery.data]);

  const saveMutation = useMutation({
    mutationFn: async (values: MenuFormValues) => {
      if (editing?.mode === 'edit' && editing.menu) {
        const menu = editing.menu;
        const parentChanged = values.parentId !== menu.parentId;
        await menuApi.update(menu.id, {
          name: values.name,
          path: values.path,
          icon: values.icon,
          sort: values.sort ?? 0,
          permissionCode: values.permissionCode,
          status: values.status,
          parentId: values.parentId,
          parentCleared: parentChanged,
          version: menu.version,
        });
      } else {
        await menuApi.create({
          parentId: values.parentId || '0',
          name: values.name,
          path: values.path,
          icon: values.icon,
          sort: values.sort ?? 0,
          permissionCode: values.permissionCode,
          status: values.status,
        });
      }
    },
    onSuccess: () => {
      message.success(buildSuccessMessage(editing?.mode === 'edit' ? ACTIONS.save : ACTIONS.create));
      setEditing(null);
      form.resetFields();
      void queryClient.invalidateQueries({ queryKey: ['menu-tree'] });
    },
    onError: (error) => showNotification(error, '菜单保存失败'),
  });

  const openModal = (state: EditingState) => {
    setEditing(state);
    if (state.mode === 'edit' && state.menu) {
      const menu = state.menu;
      form.setFieldsValue({
        parentId: menu.parentId || '0',
        name: menu.name,
        path: menu.path,
        icon: menu.icon,
        sort: menu.sort,
        permissionCode: menu.permissionCode || undefined,
        status: menu.status,
      });
    } else {
      form.resetFields();
      form.setFieldsValue({ parentId: state.parent?.id ?? '0', status: 'enabled', sort: 0 });
    }
  };

  /** 父节点下拉：排除自身及其后代（防成环） */
  const parentOptions = useMemo(() => {
    const options: { value: string; label: string }[] = [{ value: '0', label: '顶级' }];
    const exclude = new Set<string>();
    if (editing?.mode === 'edit' && editing.menu) {
      const collectSubtree = (id: string) => {
        exclude.add(id);
        for (const m of menuById.values()) {
          if (m.parentId === id) collectSubtree(m.id);
        }
      };
      collectSubtree(editing.menu.id);
    }
    for (const menu of menuById.values()) {
      if (!exclude.has(menu.id)) options.push({ value: menu.id, label: menu.name });
    }
    return options;
  }, [editing, menuById]);

  const columns: TableColumnsType<MenuRow> = useMemo(() => [
    { title: '菜单名称', dataIndex: 'name', key: 'name' },
    {
      title: '图标', dataIndex: 'icon', key: 'icon', width: 80,
      render: (icon: string) => MENU_ICONS[icon] ?? '—',
    },
    { title: '路由路径', dataIndex: 'path', key: 'path', render: (value: string) => value || '—' },
    { title: '排序', dataIndex: 'sort', key: 'sort', width: 80 },
    {
      title: '展示所需码', dataIndex: 'permissionCode', key: 'permissionCode',
      render: (code: string) => code ? <Tag>{code}</Tag> : '—',
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 100,
      render: (status: string) => status === 'enabled'
        ? <Tag icon={<CheckCircleOutlined />}>启用</Tag>
        : <Tag icon={<StopOutlined />}>禁用</Tag>,
    },
    {
      title: '操作', key: 'action', width: 220,
      render: (_: unknown, record: MenuRow) => (
        <Flex gap={0}>
          {canCreate ? (
            <Button type="link" size="small" onClick={() => openModal({ mode: 'create', parent: record })}>
              新建子项
            </Button>
          ) : null}
          {canUpdate ? (
            <Button type="link" size="small" onClick={() => openModal({ mode: 'edit', menu: record })}>
              {ACTIONS.edit}
            </Button>
          ) : null}
          {canDelete ? (
            <Button
              type="link"
              size="small"
              danger
              onClick={() => confirmDelete({
                entityType: '菜单',
                entityName: record.name,
                onDelete: async () => {
                  await menuApi.delete(record.id);
                  void queryClient.invalidateQueries({ queryKey: ['menu-tree'] });
                  message.success(buildSuccessMessage(ACTIONS.delete));
                },
                onError: (error) => showNotification(error, '删除菜单失败'),
              })}
            >
              {ACTIONS.delete}
            </Button>
          ) : null}
        </Flex>
      ),
    },
  ], [canCreate, canDelete, canUpdate, confirmDelete, menuById, queryClient, showNotification]);

  return (
    <div>
      <Card
        title="菜单管理"
        extra={canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal({ mode: 'create' })}>新建顶级菜单</Button>
        ) : undefined}
      >
        <Table<MenuRow>
          rowKey="id"
          columns={columns}
          dataSource={treeQuery.data ? toMenuRows(treeQuery.data.roots) : []}
          loading={treeQuery.isPending}
          pagination={false}
          expandable={{ defaultExpandAllRows: true }}
          scroll={{ x: 900 }}
        />
      </Card>

      <Modal
        title={editing?.mode === 'edit' ? `编辑菜单：${editing?.menu?.name ?? ''}` : '新建菜单'}
        open={editing !== null}
        onCancel={() => setEditing(null)}
        onOk={() => form.submit()}
        confirmLoading={isSubmitting || saveMutation.isPending}
        okText={ACTIONS.confirm}
        cancelText={ACTIONS.cancel}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={(values) => run(() => saveMutation.mutateAsync(values))}>
          <Form.Item label="父菜单" name="parentId" rules={[{ required: true, message: '请选择父菜单' }]}>
            <Select options={parentOptions} showSearch optionFilterProp="label" />
          </Form.Item>
          <Form.Item label="菜单名称" name="name" rules={[{ required: true, message: '请输入菜单名称' }]}>
            <Input placeholder="展示名称" />
          </Form.Item>
          <Form.Item label="路由路径" name="path" tooltip="前端路由绝对路径（如 /account_management/account_list）；无对应页面时自动隐藏入口">
            <Input placeholder="叶子菜单必填；分组节点可留空" />
          </Form.Item>
          <Form.Item label="图标" name="icon" tooltip="AntD 图标名">
            <Select options={ICON_OPTIONS} allowClear showSearch placeholder="选择图标" />
          </Form.Item>
          <Form.Item label="排序" name="sort" tooltip="同级内升序排列">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            label="展示所需权限码"
            name="permissionCode"
            tooltip="可空 = 绑定该菜单的角色均可见；设置后仅持有此码的用户可见该菜单。前端展示门槛，不授予权限（授权请在角色页直授码）"
          >
            <Select options={permissionCodeOptions} allowClear showSearch placeholder="展示所需权限码（可留空）" loading={permissionQuery.isPending} />
          </Form.Item>
          <Form.Item label="状态" name="status" rules={[{ required: true }]}>
            <Radio.Group>
              <Radio value="enabled">启用</Radio>
              <Radio value="disabled">禁用</Radio>
            </Radio.Group>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
