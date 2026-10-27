export interface Application {
  id: string;
  name: string;
  appCode: string;
  description: string;
  enabledStatus: string;
}

export interface CreateApplicationRequest {
  name: string;
  appCode?: string;
  description?: string;
  enabledStatus?: string;
}

export interface UpdateApplicationRequest {
  name?: string;
  appCode?: string;
  description?: string;
  enabledStatus?: string;
}
