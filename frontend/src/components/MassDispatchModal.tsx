import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { X, Upload, Send, Smartphone, Clock, AlertCircle, CheckCircle2 } from 'lucide-react'
import { listDevices } from '../api/devices'
import { Device } from '../types'
import { createCampaign, startCampaign } from '../api/campaigns'

interface Props {
  open: boolean
  onClose: () => void
}

export default function MassDispatchModal({ open, onClose }: Props) {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [message, setMessage] = useState('')
  const [delayMin, setDelayMin] = useState(30)
  const [delayMax, setDelayMax] = useState(90)
  const [csvFile, setCsvFile] = useState<File | null>(null)
  const [csvPreview, setCsvPreview] = useState<{ count: number; sample: string[] } | null>(null)
  const [devices, setDevices] = useState<Device[]>([])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!open) return
    listDevices().then(r => {
      const ready = r.data.filter(d => d.status === 'ready')
      setDevices(ready)
      // pre-select all ready devices
      setSelected(new Set(ready.map(d => d.id)))
    })
  }, [open])

  if (!open) return null

  function handleCSV(file: File) {
    setError(null)
    if (!file.name.toLowerCase().match(/\.(csv|txt)$/)) {
      setError('Arquivo precisa ser .csv ou .txt')
      return
    }
    setCsvFile(file)
    // preview: count lines, show first 3
    const reader = new FileReader()
    reader.onload = (e) => {
      const text = String(e.target?.result || '')
      const lines = text.split(/\r?\n/).filter(l => l.trim())
      const sample = lines.slice(0, 3)
      setCsvPreview({ count: lines.length, sample })
    }
    reader.readAsText(file.slice(0, 10000))
  }

  function toggleDevice(id: string) {
    const next = new Set(selected)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    setSelected(next)
  }
  function selectAll() {
    setSelected(new Set(devices.map(d => d.id)))
  }
  function selectNone() {
    setSelected(new Set())
  }

  async function handleSubmit(startAfter: boolean) {
    setError(null)
    if (!csvFile) { setError('Suba um CSV ou lista de números'); return }
    if (!message.trim()) { setError('Mensagem é obrigatória'); return }
    if (selected.size === 0) { setError('Selecione pelo menos 1 dispositivo'); return }
    if (delayMax < delayMin) { setError('Delay máximo precisa ser ≥ mínimo'); return }
    setLoading(true)
    try {
      const r = await createCampaign(csvFile, {
        name: name.trim() || 'Campanha sem nome',
        message: message.trim(),
        delay_min_seconds: delayMin,
        delay_max_seconds: delayMax,
        device_ids: Array.from(selected),
      })
      const campID = r.data.id
      if (startAfter) {
        await startCampaign(campID)
      }
      onClose()
      navigate(`/campaigns/${campID}`)
    } catch (e: any) {
      setError(e?.response?.data?.error || 'Erro ao criar campanha')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-4 py-8 overflow-auto">
      <div className="absolute inset-0 bg-forest-950/40 backdrop-blur-sm" onClick={onClose} />
      <div className="relative bg-white rounded-2xl shadow-soft-lg w-full max-w-2xl">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <h2 className="text-lg font-bold text-forest-950 tracking-tight flex items-center gap-2">
            <Send className="w-5 h-5 text-forest-700" />
            Disparo em massa
          </h2>
          <button onClick={onClose} className="p-1 rounded hover:bg-gray-100 text-gray-400">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="p-6 space-y-5 max-h-[70vh] overflow-y-auto">
          {error && (
            <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 flex items-start gap-2 text-sm text-red-800">
              <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" /> {error}
            </div>
          )}

          {/* Name */}
          <div>
            <label className="block text-xs font-bold uppercase tracking-widest text-gray-500 mb-1">
              Nome da campanha
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Ex: Promoção Black Friday"
              className="input-field"
              maxLength={120}
            />
          </div>

          {/* CSV upload */}
          <div>
            <label className="block text-xs font-bold uppercase tracking-widest text-gray-500 mb-1">
              Lista de contatos (CSV ou TXT)
            </label>
            <div
              className={`rounded-lg border-2 border-dashed p-4 text-center cursor-pointer transition-colors ${
                csvFile ? 'border-lime-accent/60 bg-lime-accent/10' : 'border-gray-200 hover:border-forest-300'
              }`}
              onClick={() => fileRef.current?.click()}
              onDragOver={(e) => e.preventDefault()}
              onDrop={(e) => {
                e.preventDefault()
                const f = e.dataTransfer.files?.[0]
                if (f) handleCSV(f)
              }}
            >
              <input
                ref={fileRef}
                type="file"
                accept=".csv,.txt,text/csv,text/plain"
                className="hidden"
                onChange={(e) => { const f = e.target.files?.[0]; if (f) handleCSV(f); e.target.value = '' }}
              />
              {csvFile ? (
                <div className="flex items-center justify-center gap-2 text-sm text-forest-900">
                  <CheckCircle2 className="w-4 h-4" />
                  <span className="font-medium">{csvFile.name}</span>
                  {csvPreview && (
                    <span className="text-xs text-emerald-600">· ~{csvPreview.count} linhas</span>
                  )}
                </div>
              ) : (
                <div className="text-sm text-gray-600">
                  <Upload className="w-5 h-5 mx-auto mb-1 text-gray-400" />
                  Clique pra selecionar ou arraste o arquivo
                  <div className="text-[11px] text-gray-400 mt-1">
                    Formato: header com coluna <code className="font-mono">phone</code> (e opcional <code className="font-mono">name</code>) ·
                    ou apenas 1 número por linha
                  </div>
                </div>
              )}
            </div>
            {csvPreview && csvPreview.sample.length > 0 && (
              <div className="mt-2 text-[11px] font-mono text-gray-500 bg-gray-50 rounded px-2 py-1 max-h-16 overflow-auto">
                {csvPreview.sample.map((l, i) => <div key={i}>{l}</div>)}
                {csvPreview.count > 3 && <div className="text-gray-400">… e mais {csvPreview.count - 3}</div>}
              </div>
            )}
          </div>

          {/* Message */}
          <div>
            <label className="block text-xs font-bold uppercase tracking-widest text-gray-500 mb-1">
              Mensagem
            </label>
            <textarea
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              rows={4}
              placeholder="Olá {{name}}! Tudo bem?"
              className="input-field font-sans"
            />
            <div className="text-[11px] text-gray-500 mt-1">
              Variáveis disponíveis: <code className="font-mono">{'{{name}}'}</code>{' '}
              <code className="font-mono">{'{{phone}}'}</code>
            </div>
          </div>

          {/* Devices */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="text-xs font-bold uppercase tracking-widest text-gray-500">
                Dispositivos pra disparar ({selected.size}/{devices.length})
              </label>
              <div className="flex gap-2 text-xs">
                <button onClick={selectAll} className="text-forest-700 hover:text-forest-900 hover:underline">todos</button>
                <span className="text-gray-300">·</span>
                <button onClick={selectNone} className="text-gray-500 hover:underline">nenhum</button>
              </div>
            </div>
            {devices.length === 0 ? (
              <div className="text-xs text-gray-500 italic px-3 py-2 rounded border border-gray-200 bg-gray-50">
                Nenhum dispositivo está pronto. Crie e aguarde ficar "Pronto" antes de disparar.
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-1.5 max-h-40 overflow-auto">
                {devices.map(d => {
                  const checked = selected.has(d.id)
                  return (
                    <label
                      key={d.id}
                      className={`flex items-center gap-2 px-2.5 py-1.5 rounded border cursor-pointer text-sm ${
                        checked ? 'border-forest-700 bg-forest-50' : 'border-gray-200 bg-white hover:bg-gray-50'
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => toggleDevice(d.id)}
                        className="accent-forest-700"
                      />
                      <Smartphone className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                      <span className="truncate">{d.name}</span>
                    </label>
                  )
                })}
              </div>
            )}
          </div>

          {/* Delays */}
          <div>
            <label className="block text-xs font-bold uppercase tracking-widest text-gray-500 mb-1 flex items-center gap-1">
              <Clock className="w-3 h-3" /> Intervalo entre disparos (segundos)
            </label>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <div className="text-[11px] text-gray-500 mb-0.5">Mínimo</div>
                <input
                  type="number"
                  min="1"
                  value={delayMin}
                  onChange={(e) => setDelayMin(Math.max(1, Number(e.target.value) || 1))}
                  className="input-field"
                />
              </div>
              <div>
                <div className="text-[11px] text-gray-500 mb-0.5">Máximo</div>
                <input
                  type="number"
                  min={delayMin}
                  value={delayMax}
                  onChange={(e) => setDelayMax(Math.max(delayMin, Number(e.target.value) || delayMin))}
                  className="input-field"
                />
              </div>
            </div>
            <div className="text-[11px] text-gray-500 mt-1">
              Cada device sorteia um valor aleatório nesse intervalo entre uma mensagem e a próxima.
            </div>
          </div>
        </div>

        <div className="flex justify-end gap-2 px-6 py-4 border-t border-gray-100 bg-gray-50/50 rounded-b-2xl">
          <button onClick={onClose} className="btn-secondary" disabled={loading}>Cancelar</button>
          <button
            onClick={() => handleSubmit(false)}
            disabled={loading}
            className="px-4 py-2 rounded-lg border border-gray-300 bg-white text-gray-700 text-sm font-medium hover:bg-gray-100 disabled:opacity-60"
            title="Salva como rascunho — você inicia depois"
          >
            Salvar rascunho
          </button>
          <button
            onClick={() => handleSubmit(true)}
            disabled={loading}
            className="btn-accent"
          >
            {loading ? 'Criando…' : 'Criar e iniciar'}
          </button>
        </div>
      </div>
    </div>
  )
}
