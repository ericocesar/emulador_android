import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Send, RefreshCw, ChevronRight, Smartphone, Plus } from 'lucide-react'
import { Campaign, listCampaigns } from '../api/campaigns'
import MassDispatchModal from '../components/MassDispatchModal'

const statusBadge: Record<Campaign['status'], { label: string; cls: string; dot: string }> = {
  draft:     { label: 'Rascunho',  cls: 'bg-gray-50 text-gray-700 border border-gray-200',                dot: 'bg-gray-400' },
  running:   { label: 'Rodando',   cls: 'bg-lime-accent/30 text-forest-900 border border-lime-accent/50',  dot: 'bg-forest-700 animate-pulse' },
  paused:    { label: 'Pausada',   cls: 'bg-amber-50 text-amber-800 border border-amber-200',              dot: 'bg-amber-500' },
  completed: { label: 'Concluída', cls: 'bg-emerald-50 text-emerald-800 border border-emerald-200',        dot: 'bg-emerald-500' },
  failed:    { label: 'Falhou',    cls: 'bg-rose-50 text-rose-800 border border-rose-200',                  dot: 'bg-rose-500' },
}

export default function CampaignsPage() {
  const [items, setItems] = useState<Campaign[]>([])
  const [loading, setLoading] = useState(true)
  const [dispatchOpen, setDispatchOpen] = useState(false)

  async function refresh() {
    setLoading(true)
    try {
      const r = await listCampaigns()
      setItems(r.data)
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => {
    refresh()
    const t = setInterval(refresh, 5000)
    return () => clearInterval(t)
  }, [])

  return (
    <div className="p-6 lg:p-8">
      <div className="flex items-start justify-between mb-6 gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-bold text-forest-950 tracking-tight">Disparos</h1>
          <p className="text-sm text-gray-500 mt-1">
            Campanhas de envio em massa para WhatsApp.
          </p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={refresh}
            className="p-2 rounded-lg text-gray-500 hover:text-forest-900 hover:bg-white border border-transparent hover:border-gray-200 transition-colors"
            title="Atualizar"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => setDispatchOpen(true)}
            className="btn-accent inline-flex items-center gap-2"
          >
            <Plus className="w-4 h-4" />
            Novo disparo
          </button>
        </div>
      </div>

      {/* List */}
      <div className="card overflow-hidden">
        {items.length === 0 ? (
          <div className="px-6 py-12 text-center">
            <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-forest-50 text-forest-700 mb-3">
              <Send className="w-6 h-6" />
            </div>
            <div className="text-base font-semibold text-forest-950">Nenhuma campanha ainda</div>
            <div className="text-sm text-gray-500 mt-1 mb-4">
              Crie sua primeira campanha de envio em massa.
            </div>
            <button
              onClick={() => setDispatchOpen(true)}
              className="btn-accent inline-flex items-center gap-2"
            >
              <Plus className="w-4 h-4" />
              Novo disparo
            </button>
          </div>
        ) : (
          <div className="divide-y divide-gray-100">
            {items.map(c => {
              const s = statusBadge[c.status]
              const pct = c.total > 0 ? Math.round(((c.sent + c.failed) / c.total) * 100) : 0
              return (
                <Link
                  key={c.id}
                  to={`/campaigns/${c.id}`}
                  className="block px-5 py-4 hover:bg-gray-50 transition-colors group"
                >
                  <div className="flex items-center gap-4">
                    <div className="w-11 h-11 rounded-xl bg-forest-50 grid place-items-center text-forest-700 shrink-0">
                      <Send className="w-5 h-5" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <div className="font-semibold text-forest-950 truncate">{c.name}</div>
                        <span className={`inline-flex items-center gap-1.5 text-[11px] font-semibold px-2 py-0.5 rounded-full ${s.cls}`}>
                          <span className={`w-1.5 h-1.5 rounded-full ${s.dot}`} />
                          {s.label}
                        </span>
                      </div>
                      <div className="text-xs text-gray-500 mt-1 flex items-center gap-3 flex-wrap">
                        <span>
                          <span className="font-medium text-forest-900">{c.total}</span> contatos
                        </span>
                        <span className="text-gray-300">·</span>
                        <span>
                          <span className="text-emerald-700 font-medium">{c.sent}</span> enviados
                        </span>
                        {c.failed > 0 && (
                          <>
                            <span className="text-gray-300">·</span>
                            <span>
                              <span className="text-rose-700 font-medium">{c.failed}</span> falhas
                            </span>
                          </>
                        )}
                        <span className="text-gray-300">·</span>
                        <span className="inline-flex items-center gap-1">
                          <Smartphone className="w-3 h-3" />
                          {c.device_ids.length} device(s)
                        </span>
                        <span className="text-gray-300">·</span>
                        <span>{new Date(c.created_at).toLocaleString('pt-BR')}</span>
                      </div>
                      {c.total > 0 && (
                        <div className="mt-2 h-1.5 w-full bg-gray-100 rounded-full overflow-hidden max-w-md">
                          <div
                            className="h-full bg-forest-900 rounded-full transition-all"
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                      )}
                    </div>
                    <ChevronRight className="w-5 h-5 text-gray-300 group-hover:text-forest-700 transition-colors" />
                  </div>
                </Link>
              )
            })}
          </div>
        )}
      </div>

      <MassDispatchModal
        open={dispatchOpen}
        onClose={() => { setDispatchOpen(false); refresh() }}
      />
    </div>
  )
}
