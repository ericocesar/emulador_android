import client from './client';
import {
  DeviceResponse,
  DevicesResponse,
  DeviceStatsResponse,
  MessageResponse,
} from '../types';

export async function listDevices() {
  const response = await client.get<DevicesResponse>('/devices');
  return response.data;
}

export async function createDevice(name: string, apkID?: string) {
  const body: Record<string, unknown> = { name };
  if (apkID) body.apk_id = apkID;
  const response = await client.post<DeviceResponse>('/devices', body);
  return response.data;
}

export async function getDevice(id: string) {
  const response = await client.get<DeviceResponse>(`/devices/${id}`);
  return response.data;
}

export async function deleteDevice(id: string) {
  const response = await client.delete<MessageResponse>(`/devices/${id}`);
  return response.data;
}

export async function startDevice(id: string) {
  const response = await client.post<DeviceResponse>(`/devices/${id}/start`);
  return response.data;
}

export async function stopDevice(id: string) {
  const response = await client.post<DeviceResponse>(`/devices/${id}/stop`);
  return response.data;
}

export async function getDeviceStats(id: string) {
  const response = await client.get<DeviceStatsResponse>(
    `/devices/${id}/stats`
  );
  return response.data;
}
