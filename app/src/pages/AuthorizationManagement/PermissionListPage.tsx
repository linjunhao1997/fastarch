import React, { useState, useEffect } from "react";
import { Flex, Input, Button, Table, message } from "antd";
import { useNavigate } from "react-router";
import { permissionApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

const { Search } = Input;

export default function PermissionListPage() {
  const navigate = useNavigate();
  const [permissions, setPermissions] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: "" });
  const showNotification = useApiErrorNotification();

  const fetchPermissions = async () => {
    setLoading(true);
    try {
      const response = await permissionApi.list(1, 100);
      setPermissions(response.permissions || []);
    } catch (error) {
      showNotification(error, "获取权限列表失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPermissions();
  }, [showNotification]);

  const handleSearch = (value: string) => {
    setFilters({ name: value });
  };

  const handleReset = () => {
    setFilters({ name: "" });
  };

  const handleCreatePermission = () => {
    navigate('/authorization_management/permission_create');
  };

  const filteredPermissions = permissions.filter((permission) =>
    !filters.name || permission.name?.includes(filters.name)
  );

  const columns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "权限名称", dataIndex: "name", key: "name" },
    { title: "权限编码", dataIndex: "permissionCode", key: "permissionCode" },
    { title: "所属应用", dataIndex: "applicationId", key: "applicationId" },
    { title: "状态", dataIndex: "enabledStatus", key: "enabledStatus" },
  ];

  return (
    <div>
      <Flex gap={16} style={{ marginBottom: 16 }}>
        <Search placeholder="搜索权限名称" onSearch={handleSearch} style={{ width: 200 }} />
        <Button onClick={handleReset}>重置</Button>
        <Button type="primary" onClick={handleCreatePermission}>创建权限</Button>
      </Flex>
      <Table
        columns={columns}
        dataSource={filteredPermissions}
        rowKey="id"
        loading={loading}
      />
    </div>
  );
}