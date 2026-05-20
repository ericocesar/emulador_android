import client from './client'

export interface APIToken {
  id: string
  user_id: string
  name: string
  prefix: string
  last_used_at: string | null
  created_at: string
  revoked_at: string | null
}

export async function listTokens(): Promise<{ data: APIToken[] }> {
  const r = await client.get<{ data: APIToken[] | null }>('/tokens')
  return { data: r.data.data || [] }
}

export async function createToken(name: string): Promise<{ id: string; name: string; token: string }> {
  const r = await client.post('/tokens', { name })
  return r.data
}

export async function revokeToken(id: string): Promise<{ ok: boolean }> {
  const r = await client.delete(`/tokens/${id}`)
  return r.data
}
