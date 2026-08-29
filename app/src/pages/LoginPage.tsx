import React, { useState, useEffect } from "react";
import { Form, Input, Button, Card, message, Flex, Alert } from "antd";
import { useNavigate } from "react-router-dom";
import { authApi } from "../api/iam";
import { getCookie } from "../api/request";

function clearCookie(name: string): void {
  document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
}

export default function LoginPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    const token = getCookie("access_token");
    if (!token || typeof token !== 'string' || token.trim().length === 0) {
      clearCookie("access_token");
      console.log("Cleared invalid token from cookie");
    }
  }, []);

  const onFinish = async (values: { email: string; password: string }) => {
    setLoading(true);
    setErrorMsg(null);
    try {
      console.log("Login request:", values);
      const response = await authApi.login(values);
      console.log("Login response:", response);

      if (!response) {
        throw new Error("登录响应为空，请稍后重试");
      }

      const token = response.accessToken;
      console.log("Token from response:", token);

      if (token && typeof token === 'string' && token.trim().length > 0) {
        document.cookie = `access_token=${token}; path=/`;
        localStorage.setItem("access_token", token);
        message.success("登录成功");
        navigate("/");
      } else {
        throw new Error("登录响应格式错误：缺少有效的 token");
      }
    } catch (error) {
      console.error("Login error:", error);
      clearCookie("access_token");
      localStorage.removeItem("access_token");
      let errorMessage = "登录失败，请检查邮箱和密码";
      if (error instanceof Error) {
        errorMessage = error.message;
      }
      setErrorMsg(errorMessage);
      message.error(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const onFinishFailed = () => {
    message.error("请检查表单填写是否正确");
  };

  return (
    <div style={{
      display: "flex",
      justifyContent: "center",
      alignItems: "center",
      height: "100vh",
      background: "linear-gradient(135deg, #667eea 0%, #764ba2 100%)"
    }}>
      <Card
        title="IAM 系统登录"
        style={{ width: 400 }}
        headStyle={{ textAlign: "center", fontSize: 24 }}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          onFinishFailed={onFinishFailed}
        >
          {errorMsg && (
            <Alert
              message={errorMsg}
              type="error"
              showIcon
              style={{ marginBottom: 16 }}
            />
          )}
          <Form.Item
            label="邮箱"
            name="email"
            rules={[
              { required: true, message: "请输入邮箱!" },
              { type: "email", message: "请输入有效的邮箱格式!" }
            ]}
          >
            <Input placeholder="请输入邮箱" />
          </Form.Item>

          <Form.Item
            label="密码"
            name="password"
            rules={[{ required: true, message: "请输入密码!" }]}
          >
            <Input.Password placeholder="请输入密码" />
          </Form.Item>

          <Form.Item>
            <Flex justify="center">
              <Button type="primary" htmlType="submit" loading={loading} style={{ width: "100%" }}>
                登录
              </Button>
            </Flex>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}