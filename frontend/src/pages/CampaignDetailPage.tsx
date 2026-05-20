import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { ArrowLeft, Play, Pause, Trash2, RefreshCw, AlertCircle, Send } from 'lucide-react'
import {
  Campaign, CampaignRow, getCampaign, listCampaignRows,
  startCampaign, pauseCampaign, deleteCampaign,
} from '../api/campaigns'

const rowStatus: Record<string, { label: string; cls: string; dot: string }> = {
  pending:  { label: 'pendente',  cls: 'text-gray-700 bg-gray-50 border border-gray-200',                dot: 'bg-gray-400' },
  sending:  { label: 'enviando',  cls: 'text-forest-900 bg-lime-accent/30 border border-lime-accent/50', dot: 'bg-forest-700 animate-pulse' },
  sent:     { label: 'enviado',   cls: 'text-emerald-800 bg-emerald-50 border border-emerald-200',       dot: 'bg-emerald-500' },
  failed:   { label: 'falha',     cls: 'text-rose-800 bg-rose-50 border border-rose-200',                dot: 'bg-rose-500' },
  skipped:  { label: 'pulado',    cls: 'text-gray-500 bg-gray-50 border border-gray-200',                dot: 'bg-gray-300' },
}

const campaignBadge: Record<string, { label: string; cls: string; dot: string }> = {
  draft:     { label: 'Rascunho',  cls: 'bg-gray-50 text-gray-700 border border-gray-200',                dot: 'bg-gray-400' },
  running:   { label: 'Rodando',   cls: 'bg-lime-accent/30 text-forest-900 border border-lime-accent/50',  dot: 'bg-forest-700 animate-pulse' },
  paused:    { label: 'Pausada',   cls: 'bg-amber-50 text-amber-800 border border-amber-200',              dot: 'bg-amber-500' },
  completed: { label: 'Concluída', cls: 'bg-emerald-50 text-emerald-800 border border-emerald-200',        dot: 'bg-emerald-500' },
  failed:    { label: 'Falhou',    cls: 'bg-rose-50 text-rose-800 border border-rose-200',                  dot: 'bg-rose-500' },
}

