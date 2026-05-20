import { useEffect, useRef, useState } from 'react'
import {
  Upload, Package, RefreshCw, AlertCircle, CheckCircle2, FileBox,
  Star, CloudDownload, Layers,
} from 'lucide-react'
import {
  APKListItem, listAPKs, uploadAPKv2, setDefaultAPK, checkAPKUpdate,
} from '../api/apk'

export default function APKPage() {
  const [apks, setApks] = useState<APKListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState(0)
  const [checking, setChecking] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)
  const [dragOver, setDragOver] = useState(false)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const r = await listAPKs()
      setApks(r.data)
    } catch (e: any) {
      setError(e?.response?.data?.error || 'Falha ao carregar APKs')
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => { refresh() }, [])

  async function handleUpload(file: File) {
    setError(null); setSuccess(null)
    if (!file.name.toLowerCase().endsWith('.apk')) {
      setError('Arquivo precisa ter extensão .apk'); return
    }
    if (file.size > 200 * 1024 * 1024) {
      setError('Arquivo grande demais — máximo 200 MB'); return
    }
    setUploading(true); setProgress(0)
    try {
      const r = await uploadAPKv2(file, p => setProgress(p))
      setSuccess(`Upload concluído · ${r.data.app_label || r.data.filename} v${r.data.version_name}`)
      await refresh()
    } catch (e: any) {
      setError(e?.response?.data?.error || 'Erro no upload')
    } finally {
      setUploading(false); setProgress(0)
    }
  }

  async function handleSetDefault(id: string) {
    setError(null); setSuccess(null)
    try {
      await setDefaultAPK(id)
      setSuccess('Marcada como padrão · novos dispositivos vão usar essa versão')
      await refresh()
    } catch (e: any) {
      setError(e?.response?.data?.error || 'Falha ao definir padrão')
    }
  }

  const [checkElapsed, setCheckElapsed] = useState(0)
  async function handleCheckUpdate() {
    setError(null); setSuccess(null); setChecking(true); setCheckElapsed(0)
    const start = Date.now()
    const tick = setInterval(() => {
      setCheckElapsed(Math.floor((Date.now() - start) / 1000))
    }, 1000)
    try {
      const r = await checkAPKUpdate()
      setSuccess(r.message)
      await refresh()
    } catch (e: any) {
      setError(e?.response?.data?.message || e?.response?.data?.error || 'Falha ao verificar')
    } finally {
      clearInterval(tick)
      setChecking(false)
      setCheckElapsed(0)
    }
  }

  return (
    <div className="p-6 lg:p-8">
      <div className="flex items-start justify-between mb-6 gap-4 flex-wrap">
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-bold text-forest-950 tracking-tight flex items-center gap-2">
            <Package className="w-6 h-6 text-forest-700" />
            Aplicativo (APK)
          </h1>
          <p className="text-sm text-gray-500 mt-1">
            Gerencie as versões do WhatsApp. Versões antigas ficam preservadas (podem estar atreladas a dispositivos).
            A versão marcada como <strong className="text-forest-900">padrão</strong> é usada em novos dispositivos.
          </p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={handleCheckUpdate}
            disabled={checking}
            className="btn-primary inline-flex items-center gap-2 whitespace-nowrap disabled:cursor-wait"
            title="Verificar manualmente se há nova versão no whatsapp.com"
          >
            {checking ? (
              <>
                <span className="w-4 h-4 rounded-full border-2 border-white/40 border-t-white animate-spin" />
                <span>Verificando…</span>
              </>
            ) : (
              <>
                <CloudDownload className="w-4 h-4" />
                <span>Verificar atualização</span>
              </>
            )}
          </button>
          <button
            onClick={refresh}
            className="p-2 rounded-lg text-gray-500 hover:text-forest-900 hover:bg-white border border-transparent hover:border-gray-200 transition-colors"
            title="Recarregar lista"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {checking && (
        <div className="mb-4 rounded-2xl border-2 border-forest-200 bg-forest-50 px-4 py-3 flex gap-3 items-start">
          <div className="w-9 h-9 rounded-full bg-forest-900 grid place-items-center text-lime-accent shrink-0">
            <CloudDownload className="w-5 h-5 animate-bounce" />
          </div>
          <div className="flex-1">
            <div className="text-sm font-semibold text-forest-950">Verificando whatsapp.com…</div>
            <div className="text-xs text-forest-700 mt-0.5">
              Baixando ~140 MB · pode demorar 30 a 90 segundos · {checkElapsed}s decorridos
            </div>
            <div className="mt-2 h-1 w-full bg-forest-200 rounded-full overflow-hidden">
              <div className="h-full bg-forest-700 rounded-full animate-pulse" style={{ width: '40%' }} />
            </div>
          </div>
        </div>
      )}
      {error && (
        <div className="mb-4 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 flex gap-2 items-start">
          <AlertCircle className="w-5 h-5 text-rose-600 shrink-0 mt-0.5" />
          <div className="text-sm text-rose-800">{error}</div>
        </div>
      )}
      {success && (
        <div className="mb-4 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 flex gap-2 items-start">
          <CheckCircle2 className="w-5 h-5 text-emerald-600 shrink-0 mt-0.5" />
          <div className="text-sm text-emerald-800">{success}</div>
        </div>
      )}

      {/* Upload area */}
      <div
        className={`bg-white rounded-2xl border-2 border-dashed mb-6 p-5 transition-colors ${
          dragOver ? 'border-forest-700 bg-forest-50' : 'border-gray-200'
        }`}
        onDragOver={e => { e.preventDefault(); setDragOver(true) }}
        onDragLeave={() => setDragOver(false)}
        onDrop={e => {
          e.preventDefault(); setDragOver(false)
          const f = e.dataTransfer.files?.[0]; if (f) handleUpload(f)
        }}
      >
        <div className="flex items-center gap-4">
          <div className="w-12 h-12 rounded-xl bg-forest-50 grid place-items-center text-forest-700 shrink-0">
            <Upload className="w-6 h-6" />
          </div>
          <div className="flex-1">
            <div className="font-semibold text-forest-950">Adicionar nova versão</div>
            <div className="text-xs text-gray-500">
              Arraste o .apk aqui ou clique para selecionar · máx 200 MB
            </div>
          </div>
          <input
            ref={fileRef}
            type="file"
            accept=".apk,application/vnd.android.package-archive"
            className="hidden"
            onChange={e => { const f = e.target.files?.[0]; if (f) handleUpload(f); e.target.value = '' }}
          />
          <button
            onClick={() => fileRef.current?.click()}
            disabled={uploading}
            className="btn-accent disabled:opacity-50"
          >
            {uploading ? `${progress}%` : 'Selecionar'}
          </button>
        </div>
        {uploading && (
          <div className="mt-3 h-1.5 bg-gray-100 rounded-full overflow-hidden">
            <div className="h-full bg-forest-700 transition-all" style={{ width: `${progress}%` }} />
          </div>
        )}
      </div>

      {/* Version list */}
      <div className="card overflow-hidden">
        <div className="px-5 py-3 border-b border-gray-100 bg-gray-50/50 flex items-center gap-2">
          <Layers className="w-4 h-4 text-gray-500" />
          <div className="section-label">
            Versões armazenadas {apks.length > 0 && `(${apks.length})`}
          </div>
        </div>
        {loading ? (
          <div className="px-5 py-6 text-sm text-gray-400">Carregando…</div>
        ) : apks.length === 0 ? (
          <div className="px-5 py-8 text-sm text-gray-500 italic text-center">
            Nenhum APK ainda. Envie um arquivo acima ou clique em "Verificar atualização" pra baixar do whatsapp.com.
          </div>
        ) : (
          <div className="divide-y divide-gray-100">
            {apks.map(a => (
              <APKRow key={a.id} apk={a} onSetDefault={handleSetDefault} />
            ))}
          </div>
        )}
      </div>

      <div className="mt-4 text-xs text-gray-500 text-center">
        Verificação automática: 1× ao dia (background). Use "Verificar atualização" pra checar agora.
      </div>
    </div>
  )
}

