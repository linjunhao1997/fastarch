import React, { useState, useEffect } from "react";
import { Flex, Input, Button, Table, message } from "antd";
import { useNavigate } from "react-router";
import { applicationApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

const { Search } = Input;

export default function ApplicationListPage() {
  const navigate = useNavigate();
  const [applications, setApplications] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: "" });
  const showNotification = useApiErrorNotification();

  const fetchApplications = async () => {
    try {
      setLoading(true);
      const response = await applicationApi.list(1, 100);
      if (response.status.code === "0000") {
        setApplications(response.applications || []);
      }
    } catch (error) {
      showNotification(error, "获取应用列表失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchApplications();
  }, [showNotification]);

  const handleSearch = (value: string) => {
    setFilters({ name: value });
  };

  const handleReset = () => {
    setFilters({ name: "" });
  };

  const handleCreateApplication = () => {
    navigate('/application_management/create_application');
  };

  const handleEditApplication = (appId: number) => {
    navigate(`/application_management/edit_application/${appId}`);
  };

  const filteredApplications = applications.filter((app) =>
    !filters.name || app.name?.includes(filters.name)
  );

  const columns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "名称", dataIndex: "name", key: "name" },
    { title: "邮箱", dataIndex: "email", key: "email" },
    { title: "状态", dataIndex: "status", key: "status" },
    {
      title: "操作",
      key: "action",
      render: (_: any, record: any) => (
        <Button type="link" onClick={() => handleEditApplication(record.id)}>
          编辑
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Flex gap={16} style={{ marginBottom: 16 }}>
        <Search placeholder="搜索名称" onSearch={handleSearch} style={{ width: 200 }} />
        <Button onClick={handleReset}>重置</Button>
        <Button type="primary" onClick={handleCreateApplication}>创建应用</Button>
      </Flex>
      <Table
        columns={columns}
        dataSource={filteredApplications}
        rowKey="id"
        loading={loading}
      />
    </div>
  );
}