export default function CampaignDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [c, setC] = useState<Campaign | null>(null)
  const [rows, setRows] = useState<CampaignRow[]>([])
  const [filter, setFilter] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)

  async function load() {
    if (!id) return
    try {
      const [cr, rr] = await Promise.all([getCampaign(id), listCampaignRows(id, filter)])
      setC(cr.data)
      setRows(rr.data)
      setErr(null)
    } catch (e: any) {
      setErr(e?.response?.data?.error || 'Falha ao carregar')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    const t = setInterval(load, 4000)
    return () => clearInterval(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, filter])

  async function start() {
    if (!id) return
    try { await startCampaign(id); load() }
    catch (e: any) { setErr(e?.response?.data?.error || 'Erro') }
  }
  async function pause() {
    if (!id) return
    try { await pauseCampaign(id); load() }
    catch (e: any) { setErr(e?.response?.data?.error || 'Erro') }
  }
  async function remove() {
    if (!id) return
    if (!confirm('Excluir esta campanha? Mensagens já enviadas não voltam.')) return
    try { await deleteCampaign(id); navigate('/campaigns') }
    catch (e: any) { setErr(e?.response?.data?.error || 'Erro') }
  }

  if (loading && !c) return <div className="p-8 text-gray-400">Carregando...</div>
  if (!c) return <div className="p-8 text-gray-400">Campanha não encontrada.</div>
  const s = campaignBadge[c.status]
  const pct = c.total > 0 ? Math.round(((c.sent + c.failed) / c.total) * 100) : 0
  const pending = c.total - c.sent - c.failed

  return (
    <div className="p-6 lg:p-8">
      {/* Header */}
      <div className="flex items-start gap-4 mb-6">
        <Link
          to="/campaigns"
          className="p-2 rounded-lg text-gray-500 hover:text-forest-900 hover:bg-white border border-transparent hover:border-gray-200 transition-colors"
        >
          <ArrowLeft className="w-5 h-5" />
        </Link>
        <div className="flex-1 min-w-0">
          <h1 className="text-2xl font-bold text-forest-950 tracking-tight truncate">{c.name}</h1>
          <div className="text-sm text-gray-500 mt-1 flex items-center gap-3 flex-wrap">
            <span className={`inline-flex items-center gap-1.5 text-[11px] font-semibold px-2 py-0.5 rounded-full ${s.cls}`}>
              <span className={`w-1.5 h-1.5 rounded-full ${s.dot}`} />
              {s.label}
            </span>
            <span>·</span>
            <span>{c.total} contatos</span>
            <span>·</span>
            <span>{c.device_ids.length} device(s)</span>
            <span>·</span>
            <span>delay {c.delay_min_seconds}-{c.delay_max_seconds}s</span>
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {(c.status === 'draft' || c.status === 'paused') && (
            <button onClick={start} className="btn-accent inline-flex items-center gap-1.5">
              <Play className="w-4 h-4" /> Iniciar
            </button>
          )}
          {c.status === 'running' && (
            <button onClick={pause} className="px-4 py-2 rounded-lg bg-amber-100 text-amber-800 border border-amber-200 text-sm font-medium hover:bg-amber-200 active:bg-amber-300 inline-flex items-center gap-1.5">
              <Pause className="w-4 h-4" /> Pausar
            </button>
          )}
          <button
            onClick={remove}
            className="p-2 rounded-lg text-rose-600 hover:bg-rose-50 border border-transparent hover:border-rose-200 transition-colors"
            title="Excluir"
          >
            <Trash2 className="w-4 h-4" />
          </button>
          <button
            onClick={load}
            className="p-2 rounded-lg text-gray-500 hover:text-forest-900 hover:bg-white border border-transparent hover:border-gray-200 transition-colors"
            title="Atualizar"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
        </div>
      </div>

      {err && (
        <div className="mb-4 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 flex items-start gap-2 text-sm text-rose-800">
          <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" /> {err}
        </div>
      )}

      {/* Hero metrics */}
      <div className="rounded-2xl bg-forest-900 text-white p-6 mb-4 relative overflow-hidden">
        <div className="absolute -top-20 -right-20 w-64 h-64 rounded-full bg-lime-accent/10 blur-3xl" />
        <div className="absolute -bottom-32 -left-10 w-72 h-72 rounded-full bg-forest-700/40 blur-3xl" />
        <div className="relative grid grid-cols-2 md:grid-cols-4 gap-6 mb-4">
          <HeroStat label="Total" value={c.total} />
          <HeroStat label="Enviados" value={c.sent} accent />
          <HeroStat label="Pendentes" value={pending} />
          <HeroStat label="Falhas" value={c.failed} tone="rose" />
        </div>
        <div className="relative flex items-center gap-3">
          <div className="flex-1 h-2 bg-white/10 rounded-full overflow-hidden">
            <div
              className="h-full bg-lime-accent rounded-full transition-all"
              style={{ width: `${pct}%` }}
            />
          </div>
          <div className="text-sm font-mono text-white/80 w-12 text-right">{pct}%</div>
        </div>
      </div>

      {/* Message preview */}
      <div className="card p-5 mb-4">
        <div className="flex items-center gap-2 mb-2">
          <Send className="w-4 h-4 text-forest-700" />
          <div className="section-label">Mensagem</div>
        </div>
        <pre className="whitespace-pre-wrap font-sans text-sm text-gray-800 bg-gray-50 rounded-lg px-4 py-3 border border-gray-200">{c.message}</pre>
      </div>

      {/* Rows */}
      <div className="card overflow-hidden">
        <div className="px-5 py-3 border-b border-gray-100 bg-gray-50/50 flex items-center gap-3">
          <div className="section-label flex-1">Contatos</div>
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="text-xs px-3 py-1.5 rounded-lg border border-gray-200 bg-white text-gray-700 focus:outline-none focus:ring-2 focus:ring-forest-500/30"
          >
            <option value="">Todos</option>
            <option value="pending">Pendentes</option>
            <option value="sending">Enviando</option>
            <option value="sent">Enviados</option>
            <option value="failed">Falhas</option>
          </select>
        </div>
        {rows.length === 0 ? (
          <div className="px-5 py-8 text-sm text-gray-400 italic text-center">
            Sem contatos pra exibir.
          </div>
        ) : (
          <div className="max-h-[480px] overflow-y-auto">
            <table className="w-full text-sm">
              <thead className="text-[11px] uppercase tracking-widest text-gray-500 bg-white border-b border-gray-100 sticky top-0">
                <tr>
                  <th className="px-5 py-2.5 text-left font-bold">Telefone</th>
                  <th className="px-5 py-2.5 text-left font-bold">Nome</th>
                  <th className="px-5 py-2.5 text-left font-bold">Status</th>
                  <th className="px-5 py-2.5 text-left font-bold">Enviado em</th>
                  <th className="px-5 py-2.5 text-left font-bold">Erro</th>
                </tr>
              </thead>
              <tbody>
                {rows.map(r => {
                  const rs = rowStatus[r.status] || rowStatus.pending
                  return (
                    <tr key={r.id} className="border-b border-gray-100 last:border-0 hover:bg-gray-50/50">
                      <td className="px-5 py-3 font-mono text-forest-950">{r.phone}</td>
                      <td className="px-5 py-3 text-gray-700">{r.name || <span className="text-gray-300">—</span>}</td>
                      <td className="px-5 py-3">
                        <span className={`inline-flex items-center gap-1.5 text-[11px] font-semibold px-2 py-0.5 rounded-full ${rs.cls}`}>
                          <span className={`w-1.5 h-1.5 rounded-full ${rs.dot}`} />
                          {rs.label}
                        </span>
                      </td>
                      <td className="px-5 py-3 text-gray-500 text-xs">
                        {r.sent_at ? new Date(r.sent_at).toLocaleString('pt-BR') : '—'}
                      </td>
                      <td className="px-5 py-3 text-rose-700 text-xs">{r.error_msg || ''}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}

function HeroStat({ label, value, accent, tone }: { label: string; value: number; accent?: boolean; tone?: 'rose' }) {
  let valueCls = 'text-white'
  if (accent) valueCls = 'text-lime-accent'
  if (tone === 'rose') valueCls = 'text-rose-300'
  return (
    <div>
      <div className="text-xs uppercase tracking-widest text-white/50 font-bold">{label}</div>
      <div className={`text-3xl md:text-4xl font-extrabold tracking-tight mt-1 ${valueCls}`}>{value}</div>
    </div>
  )
}
