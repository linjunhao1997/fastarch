import { useState, useEffect } from "react";
import { Flex, Input, Button, Table, Modal, message } from "antd";
import { useNavigate } from "react-router";
import { applicationApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import type { Application } from "../../api/iam";

const { Search } = Input;

export default function ApplicationListPage() {
  const navigate = useNavigate();
  const [applications, setApplications] = useState<Application[]>([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: "" });
  const showNotification = useApiErrorNotification();

  const fetchApplications = async () => {
    try {
      setLoading(true);
      const response = await applicationApi.list(1, 100);
      setApplications(response.applications || []);
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

  const handleEditApplication = (appId: string) => {
    navigate(`/application_management/edit_application/${appId}`);
  };

  const handleDeleteApplication = (appId: string) => {
    Modal.confirm({
      title: "确认删除",
      content: "确定要删除此应用吗？删除后不可恢复。",
      onOk: async () => {
        try {
          await applicationApi.delete(appId);
          message.success("删除成功");
          await fetchApplications();
        } catch (error) {
          showNotification(error, "删除应用失败");
          throw error;
        }
      },
    });
  };

  const filteredApplications = applications.filter((app) =>
    !filters.name || app.name?.includes(filters.name)
  );

  const columns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "名称", dataIndex: "name", key: "name" },
    { title: "应用编码", dataIndex: "appCode", key: "appCode" },
    { title: "状态", dataIndex: "enabledStatus", key: "enabledStatus" },
    {
      title: "操作",
      key: "action",
      render: (_: unknown, record: Application) => (
        <Flex gap={8}>
          <Button type="link" onClick={() => handleEditApplication(record.id)}>
            编辑
          </Button>
          <Button type="link" danger onClick={() => handleDeleteApplication(record.id)}>
            删除
          </Button>
        </Flex>
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
