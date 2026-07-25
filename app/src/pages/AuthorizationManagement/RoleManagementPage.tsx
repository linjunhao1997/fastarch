import React, { useState, useEffect } from "react";
import { Flex, Input, Button, Table, message } from "antd";
import { useNavigate } from "react-router";
import { roleApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

const { Search } = Input;

export default function RoleManagementPage() {
  const navigate = useNavigate();
  const [roles, setRoles] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: "" });
  const showNotification = useApiErrorNotification();

  const fetchRoles = async () => {
    setLoading(true);
    try {
      const response = await roleApi.list(1, 100);
      if (response.status.code === "0000") {
        setRoles(response.roles || []);
      }
    } catch (error) {
      showNotification(error, "获取角色列表失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRoles();
  }, [showNotification]);

  const handleSearch = (value: string) => {
    setFilters({ name: value });
  };

  const handleReset = () => {
    setFilters({ name: "" });
  };

  const handleCreateRole = () => {
    navigate('/authorization_management/role_create');
  };

  const filteredRoles = roles.filter((role) =>
    !filters.name || role.name?.includes(filters.name)
  );

  const columns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "角色名称", dataIndex: "name", key: "name" },
    { title: "角色编码", dataIndex: "role_code", key: "role_code" },
    { title: "所属应用", dataIndex: "application_id", key: "application_id" },
    { title: "状态", dataIndex: "enabled_status", key: "enabled_status" },
  ];

  return (
    <div>
      <Flex gap={16} style={{ marginBottom: 16 }}>
        <Search placeholder="搜索角色名称" onSearch={handleSearch} style={{ width: 200 }} />
        <Button onClick={handleReset}>重置</Button>
        <Button type="primary" onClick={handleCreateRole}>创建角色</Button>
      </Flex>
      <Table
        columns={columns}
        dataSource={filteredRoles}
        rowKey="id"
        loading={loading}
      />
    </div>
  );
}