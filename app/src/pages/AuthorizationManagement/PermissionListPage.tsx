import { useState, useEffect } from "react";
import { Flex, Input, Button, Table, Modal } from "antd";
import { useNavigate } from "react-router";
import { permissionApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import type { Permission } from "../../api/iam";

const { Search } = Input;

export default function PermissionListPage() {
  const navigate = useNavigate();
  const [permissions, setPermissions] = useState<Permission[]>([]);
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

  const handleEditPermission = (permissionId: string) => {
    navigate(`/authorization_management/permission_edit/${permissionId}`);
  };

  const handleDeletePermission = (permission: Permission) => {
    Modal.confirm({
      title: "删除权限",
      content: `确定删除权限 ${permission.name} 吗？`,
      onOk: async () => {
        try {
          await permissionApi.delete(permission.id);
          await fetchPermissions();
        } catch (error) {
          showNotification(error, "删除权限失败");
          throw error;
        }
      },
    });
  };

  const filteredPermissions = permissions.filter((permission) =>
    !filters.name || permission.name?.includes(filters.name)
  );

  const auditColumns = [
    { title: "创建人", dataIndex: "createdBy", key: "createdBy", width: 90 },
    { title: "创建时间", dataIndex: "createTime", key: "createTime", width: 170, render: (text: string) => text || "—" },
    { title: "修改人", dataIndex: "updatedBy", key: "updatedBy", width: 90 },
    { title: "修改时间", dataIndex: "updateTime", key: "updateTime", width: 170, render: (text: string) => text || "—" },
  ];

  const columns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "权限名称", dataIndex: "name", key: "name" },
    { title: "权限编码", dataIndex: "permissionCode", key: "permissionCode" },
    { title: "所属应用", dataIndex: "applicationId", key: "applicationId" },
    { title: "状态", dataIndex: "enabledStatus", key: "enabledStatus" },
    ...auditColumns,
    {
      title: "操作",
      key: "action",
      render: (_: unknown, record: Permission) => (
        <Flex gap={0}>
          <Button type="link" onClick={() => handleEditPermission(record.id)}>编辑</Button>
          <Button type="link" danger onClick={() => handleDeletePermission(record)}>删除</Button>
        </Flex>
      ),
    },
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
