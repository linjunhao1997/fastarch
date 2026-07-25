// AuthenticationPolicyCreatePage.jsx
import React, { useState, useEffect } from "react";
import { Form, Input, Button, message, Flex, Card, Spin, Select } from "antd";
import { useNavigate, useParams } from "react-router-dom";

export default function AuthenticationPolicyCreatePage() {
  const { type } = useParams(); // 获取路由参数中的策略类型
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [policyTypes, setPolicyTypes] = useState([]);

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

  // 确保类型参数有效
  if (!policyTypes.some(t => t.id === type)) {
    message.error("无效的认证策略类型");
    navigate('/authentication_management/authentication_policy_list');
    return null;
  }

  // 处理表单提交
  const onFinish = async (values) => {
    setSubmitting(true);
    try {
      console.log("提交的创建数据:", { ...values, type });
      
      // 这里替换为实际的API调用
      await new Promise((resolve) => setTimeout(resolve, 1000)); // 模拟API请求
      
      message.success("认证策略创建成功");
      navigate('/authentication_management/authentication_policy_list'); // 返回认证策略列表页
    } catch (error) {
      message.error("认证策略创建失败: " + (error.message || "未知错误"));
    } finally {
      setSubmitting(false);
    }
  };

  // 表单验证失败
  const onFinishFailed = (errorInfo) => {
    console.log("表单验证失败:", errorInfo);
    message.error("请检查表单填写是否正确");
  };

  if (loading) {
    return (
      <div style={{ padding: "24px" }}>
        <div style={{ textAlign: "center", padding: "50px" }}>
          <Spin size="large" tip="加载中..." />
        </div>
      </div>
    );
  }

  return (
    <div style={{ padding: "24px" }}>
      <Card title={`创建${getPolicyTypeName(type)}策略`}>
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          onFinishFailed={onFinishFailed}
        >
          <Form.Item
            label="策略类型"
            name="type"
            initialValue={type}
            hidden
          >
            <Select>
              {policyTypes.map(t => (
                <Select.Option key={t.id} value={t.id}>{t.name}</Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            label="策略名称"
            name="name"
            rules={[{ required: true, message: "请输入策略名称!" }]}
          >
            <Input />
          </Form.Item>

          <Form.Item
            label="策略描述"
            name="description"
            rules={[{ required: true, message: "请输入策略描述!" }]}
          >
            <Input.TextArea rows={4} />
          </Form.Item>

          <Form.Item
            label="实现方式ID"
            name="implementationId"
            rules={[{ required: true, message: "请输入实现方式ID!" }]}
          >
            <Input />
          </Form.Item>

          <Form.Item>
            <Flex justify="space-between">
              <Button onClick={() => navigate(-1)}>
                返回
              </Button>
              <Flex gap={8}>
                <Button onClick={() => navigate(-1)}>
                  取消
                </Button>
                <Button type="primary" htmlType="submit" loading={submitting}>
                  保存
                </Button>
              </Flex>
            </Flex>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}

// 获取策略类型名称的辅助函数
function getPolicyTypeName(type) {
  const policyTypes = {
    "sso": "单点登录认证",
    "mfa": "多重身份认证",
    "otp": "一次性密码认证"
  };
  return policyTypes[type] || "认证";
}
