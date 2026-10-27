import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { App, Alert, Button, DatePicker, Flex, Form, Input, Modal, Select, Spin, Tag, Typography } from 'antd';
import type { TableColumnsType } from 'antd';
import { useSearchParams } from 'react-router-dom';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import { accessCredentialApi, accountApi } from '../../api/iam';
import { ListPageShell } from '../../components/ListPageShell';
import { useConfirmDelete } from '../../components/ConfirmDelete';
import { useApiErrorNotification } from '../../hooks/useApiError';
import { usePermissions } from '../../hooks/permission-context';
import { useSubmitLock } from '../../hooks/useSubmitLock';
import { useTableQuery } from '../../hooks/useTableQuery';
import type { AccessCredential, AccessCredentialStatus, Account } from '../../types';
import { ACTIONS, buildSuccessMessage } from '../../types/copywriting';
import { PERMISSIONS } from '../../types/permissions';

const TIME_FORMAT = 'YYYY-MM-DD HH:mm:ss';

/** ISO 时间格式化展示；空值显示占位符 */
const formatDateTime = (value: string | null | undefined): string => {
  if (!value) return '—';
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format(TIME_FORMAT) : value;
};

const STATUS_OPTIONS: { value: AccessCredentialStatus; label: string }[] = [
  { value: 'enabled', label: '启用' },
  { value: 'disabled', label: '禁用' },
];

/**
 * 拉取启用状态账号选项（凭证绑定用户下拉与筛选区共用，FR-004 仅启用账号可绑定）。
 * 单次批量拉取（pageSize=100），失败时保留空列表由调用方提示。
 */
function useEnabledAccountOptions() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    accountApi
      .list(1, 100)
      .then((response) => {
        if (!cancelled) setAccounts(response.accounts.filter((account) => account.status === 'enabled'));
      })
      .catch(() => {
        // 选项加载失败不阻断页面渲染，提交时服务端校验兜底
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return { accounts, loading };
}

/** 一次性 SK 展示状态（创建成功 / 重置成功共用） */
interface SecretRevealState {
  title: string;
  accessKey: string;
  secretKey: string;
}

/** 一次性 SK 展示弹窗：强制显式关闭（不可点遮罩/ESC 关闭），关闭后由父级刷新列表 */
function SecretRevealModal({ reveal, onClose }: { reveal: SecretRevealState | null; onClose: () => void }) {
  return (
    <Modal
      open={reveal !== null}
      title={reveal?.title ?? ''}
      onCancel={onClose}
      closable={false}
      maskClosable={false}
      keyboard={false}
      width={480}
      footer={(
        <Button type="primary" onClick={onClose}>我已保存，关闭</Button>
      )}
    >
      {reveal ? (
        <Flex vertical gap={12}>
          <Alert
            type="warning"
            showIcon
            message="Secret Key 仅展示一次，关闭后无法再查看，请立即复制并妥善保存"
          />
          <div>
            <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12, marginBottom: 4 }}>Access Key</div>
            <Typography.Text copyable style={{ fontSize: 16 }}>{reveal.accessKey}</Typography.Text>
          </div>
          <div>
            <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12, marginBottom: 4 }}>Secret Key</div>
            <Typography.Text copyable style={{ fontSize: 16 }}>{reveal.secretKey}</Typography.Text>
          </div>
        </Flex>
      ) : null}
    </Modal>
  );
}

interface CreateCredentialFormValues {
  accountId: string;
  remark?: string;
  expireTime?: Dayjs | null;
}

interface CreateCredentialModalProps {
  open: boolean;
  accounts: Account[];
  accountsLoading: boolean;
  onClose: () => void;
  /** 创建成功：父级展示一次性 SK 弹窗，用户显式关闭后刷新列表 */
  onCreated: (accessKey: string, secretKey: string) => void;
  onError: (error: unknown) => void;
}

