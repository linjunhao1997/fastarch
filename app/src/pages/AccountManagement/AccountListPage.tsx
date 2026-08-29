import React, { useState, useEffect } from "react";
import { Flex, Input, Button, Table, message } from "antd";
import { useNavigate } from "react-router";
import { accountApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

const { Search } = Input;

export default function AccountListPage() {
  const navigate = useNavigate();
  const [users, setUsers] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: "" });
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

  const handleEditAccount = (userId: number) => {
    navigate(`/account_management/edit_account/${userId}`);
  };

  const filteredUsers = users.filter((user) =>
    !filters.name || user.name?.includes(filters.name)
  );

  const columns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "姓名", dataIndex: "name", key: "name" },
    { title: "邮箱", dataIndex: "email", key: "email" },
    { title: "类型", dataIndex: "kindId", key: "kindId", render: (text: string) => text === "0" ? "用户" : "系统" },
    {
      title: "操作",
      key: "action",
      render: (_: any, record: any) => (
        <Button type="link" onClick={() => handleEditAccount(record.id)}>
          编辑
        </Button>
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