import { useState, useEffect } from "react";
import { Flex, Input, Button, Table, Modal } from "antd";
import { useNavigate } from "react-router";
import { roleApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import type { Role } from "../../api/iam";

const { Search } = Input;

export default function RoleManagementPage() {
  const navigate = useNavigate();
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: "" });
  const showNotification = useApiErrorNotification();

  const fetchRoles = async () => {
    setLoading(true);
    try {
      const response = await roleApi.list(1, 100);
      setRoles(response.roles || []);
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

  const handleEditRole = (roleId: string) => {
    navigate(`/authorization_management/role_edit/${roleId}`);
  };

  const handleDeleteRole = (role: Role) => {
    Modal.confirm({
      title: "删除角色",
      content: `确定删除角色 ${role.name} 吗？`,
      onOk: async () => {
        try {
          await roleApi.delete(role.id);
          await fetchRoles();
        } catch (error) {
          showNotification(error, "删除角色失败");
          throw error;
        }
      },
    });
  };

  const filteredRoles = roles.filter((role) =>
    !filters.name || role.name?.includes(filters.name)
  );

  const auditColumns = [
    { title: "创建人", dataIndex: "createdBy", key: "createdBy", width: 90 },
    { title: "创建时间", dataIndex: "createTime", key: "createTime", width: 170, render: (text: string) => text || "—" },
    { title: "修改人", dataIndex: "updatedBy", key: "updatedBy", width: 90 },
    { title: "修改时间", dataIndex: "updateTime", key: "updateTime", width: 170, render: (text: string) => text || "—" },
  ];

  const columns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "角色名称", dataIndex: "name", key: "name" },
    { title: "角色编码", dataIndex: "roleCode", key: "roleCode" },
    { title: "所属应用", dataIndex: "applicationId", key: "applicationId" },
    { title: "状态", dataIndex: "enabledStatus", key: "enabledStatus" },
    ...auditColumns,
    {
      title: "操作",
      key: "action",
      render: (_: unknown, record: Role) => (
        <Flex gap={0}>
          <Button type="link" onClick={() => handleEditRole(record.id)}>编辑</Button>
          <Button type="link" danger onClick={() => handleDeleteRole(record)}>删除</Button>
        </Flex>
      ),
    },
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
