import { useMemo, useState } from 'react';
import { Button, Card, Flex, Input, Select, Table, Tag } from 'antd';
import { useNavigate } from 'react-router';
import { ACTIONS } from '../../types/copywriting';

// 演示数据，认证策略后端契约待后续特性。
interface AuthPolicy {
  id: string;
  type: string;
  name: string;
  implementationId: string;
}

const policies: AuthPolicy[] = [
  { id: '1', type: 'sso', name: '企业 SSO 策略', implementationId: 'corp-sso' },
  { id: '2', type: 'mfa', name: '管理员 MFA 策略', implementationId: 'totp' },
  { id: '3', type: 'otp', name: '一次性密码策略', implementationId: 'email-otp' },
];

const policyTypeNames: Record<string, string> = {
  sso: 'SSO',
  mfa: 'MFA',
  otp: 'OTP',
};

export default function AuthenticationPolicyListPage() {
  const navigate = useNavigate();
  const [keyword, setKeyword] = useState('');
  const [policyType, setPolicyType] = useState<string | undefined>();

  const filteredPolicies = useMemo(() => policies.filter((policy) =>
    (!policyType || policy.type === policyType) &&
    (!keyword || policy.name.includes(keyword) || policy.implementationId.includes(keyword))
  ), [keyword, policyType]);

  const columns = useMemo(() => [
    { title: 'ID', dataIndex: 'id', key: 'id' },
    {
      title: '策略类型',
      dataIndex: 'type',
      key: 'type',
      render: (type: string) => <Tag>{policyTypeNames[type] ?? type}</Tag>,
    },
    { title: '策略名称', dataIndex: 'name', key: 'name' },
    { title: '实现标识', dataIndex: 'implementationId', key: 'implementationId' },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: AuthPolicy) => (
        <Button
          type="link"
          onClick={() => navigate(`/authentication_management/edit_authentication_policy/${record.type}/${record.id}`)}
        >
          {ACTIONS.edit}
        </Button>
      ),
    },
  ], [navigate]);

  return (
    <Card
      title="认证策略"
      extra={(
        <Button type="primary" onClick={() => navigate('/authentication_management/create_authentication_policy/sso')}>
          {ACTIONS.create}策略
        </Button>
      )}
    >
      <Flex gap={16} style={{ marginBottom: 16 }}>
        <Input.Search placeholder="搜索策略名称或实现标识" onSearch={setKeyword} style={{ width: 240 }} />
        <Select
          placeholder="选择策略类型"
          allowClear
          style={{ width: 160 }}
          value={policyType}
          onChange={setPolicyType}
          options={Object.entries(policyTypeNames).map(([value, label]) => ({ value, label }))}
        />
      </Flex>
      <Table<AuthPolicy> columns={columns} dataSource={filteredPolicies} rowKey="id" />
    </Card>
  );
}
