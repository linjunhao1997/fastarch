import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Spin, Flex } from "antd";

interface AuthWrapperProps {
  children: React.ReactNode;
}

function getCookie(name: string): string | null {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop()?.split(';').shift() || null;
  return null;
}

export default function AuthWrapper({ children }: AuthWrapperProps) {
  const navigate = useNavigate();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    const token = getCookie("access_token");
    console.log("AuthWrapper token check from cookie:", token);
    if (!token || typeof token !== 'string' || token.trim().length === 0) {
      navigate("/login");
    } else {
      setChecking(false);
    }
  }, [navigate]);

  if (checking) {
    return (
      <Flex justify="center" align="center" style={{ height: "100vh" }}>
        <Spin size="large" tip="加载中..." />
      </Flex>
    );
  }

  return <>{children}</>;
}