function CreateCredentialModal({
  open,
  accounts,
  accountsLoading,
  onClose,
  onCreated,
  onError,
}: CreateCredentialModalProps) {
  const [form] = Form.useForm<CreateCredentialFormValues>();
  const { isSubmitting, run } = useSubmitLock();

  const handleFinish = (values: CreateCredentialFormValues) => {
    run(async () => {
      const response = await accessCredentialApi.create({
        accountId: values.accountId,
        remark: values.remark ?? '',
        expireTime: values.expireTime ? values.expireTime.toISOString() : null,
      });
      form.resetFields();
      onCreated(response.credential.accessKey, response.secretKey);
    }).catch(onError);
  };

  return (
    <Modal
      title="新建凭证"
      open={open}
      onCancel={onClose}
      onOk={() => form.submit()}
      confirmLoading={isSubmitting}
      okText={ACTIONS.create}
      cancelText={ACTIONS.cancel}
      destroyOnClose
      width={480}
    >
      <Form form={form} layout="vertical" onFinish={handleFinish}>
        <Form.Item
          label="绑定用户"
          name="accountId"
          rules={[{ required: true, message: '请选择绑定用户' }]}
        >
          <Select
            placeholder="请选择启用状态的账号"
            showSearch
            optionFilterProp="label"
            loading={accountsLoading}
            notFoundContent={accountsLoading ? <Spin size="small" /> : '暂无启用账号'}
            options={accounts.map((account) => ({
              value: account.id,
              label: `${account.name}（${account.email}）`,
            }))}
          />
        </Form.Item>
        <Form.Item
          label="备注"
          name="remark"
          rules={[{ max: 255, message: '备注最长 255 个字符' }]}
        >
          <Input.TextArea rows={3} maxLength={255} showCount placeholder="凭证用途说明（可选）" />
        </Form.Item>
        <Form.Item label="过期时间" name="expireTime" extra="留空表示永不过期">
          <DatePicker
            showTime
            style={{ width: '100%' }}
            placeholder="选择过期时间（可选）"
            disabledDate={(current) => current.isBefore(dayjs().startOf('day'))}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}

interface EditCredentialFormValues {
  remark?: string;
  expireTime?: Dayjs | null;
}

interface EditCredentialModalProps {
  record: AccessCredential | null;
  onClose: () => void;
  /** 保存成功：父级提示并刷新列表 */
  onSuccess: () => void;
  onError: (error: unknown) => void;
}

/** 编辑弹窗：仅 remark、expireTime（契约可改字段全集），携带 version 乐观锁 */
function EditCredentialModal({ record, onClose, onSuccess, onError }: EditCredentialModalProps) {
  const [form] = Form.useForm<EditCredentialFormValues>();
  const [version, setVersion] = useState(0);
  const [loading, setLoading] = useState(false);
  const { isSubmitting, run } = useSubmitLock();

  const credentialId = record?.id;

  useEffect(() => {
    if (!credentialId) return;
    let cancelled = false;
    setLoading(true);
    accessCredentialApi.get(credentialId)
      .then((credential) => {
        if (cancelled) return;
        form.setFieldsValue({
          remark: credential.remark,
          expireTime: credential.expireTime ? dayjs(credential.expireTime) : null,
        });
        setVersion(credential.version);
      })
      .catch((error) => {
        if (!cancelled) onError(error);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [credentialId, form, onError]);

  const handleFinish = (values: EditCredentialFormValues) => {
    if (!credentialId) return;
    run(async () => {
      await accessCredentialApi.update(credentialId, {
        remark: values.remark ?? '',
        expireTime: values.expireTime ? values.expireTime.toISOString() : null,
        version,
      });
      onSuccess();
      onClose();
    }).catch(onError);
  };

  return (
    <Modal
      title="编辑凭证"
      open={record !== null}
      onCancel={onClose}
      onOk={() => form.submit()}
      confirmLoading={isSubmitting}
      okButtonProps={{ disabled: loading }}
      okText={ACTIONS.save}
      cancelText={ACTIONS.cancel}
      destroyOnClose
      width={480}
    >
      {record ? (
        <>
          <div style={{ marginBottom: 12, color: 'rgba(0,0,0,0.45)', fontSize: 12 }}>
            Access Key：{record.accessKey}
          </div>
          <Form form={form} layout="vertical" onFinish={handleFinish}>
            <Form.Item
              label="备注"
              name="remark"
              rules={[{ max: 255, message: '备注最长 255 个字符' }]}
            >
              <Input.TextArea rows={3} maxLength={255} showCount placeholder="凭证用途说明" />
            </Form.Item>
            <Form.Item label="过期时间" name="expireTime" extra="清空表示永不过期">
              <DatePicker
                showTime
                style={{ width: '100%' }}
                disabledDate={(current) => current.isBefore(dayjs().startOf('day'))}
              />
            </Form.Item>
          </Form>
        </>
      ) : null}
    </Modal>
  );
}

export default function AccessCredentialListPage() {
  const { message, modal } = App.useApp();
  const showNotification = useApiErrorNotification();
  const confirmDelete = useConfirmDelete();
  const { hasHttpPermission } = usePermissions();
  const [searchParams, setSearchParams] = useSearchParams();
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [editingRecord, setEditingRecord] = useState<AccessCredential | null>(null);
  const [secretReveal, setSecretReveal] = useState<SecretRevealState | null>(null);
  const { accounts, loading: accountsLoading } = useEnabledAccountOptions();

  const canCreate = hasHttpPermission(PERMISSIONS.credentialCreate);
  const canUpdate = hasHttpPermission(PERMISSIONS.credentialUpdate);
  const canChangeStatus = hasHttpPermission(PERMISSIONS.credentialChangeStatus);
  const canResetSecret = hasHttpPermission(PERMISSIONS.credentialResetSecret);
  const canDelete = hasHttpPermission(PERMISSIONS.credentialDelete);

  const tableQuery = useTableQuery({
    domain: 'access-credentials',
    filterKeys: ['account_id', 'status'],
    fetcher: (currentPage, pageSize, keyword, filters) => {
      const listFilters: Record<string, string> = { ...filters };
      const accessKey = keyword.trim();
      if (accessKey) listFilters.access_key = accessKey;
      return accessCredentialApi.list(currentPage, pageSize, listFilters);
    },
    getItems: (response) => response.accessCredentials,
  });

  /** 自定义筛选写入 URL（翻页保留），并回到第一页 */
  const setFilter = useCallback((key: string, value?: string) => {
    const nextParams = new URLSearchParams(searchParams);
    if (!value) nextParams.delete(key);
    else nextParams.set(key, value);
    nextParams.set('current_page', '1');
    setSearchParams(nextParams, { replace: true });
  }, [searchParams, setSearchParams]);

  const accountIdFilter = searchParams.get('account_id') ?? undefined;
  const statusParam = searchParams.get('status');
  const statusFilter: AccessCredentialStatus | undefined =
    statusParam === 'enabled' || statusParam === 'disabled' ? statusParam : undefined;

  const handleCreated = useCallback((accessKey: string, secretKey: string) => {
    setCreateModalOpen(false);
    setSecretReveal({ title: '凭证创建成功', accessKey, secretKey });
  }, []);

  /** SK 弹窗显式关闭后再刷新列表（用户确认已保存明文 SK） */
  const handleSecretRevealClose = useCallback(() => {
    setSecretReveal(null);
    void tableQuery.refresh();
  }, [tableQuery]);

  const handleEditSuccess = useCallback(() => {
    message.success(buildSuccessMessage(ACTIONS.save));
    void tableQuery.refresh();
  }, [message, tableQuery]);

  const handleToggleStatus = useCallback((record: AccessCredential) => {
    const disabling = record.status === 'enabled';
    modal.confirm({
      title: disabling ? '禁用凭证' : '启用凭证',
      content: disabling
        ? `禁用后凭证 ${record.accessKey} 的所有调用将立即失败，确定禁用吗？`
        : `确定启用凭证 ${record.accessKey} 吗？启用后恢复调用。`,
      okButtonProps: disabling ? { danger: true } : undefined,
      onOk: async () => {
        try {
          await accessCredentialApi.changeStatus(record.id, {
            id: record.id,
            status: disabling ? 'disabled' : 'enabled',
          });
          await tableQuery.refresh();
          message.success(buildSuccessMessage(disabling ? '禁用' : '启用'));
        } catch (error) {
          showNotification(error, disabling ? '禁用凭证失败' : '启用凭证失败');
          throw error;
        }
      },
    });
  }, [modal, tableQuery, message, showNotification]);

  const handleResetSecret = useCallback((record: AccessCredential) => {
    modal.confirm({
      title: '重置 Secret Key',
      content: `确定重置凭证 ${record.accessKey} 的 SK 吗？重置后旧 SK 立即失效。`,
      onOk: async () => {
        try {
          const response = await accessCredentialApi.resetSecret(record.id);
          setSecretReveal({
            title: '重置 SK 成功',
            accessKey: response.credential.accessKey,
            secretKey: response.secretKey,
          });
        } catch (error) {
          showNotification(error, '重置 SK 失败');
          throw error;
        }
      },
    });
  }, [modal, showNotification]);

  const handleDelete = useCallback((record: AccessCredential) => {
    confirmDelete({
      entityType: '访问凭证',
      entityName: record.accessKey,
      onDelete: async () => {
        await accessCredentialApi.delete(record.id);
        await tableQuery.refresh();
        message.success(buildSuccessMessage(ACTIONS.delete));
      },
      onError: (error) => showNotification(error, '删除访问凭证失败'),
    });
  }, [confirmDelete, tableQuery, message, showNotification]);

  const columns = useMemo<TableColumnsType<AccessCredential>>(() => [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 80, hidden: true },
    {
      title: 'Access Key',
      dataIndex: 'accessKey',
      key: 'accessKey',
      width: 220,
      ellipsis: true,
      render: (accessKey: string) => (
        <Typography.Text copyable={{ text: accessKey }}>{accessKey}</Typography.Text>
      ),
    },
    { title: '绑定用户', dataIndex: 'accountName', key: 'accountName', width: 140, ellipsis: true },
    { title: '备注', dataIndex: 'remark', key: 'remark', ellipsis: true, render: (remark: string) => remark || '—' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (status: string) => status === 'disabled'
        ? <Tag color="error">禁用</Tag>
        : <Tag color="success">启用</Tag>,
    },
    {
      title: '过期时间',
      dataIndex: 'expireTime',
      key: 'expireTime',
      width: 160,
      render: (expireTime: string | null) => (expireTime ? formatDateTime(expireTime) : '永不过期'),
    },
    {
      title: '最近使用',
      dataIndex: 'lastUsedTime',
      key: 'lastUsedTime',
      width: 160,
      render: (lastUsedTime: string | null) => formatDateTime(lastUsedTime),
    },
    { title: '版本', dataIndex: 'version', key: 'version', width: 70 },
    { title: '创建人', dataIndex: 'createdBy', key: 'createdBy', width: 110, ellipsis: true },
    { title: '创建时间', dataIndex: 'createTime', key: 'createTime', width: 160 },
    { title: '修改人', dataIndex: 'updatedBy', key: 'updatedBy', width: 110, ellipsis: true },
    { title: '修改时间', dataIndex: 'updateTime', key: 'updateTime', width: 160 },
    {
      title: '操作',
      key: 'action',
      width: 250,
      fixed: 'right' as const,
      align: 'left' as const,
      render: (_: unknown, record: AccessCredential) => {
        const actions: ReactNode[] = [];
        if (canUpdate) {
          actions.push(
            <Button key="edit" type="link" onClick={() => setEditingRecord(record)}>
              {ACTIONS.edit}
            </Button>,
          );
        }
        if (canChangeStatus) {
          actions.push(
            record.status === 'enabled'
              ? <Button key="disable" type="link" danger onClick={() => handleToggleStatus(record)}>禁用</Button>
              : <Button key="enable" type="link" onClick={() => handleToggleStatus(record)}>启用</Button>,
          );
        }
        if (canResetSecret) {
          actions.push(
            <Button key="reset" type="link" onClick={() => handleResetSecret(record)}>重置 SK</Button>,
          );
        }
        if (canDelete) {
          actions.push(
            <Button key="delete" type="link" danger onClick={() => handleDelete(record)}>
              {ACTIONS.delete}
            </Button>,
          );
        }
        return actions.length > 0 ? <Flex gap={4} wrap="nowrap">{actions}</Flex> : '—';
      },
    },
  ], [canUpdate, canChangeStatus, canResetSecret, canDelete, handleToggleStatus, handleResetSecret, handleDelete]);

  return (
    <>
      <ListPageShell<AccessCredential>
        columns={columns}
        tableQuery={tableQuery}
        searchPlaceholder="搜索 Access Key"
        keyword={searchParams.get('keyword') ?? ''}
        onSearch={tableQuery.setKeyword}
        emptyDescription="暂无访问凭证，可创建凭证用于 AK/SK 签名调用"
        errorDescription="获取访问凭证列表失败"
        createLabel={canCreate ? `${ACTIONS.create}凭证` : undefined}
        onCreate={canCreate ? () => setCreateModalOpen(true) : undefined}
        extraActions={(
          <Flex gap={16}>
            <Select
              placeholder="绑定用户"
              allowClear
              showSearch
              optionFilterProp="label"
              style={{ width: 200 }}
              loading={accountsLoading}
              value={accountIdFilter}
              onChange={(value) => setFilter('account_id', value)}
              options={accounts.map((account) => ({
                value: account.id,
                label: account.name,
              }))}
            />
            <Select
              placeholder="状态"
              allowClear
              style={{ width: 120 }}
              value={statusFilter}
              onChange={(value) => setFilter('status', value)}
              options={STATUS_OPTIONS}
            />
          </Flex>
        )}
      />
      <CreateCredentialModal
        open={createModalOpen}
        accounts={accounts}
        accountsLoading={accountsLoading}
        onClose={() => setCreateModalOpen(false)}
        onCreated={handleCreated}
        onError={(error) => showNotification(error, '创建凭证失败')}
      />
      <EditCredentialModal
        record={editingRecord}
        onClose={() => setEditingRecord(null)}
        onSuccess={handleEditSuccess}
        onError={(error) => showNotification(error, '保存凭证失败')}
      />
      <SecretRevealModal reveal={secretReveal} onClose={handleSecretRevealClose} />
    </>
  );
}
