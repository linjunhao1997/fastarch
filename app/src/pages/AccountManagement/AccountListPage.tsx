import { useState, useEffect } from "react";
import { Flex, Input, Button, Table, Modal, Select, Tag, message } from "antd";
import { useNavigate } from "react-router";
import { accountApi } from "../../api/iam";
import { roleApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import type { Account } from "../../api/iam";

const { Search } = Input;

export default function AccountListPage() {
  const navigate = useNavigate();
  const [users, setUsers] = useState<Account[]>([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: "" });
  const [roleOptions, setRoleOptions] = useState<{ label: string; value: string }[]>([]);
  const [selectedRoleCodes, setSelectedRoleCodes] = useState<string[]>([]);
  const showNotification = useApiErrorNotification();

  const fetchAccounts = async () => {
    setLoading(true);
    try {
      const response = await accountApi.list(1, 100);
      setUsers(response.accounts || []);
    } catch (error) {
      showNotification(error, "获取账号列表失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAccounts();
  }, [showNotification]);

  const handleSearch = (value: string) => {
    setFilters({ name: value });
  };

  const handleReset = () => {
    setFilters({ name: "" });
  };

  const handleCreateAccount = () => {
    navigate('/account_management/create_account');
  };

  const handleBindRoles = (account: Account) => {
    setSelectedRoleCodes(account.roleCodeList || []);
    Modal.confirm({
      title: "绑定角色",
      content: (
        <div>
          <Select
            mode="multiple"
            style={{ width: "100%" }}
            options={roleOptions}
            value={selectedRoleCodes}
            onChange={setSelectedRoleCodes}
          />
          <p style={{ marginTop: 12 }}>保存后权限变更立即生效，无需重新登录。</p>
        </div>
      ),
      onOk: async () => {
        const previous = new Set(account.roleCodeList || []);
        const selected = new Set(selectedRoleCodes);
        const addedRoleCodeList = [...selected].filter(code => !previous.has(code));
        const removedRoleCodeList = [...previous].filter(code => !selected.has(code));
        try {
          await accountApi.updateRoles(account.id, { addedRoleCodeList, removedRoleCodeList });
          message.success("权限变更已生效");
          await fetchAccounts();
        } catch (error) {
          showNotification(error, "绑定角色失败");
          throw error;
        }
      },
    });
  };

  const handleDeleteAccount = (account: Account) => {
    Modal.confirm({
      title: "删除账号",
      content: `确定删除账号 ${account.name} 吗？`,
      onOk: async () => {
        try {
          await accountApi.delete(account.id);
          await fetchAccounts();
        } catch (error) {
          showNotification(error, "删除账号失败");
          throw error;
        }
      },
    });
  };

  useEffect(() => {
    const fetchRoles = async () => {
      try {
        const response = await roleApi.list(1, 100);
        setRoleOptions((response.roles || []).map(role => ({ label: role.name, value: role.roleCode })));
      } catch (error) {
        showNotification(error, "获取角色列表失败");
      }
    };
    fetchRoles();
  }, [showNotification]);

  const handleEditAccount = (userId: string) => {
    navigate(`/account_management/edit_account/${userId}`);
  };

  const filteredUsers = users.filter((user) =>
    !filters.name || user.name?.includes(filters.name)
  );

  const auditColumns = [
    { title: "创建人", dataIndex: "createdBy", key: "createdBy", width: 90 },
    { title: "创建时间", dataIndex: "createTime", key: "createTime", width: 170, render: (text: string) => text || "—" },
    { title: "修改人", dataIndex: "updatedBy", key: "updatedBy", width: 90 },
    { title: "修改时间", dataIndex: "updateTime", key: "updateTime", width: 170, render: (text: string) => text || "—" },
  ];

  const columns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "姓名", dataIndex: "name", key: "name" },
    { title: "邮箱", dataIndex: "email", key: "email" },
    { title: "类型", dataIndex: "kind", key: "kind", render: (text: string) => text === "0" ? "用户" : "系统" },
    {
      title: "绑定角色",
      dataIndex: "roleCodeList",
      key: "roleCodeList",
      render: (roleCodeList: string[] | undefined) =>
        roleCodeList && roleCodeList.length > 0
          ? <Flex gap={4} wrap>{roleCodeList.map(code => <Tag key={code}>{code}</Tag>)}</Flex>
          : "—",
    },
    ...auditColumns,
    {
      title: "操作",
      key: "action",
      render: (_: unknown, record: Account) => (
        <Flex gap={0}>
          <Button type="link" onClick={() => handleEditAccount(record.id)}>编辑</Button>
          <Button type="link" onClick={() => handleBindRoles(record)}>绑定角色</Button>
          <Button type="link" danger onClick={() => handleDeleteAccount(record)}>删除</Button>
        </Flex>
      ),
    },
  ];

  return (
    <div>
      <Flex gap={16} style={{ marginBottom: 16 }}>
        <Search placeholder="搜索姓名" onSearch={handleSearch} style={{ width: 200 }} />
        <Button onClick={handleReset}>重置</Button>
        <Button type="primary" onClick={handleCreateAccount}>创建账号</Button>
      </Flex>
      <Table
        columns={columns}
        dataSource={filteredUsers}
        rowKey="id"
        loading={loading}
      />
    </div>
  );
}
