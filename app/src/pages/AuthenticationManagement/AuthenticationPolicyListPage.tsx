// AuthenticationPolicyListPage.jsx
import React, { useState, useEffect } from "react";
import { Table, Card, Button, message, Spin, Modal, Form, Select, Input, Tabs } from "antd";
import { PlusOutlined, EditOutlined, DeleteOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";

const { TabPane } = Tabs;

export default function AuthenticationPolicyListPage() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [ssoPolicies, setSsoPolicies] = useState([]);
  const [mfaPolicies, setMfaPolicies] = useState([]);
  const [otpPolicies, setOtpPolicies] = useState([]);
  const [policyTypes, setPolicyTypes] = useState([]);
  const [activeTab, setActiveTab] = useState("sso");

  // 模拟获取认证策略类型
  useEffect(() => {
    // 实际中应该调用API获取
    setPolicyTypes([
      { id: "sso", name: "单点登录认证" },
      { id: "mfa", name: "多重身份认证" },
      { id: "otp", name: "一次性密码认证" }
    ]);
    setLoading(false);
  }, []);

  // 模拟获取各类型认证策略
  useEffect(() => {
    // 实际中应该调用API获取
    setSsoPolicies([
      { id: 1, name: "企业SSO", description: "通过企业账号系统进行单点登录", implementationId: "sso-impl-001" },
      { id: 2, name: "社交账号SSO", description: "通过社交账号进行单点登录", implementationId: "sso-impl-002" }
    ]);
    
    setMfaPolicies([
      { id: 1, name: "短信验证", description: "通过短信验证码进行二次验证", implementationId: "mfa-impl-001" },
      { id: 2, name: "邮箱验证", description: "通过邮箱验证码进行二次验证", implementationId: "mfa-impl-002" },
      { id: 3, name: "Authenticator应用", description: "通过Authenticator应用生成验证码", implementationId: "mfa-impl-003" }
    ]);
    
    setOtpPolicies([
      { id: 1, name: "邮件OTP", description: "通过邮件发送一次性密码", implementationId: "otp-impl-001" },
      { id: 2, name: "短信OTP", description: "通过短信发送一次性密码", implementationId: "otp-impl-002" }
    ]);
  }, []);

  // 处理表格行操作 - 编辑
  const handleEdit = (policy, policyType) => {
    navigate(`/authentication_management/edit_authentication_policy/${policyType}/${policy.id}`);
  };

  // 处理表格行操作 - 删除
  const handleDelete = (policyId, policyType) => {
    Modal.confirm({
      title: "确认删除",
      content: "确定要删除此认证策略吗？",
      onOk: async () => {
        // 实际中应该调用API删除
        if (policyType === "sso") {
          setSsoPolicies(ssoPolicies.filter(item => item.id !== policyId));
        } else if (policyType === "mfa") {
          setMfaPolicies(mfaPolicies.filter(item => item.id !== policyId));
        } else if (policyType === "otp") {
          setOtpPolicies(otpPolicies.filter(item => item.id !== policyId));
        }
        message.success("删除成功");
      }
    });
  };

  // SSO策略表格列定义
  const ssoColumns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "策略名称", dataIndex: "name", key: "name" },
    { title: "描述", dataIndex: "description", key: "description" },
    { title: "实现方式ID", dataIndex: "implementationId", key: "implementationId" },
    {
      title: "操作",
      key: "action",
      render: (_, record) => (
        <>
          <Button 
            type="text" 
            icon={<EditOutlined />} 
            onClick={() => handleEdit(record, "sso")}
          >
            编辑
          </Button>
          <Button 
            type="text" 
            danger 
            icon={<DeleteOutlined />} 
            onClick={() => handleDelete(record.id, "sso")}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  // MFA策略表格列定义
  const mfaColumns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "策略名称", dataIndex: "name", key: "name" },
    { title: "描述", dataIndex: "description", key: "description" },
    { title: "实现方式ID", dataIndex: "implementationId", key: "implementationId" },
    {
      title: "操作",
      key: "action",
      render: (_, record) => (
        <>
          <Button 
            type="text" 
            icon={<EditOutlined />} 
            onClick={() => handleEdit(record, "mfa")}
          >
            编辑
          </Button>
          <Button 
            type="text" 
            danger 
            icon={<DeleteOutlined />} 
            onClick={() => handleDelete(record.id, "mfa")}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  // OTP策略表格列定义
  const otpColumns = [
    { title: "ID", dataIndex: "id", key: "id" },
    { title: "策略名称", dataIndex: "name", key: "name" },
    { title: "描述", dataIndex: "description", key: "description" },
    { title: "实现方式ID", dataIndex: "implementationId", key: "implementationId" },
    {
      title: "操作",
      key: "action",
      render: (_, record) => (
        <>
          <Button 
            type="text" 
            icon={<EditOutlined />} 
            onClick={() => handleEdit(record, "otp")}
          >
            编辑
          </Button>
          <Button 
            type="text" 
            danger 
            icon={<DeleteOutlined />} 
            onClick={() => handleDelete(record.id, "otp")}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  if (loading) {
    return (
      <div style={{ padding: "24px" }}>
        <Spin size="large" tip="加载中..." />
      </div>
    );
  }

  // 根据当前选中的标签页显示不同的表格
  const renderTableByTab = () => {
    switch (activeTab) {
      case "sso":
        return (
          <>
            <Button 
              type="primary" 
              icon={<PlusOutlined />}
              onClick={() => navigate('/authentication_management/create_authentication_policy/sso')}
              style={{ marginBottom: 16 }}
            >
              新增单点登录认证策略
            </Button>
            <Table 
              columns={ssoColumns} 
              dataSource={ssoPolicies} 
              rowKey="id" 
              pagination={{ pageSize: 10 }}
            />
          </>
        );
      case "mfa":
        return (
          <>
            <Button 
              type="primary" 
              icon={<PlusOutlined />}
              onClick={() => navigate('/authentication_management/create_authentication_policy/mfa')}
              style={{ marginBottom: 16 }}
            >
              新增多重身份认证策略
            </Button>
            <Table 
              columns={mfaColumns} 
              dataSource={mfaPolicies} 
              rowKey="id" 
              pagination={{ pageSize: 10 }}
            />
          </>
        );
      case "otp":
        return (
          <>
            <Button 
              type="primary" 
              icon={<PlusOutlined />}
              onClick={() => navigate('/authentication_management/create_authentication_policy/otp')}
              style={{ marginBottom: 16 }}
            >
              新增一次性密码认证策略
            </Button>
            <Table 
              columns={otpColumns} 
              dataSource={otpPolicies} 
              rowKey="id" 
              pagination={{ pageSize: 10 }}
            />
          </>
        );
      default:
        return null;
    }
  };

  return (
    <div style={{ padding: "24px" }}>
      <Card>
        <Tabs activeKey={activeTab} onChange={setActiveTab}>
          <TabPane tab="单点登录认证" key="sso" />
          <TabPane tab="多重身份认证" key="mfa" />
          <TabPane tab="一次性密码认证" key="otp" />
        </Tabs>
        {renderTableByTab()}
      </Card>
    </div>
  );
}
