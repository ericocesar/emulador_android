import client from './client'

export interface APKInfo {
  filename: string
  size_bytes: number
  uploaded_at: string
  sha256?: string
  exists: boolean
  human_size: string
  package_name?: string
  version_name?: string
  version_code?: number
  app_label?: string
  min_sdk?: number
  target_sdk?: number
}

export async function getAPKInfo(): Promise<APKInfo> {
  const r = await client.get<APKInfo>('/apk')
  return r.data
}

export async function uploadAPK(
  file: File,
  onProgress?: (pct: number) => void
): Promise<{ ok: boolean; size_bytes: number; human_size: string; original: string }> {
  const form = new FormData()
  form.append('apk', file)
  const r = await client.post('/apk', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    onUploadProgress: (e) => {
      if (onProgress && e.total) onProgress(Math.round((e.loaded / e.total) * 100))
    },
    timeout: 0,
    maxBodyLength: 250 * 1024 * 1024,
    maxContentLength: 250 * 1024 * 1024,
  })
  return r.data
}

export async function deleteAPK(): Promise<{ ok: boolean }> {
  const r = await client.delete('/apk')
  return r.data
}

export async function updateDeviceApp(deviceID: string): Promise<{ ok: boolean; size_bytes: number }> {
  const r = await client.post(`/devices/${deviceID}/update-app`)
  return r.data
}

// ─── multi-version ───────────────────────────────────────────────────

export interface APKListItem {
  id: string
  filename: string
  package_name: string
  version_name: string
  version_code: number
  app_label: string
  size_bytes: number
  human_size: string
  sha256: string
  min_sdk: number
  target_sdk: number
  is_default: boolean
  source: string
  created_at: string
  devices_linked: number
}

export async function listAPKs(): Promise<{ data: APKListItem[] }> {
  const r = await client.get<{ data: APKListItem[] | null }>('/apks')
  return { data: r.data.data || [] }
}

export async function uploadAPKv2(
  file: File,
  onProgress?: (pct: number) => void
): Promise<{ data: APKListItem }> {
  const form = new FormData()
  form.append('apk', file)
  const r = await client.post('/apks', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    onUploadProgress: (e) => {
      if (onProgress && e.total) onProgress(Math.round((e.loaded / e.total) * 100))
    },
    timeout: 0,
    maxBodyLength: 250 * 1024 * 1024,
    maxContentLength: 250 * 1024 * 1024,
  })
  return r.data
}

export async function setDefaultAPK(id: string): Promise<{ ok: boolean }> {
  const r = await client.patch(`/apks/${id}/default`)
  return r.data
}

export async function checkAPKUpdate(): Promise<{ ok: boolean; message: string; new_apk_id?: string }> {
  const r = await client.post('/apks/check-update', null, { timeout: 5 * 60 * 1000 })
  return r.data
}
