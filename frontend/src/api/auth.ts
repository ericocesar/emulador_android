import client from './client';
import { AuthResponse, UserResponse } from '../types';

export async function register(email: string, password: string, name: string) {
  const response = await client.post<AuthResponse>('/auth/register', {
    email,
    password,
    name,
  });
  return response.data;
}

export async function login(email: string, password: string) {
  const response = await client.post<AuthResponse>('/auth/login', {
    email,
    password,
  });
  return response.data;
}

export async function getMe() {
  const response = await client.get<UserResponse>('/auth/me');
  return response.data;
}