function APKRow({ apk, onSetDefault }: { apk: APKListItem; onSetDefault: (id: string) => void }) {
  const sourceLabel: Record<string, { label: string; cls: string }> = {
    'upload':        { label: 'upload',  cls: 'bg-forest-50 text-forest-800 border border-forest-200' },
    'auto-download': { label: 'auto',    cls: 'bg-emerald-50 text-emerald-800 border border-emerald-200' },
    'legacy':        { label: 'legacy',  cls: 'bg-gray-100 text-gray-700 border border-gray-200' },
  }
  const src = sourceLabel[apk.source] || { label: apk.source, cls: 'bg-gray-100 text-gray-700 border border-gray-200' }
  return (
    <div className="px-5 py-4 flex items-start gap-4 hover:bg-gray-50/40 transition-colors">
      <div className="w-11 h-11 rounded-xl bg-forest-50 grid place-items-center text-forest-700 shrink-0">
        <FileBox className="w-5 h-5" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="font-semibold text-forest-950">
            {apk.app_label || apk.package_name || apk.filename}
          </span>
          <span className="inline-flex items-center text-[11px] font-mono font-semibold px-2 py-0.5 rounded-full bg-forest-50 text-forest-800 border border-forest-200">
            v{apk.version_name}
            {apk.version_code ? ` (${apk.version_code})` : ''}
          </span>
          {apk.is_default && (
            <span className="inline-flex items-center gap-1 text-[11px] font-bold uppercase tracking-widest px-2 py-0.5 rounded-full bg-lime-accent/40 text-forest-900 border border-lime-accent/60">
              <Star className="w-3 h-3 fill-forest-700 text-forest-700" />
              Padrão
            </span>
          )}
          <span className={`inline-flex items-center text-[10px] font-bold uppercase tracking-widest px-2 py-0.5 rounded-full ${src.cls}`}>
            {src.label}
          </span>
        </div>
        {apk.package_name && (
          <div className="text-xs text-gray-500 font-mono mt-0.5">{apk.package_name}</div>
        )}
        <div className="text-xs text-gray-500 mt-1.5 grid grid-cols-2 gap-x-4 gap-y-0.5">
          <div>Tamanho: <span className="font-mono text-gray-700">{apk.human_size}</span></div>
          <div>Adicionado: <span className="font-mono text-gray-700">{new Date(apk.created_at).toLocaleString('pt-BR')}</span></div>
          {(apk.min_sdk || apk.target_sdk) ? (
            <div>SDK: <span className="font-mono text-gray-700">min {apk.min_sdk || '?'} · target {apk.target_sdk || '?'}</span></div>
          ) : null}
          <div>Dispositivos atrelados: <span className="font-mono text-gray-700">{apk.devices_linked}</span></div>
        </div>
      </div>
      <div className="flex flex-col gap-2 items-end shrink-0">
        {!apk.is_default && (
          <button
            onClick={() => onSetDefault(apk.id)}
            className="px-3 py-1.5 rounded-lg border border-lime-accent/60 bg-lime-accent/30 text-forest-900 text-xs font-semibold hover:bg-lime-accent/50 active:bg-lime-accent inline-flex items-center gap-1.5 transition-colors"
            title="Marcar como padrão pra novos dispositivos"
          >
            <Star className="w-3.5 h-3.5" />
            Definir padrão
          </button>
        )}
      </div>
    </div>
  )
}
