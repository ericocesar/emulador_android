import client from './client'

export interface Campaign {
  id: string
  user_id: string
  name: string
  message: string
  delay_min_seconds: number
  delay_max_seconds: number
  device_ids: string[]
  status: 'draft' | 'running' | 'paused' | 'completed' | 'failed'
  total: number
  sent: number
  failed: number
  created_at: string
  started_at?: string
  completed_at?: string
}

export interface CampaignRow {
  id: string
  campaign_id: string
  phone: string
  name: string
  status: 'pending' | 'sending' | 'sent' | 'failed' | 'skipped'
  error_msg?: string
  device_id?: string
  sent_at?: string
  created_at: string
}

export async function listCampaigns(): Promise<{ data: Campaign[] }> {
  const r = await client.get<{ data: Campaign[] | null }>('/campaigns')
  return { data: r.data.data || [] }
}

export async function getCampaign(id: string): Promise<{ data: Campaign }> {
  const r = await client.get(`/campaigns/${id}`)
  return r.data
}

export async function listCampaignRows(id: string, status?: string): Promise<{ data: CampaignRow[] }> {
  const r = await client.get(`/campaigns/${id}/rows`, { params: status ? { status } : {} })
  return { data: r.data.data || [] }
}

export interface CreateCampaignConfig {
  name: string
  message: string
  delay_min_seconds: number
  delay_max_seconds: number
  device_ids: string[]
}

export async function createCampaign(csv: File, config: CreateCampaignConfig): Promise<{ data: Campaign }> {
  const form = new FormData()
  form.append('csv', csv)
  form.append('config', JSON.stringify(config))
  const r = await client.post('/campaigns', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 0,
  })
  return r.data
}

export async function startCampaign(id: string): Promise<{ ok: boolean }> {
  const r = await client.post(`/campaigns/${id}/start`)
  return r.data
}
export async function pauseCampaign(id: string): Promise<{ ok: boolean }> {
  const r = await client.post(`/campaigns/${id}/pause`)
  return r.data
}
export async function deleteCampaign(id: string): Promise<{ ok: boolean }> {
  const r = await client.delete(`/campaigns/${id}`)
  return r.data
}
