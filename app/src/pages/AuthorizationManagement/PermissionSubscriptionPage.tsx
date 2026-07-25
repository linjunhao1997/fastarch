import React, { useState, useEffect } from "react";
import { Flex, Input, Button, Table, message } from "antd";
import { useNavigate } from "react-router";
import { subscriptionApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

const { Search } = Input;

export default function PermissionSubscriptionPage() {
  const navigate = useNavigate();
  const [subscriptions, setSubscriptions] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: "" });
  const showNotification = useApiErrorNotification();

  const fetchSubscriptions = async () => {
    setLoading(true);
    try {
      const response = await subscriptionApi.list(1, 100);
      if (response.status.code === "0000") {
        setSubscriptions(response.subscriptions || []);
      }
    } catch (error) {
      showNotification(error, "获取订阅列表失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSubscriptions();
  }, [showNotification]);

  const handleSearch = (value: string) => {
    setFilters({ name: value });
  };

  const handleReset = () => {
    setFilters({ name: "" });
  };

  const handleCreateSubscription = () => {
    navigate('/authorization_management/subscription_create');
  };

  const filteredSubscriptions = subscriptions.filter((subscription) =>
    !filters.name || subscription.applicant_name?.includes(filters.name)
  );

  const columns = [
    { title: "源应用", dataIndex: "source_application_code", key: "source_application_code" },
    { title: "目标应用", dataIndex: "target_application_code", key: "target_application_code" },
    { title: "申请人", dataIndex: "applicant_name", key: "applicant_name" },
    { title: "审批人", dataIndex: "approver_name", key: "approver_name" },
    { title: "状态", dataIndex: "status", key: "status" },
  ];

  return (
    <div>
      <Flex gap={16} style={{ marginBottom: 16 }}>
        <Search placeholder="搜索申请人" onSearch={handleSearch} style={{ width: 200 }} />
        <Button onClick={handleReset}>重置</Button>
        <Button type="primary" onClick={handleCreateSubscription}>创建订阅</Button>
      </Flex>
      <Table
        columns={columns}
        dataSource={filteredSubscriptions}
        rowKey="source_application_code"
        loading={loading}
      />
    </div>
  );
}