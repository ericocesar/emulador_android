export interface User {
  id: string;
  email: string;
  name: string;
  is_admin: boolean;
}

export type DeviceStatus =
  | 'creating'
  | 'booting'
  | 'installing'
  | 'ready'
  | 'stopped'
  | 'error';

export interface Device {
  id: string;
  user_id: string;
  name: string;
  container_id: string;
  status: DeviceStatus;
  error_message: string;
  android_version: string;
  adb_port: number;
  installed_apk_hash: string;
  created_at: string;
  updated_at: string;
}

export interface DeviceStats {
  cpu_percent: number;
  memory_usage_mb: number;
  memory_limit_mb: number;
  uptime: string;
}

export interface TapInput {
  type: 'tap';
  x: number;
  y: number;
}

export interface SwipeInput {
  type: 'swipe';
  start_x: number;
  start_y: number;
  end_x: number;
  end_y: number;
  duration: number;
}

export interface TextInput {
  type: 'text';
  text: string;
}

export interface KeyEventInput {
  type: 'keyevent';
  keycode: number;
}

export type InputMessage = TapInput | SwipeInput | TextInput | KeyEventInput;

export interface AuthResponse {
  data: {
    token: string;
    user: User;
  };
}

export interface UserResponse {
  data: User;
}

export interface DeviceResponse {
  data: Device;
}

export interface DevicesResponse {
  data: Device[];
}

export interface DeviceStatsResponse {
  data: DeviceStats;
}

export interface MessageResponse {
  data: {
    message: string;
  };
}
