import { useEffect, useState } from 'react'
import { Key, Plus, Trash2, Copy, AlertCircle, CheckCircle2, Eye, EyeOff, Smartphone, Layers, Send, Package, ChevronDown } from 'lucide-react'
import { APIToken, createToken, listTokens, revokeToken } from '../api/tokens'

export default function APIPage() {
  const [tokens, setTokens] = useState<APIToken[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [creating, setCreating] = useState(false)
  const [justCreated, setJustCreated] = useState<{ id: string; name: string; token: string } | null>(null)
  const [revealCreated, setRevealCreated] = useState(false)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const r = await listTokens()
      setTokens(r.data)
    } catch (e: any) {
      setError(e?.response?.data?.error || 'Falha ao carregar tokens')
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => { refresh() }, [])

  async function handleCreate() {
    setCreating(true)
    setError(null)
    try {
      const r = await createToken(newName.trim() || 'default')
      setJustCreated(r)
      setRevealCreated(true)
      setNewName('')
      setShowCreate(false)
      await refresh()
    } catch (e: any) {
      setError(e?.response?.data?.error || 'Falha ao criar token')
    } finally {
      setCreating(false)
    }
  }

  async function handleRevoke(id: string) {
    if (!confirm('Revogar este token? Aplicações que usam ele param de funcionar imediatamente.')) return
    try {
      await revokeToken(id)
      await refresh()
    } catch (e: any) {
      setError(e?.response?.data?.error || 'Falha ao revogar')
    }
  }

  function copy(text: string) {
    navigator.clipboard.writeText(text)
  }

  const baseURL = window.location.origin

  return (
    <div className="p-6 lg:p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-forest-950 tracking-tight flex items-center gap-2">
            <Key className="w-6 h-6 text-forest-700" />
            API
          </h1>
          <p className="text-sm text-gray-500 mt-1">
            Crie tokens para fazer chamadas programáticas (CRUD de dispositivos) sem precisar logar no painel.
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="btn-accent flex items-center gap-2"
        >
          <Plus className="w-4 h-4" />
          Novo token
        </button>
      </div>

      {error && (
        <div className="mb-4 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 flex gap-2 items-start">
          <AlertCircle className="w-5 h-5 text-rose-600 shrink-0 mt-0.5" />
          <div className="text-sm text-rose-800">{error}</div>
        </div>
      )}

      {/* Just-created token banner — only shows once */}
      {justCreated && (
        <div className="mb-4 rounded-2xl border-2 border-lime-accent/60 bg-lime-accent/15 p-5">
          <div className="flex items-start gap-2 mb-3">
            <CheckCircle2 className="w-5 h-5 text-forest-700 shrink-0 mt-0.5" />
            <div>
              <div className="font-semibold text-forest-950">Token criado: "{justCreated.name}"</div>
              <div className="text-xs text-forest-800 mt-0.5">
                <strong>Copie agora.</strong> Esse token não vai ser mostrado de novo — se perder, precisa criar outro.
              </div>
            </div>
          </div>
          <div className="bg-white rounded-lg border border-forest-200 p-3 flex items-center gap-2">
            <code className="flex-1 font-mono text-sm text-forest-950 break-all">
              {revealCreated ? justCreated.token : '•'.repeat(38)}
            </code>
            <button
              onClick={() => setRevealCreated(v => !v)}
              className="p-1.5 rounded-md text-gray-500 hover:text-forest-900 hover:bg-gray-100"
              title={revealCreated ? 'Ocultar' : 'Mostrar'}
            >
              {revealCreated ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
            <button
              onClick={() => copy(justCreated.token)}
              className="p-1.5 rounded-md text-forest-700 hover:text-forest-900 hover:bg-forest-50"
              title="Copiar"
            >
              <Copy className="w-4 h-4" />
            </button>
          </div>
          <button
            onClick={() => { setJustCreated(null); setRevealCreated(false) }}
            className="mt-3 text-xs text-forest-800 hover:text-forest-950 underline"
          >
            Já copiei, fechar
          </button>
        </div>
      )}

      {/* Token list */}
      <div className="card overflow-hidden mb-6">
        <div className="px-5 py-3 border-b border-gray-100 bg-gray-50/50 section-label">
          Seus tokens
        </div>
        {loading ? (
          <div className="px-5 py-6 text-sm text-gray-400">Carregando…</div>
        ) : tokens.length === 0 ? (
          <div className="px-5 py-6 text-sm text-gray-400 italic">Nenhum token criado ainda.</div>
        ) : (
          <table className="w-full text-sm">
            <thead className="text-[11px] uppercase tracking-widest text-gray-500 border-b border-gray-100">
              <tr>
                <th className="px-5 py-2.5 text-left font-bold">Nome</th>
                <th className="px-5 py-2.5 text-left font-bold">Prefixo</th>
                <th className="px-5 py-2.5 text-left font-bold">Último uso</th>
                <th className="px-5 py-2.5 text-left font-bold">Criado em</th>
                <th className="px-5 py-2.5 text-left font-bold">Status</th>
                <th className="px-5 py-2.5"></th>
              </tr>
            </thead>
            <tbody>
              {tokens.map(t => (
                <tr key={t.id} className="border-b border-gray-100 last:border-0 hover:bg-gray-50/50">
                  <td className="px-5 py-3 font-medium text-forest-950">{t.name}</td>
                  <td className="px-5 py-3 font-mono text-gray-700">{t.prefix}…</td>
                  <td className="px-5 py-3 text-gray-600">
                    {t.last_used_at ? new Date(t.last_used_at).toLocaleString('pt-BR') : <span className="italic text-gray-400">nunca</span>}
                  </td>
                  <td className="px-5 py-3 text-gray-600">
                    {new Date(t.created_at).toLocaleString('pt-BR')}
                  </td>
                  <td className="px-5 py-3">
                    {t.revoked_at ? (
                      <span className="inline-flex items-center gap-1.5 text-[11px] px-2 py-0.5 rounded-full bg-gray-100 text-gray-700 border border-gray-200">
                        <span className="w-1.5 h-1.5 rounded-full bg-gray-400" />revogado
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1.5 text-[11px] px-2 py-0.5 rounded-full bg-lime-accent/30 text-forest-900 border border-lime-accent/50">
                        <span className="w-1.5 h-1.5 rounded-full bg-forest-700" />ativo
                      </span>
                    )}
                  </td>
                  <td className="px-5 py-3 text-right">
                    {!t.revoked_at && (
                      <button
                        onClick={() => handleRevoke(t.id)}
                        className="p-1.5 rounded-md text-rose-600 hover:bg-rose-50 border border-transparent hover:border-rose-200 transition-colors"
                        title="Revogar"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Documentation */}
      <Documentation baseURL={baseURL} />

      {/* Create modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowCreate(false)} />
          <div className="relative bg-white rounded-2xl shadow-soft-lg w-full max-w-md p-6">
            <h2 className="text-lg font-bold text-forest-950 tracking-tight mb-4">Criar novo token</h2>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">Nome (apenas pra você identificar)</label>
            <input
              type="text"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="Ex: bot-disparos · zapier · meu-script"
              className="input-field mb-4"
              autoFocus
              maxLength={120}
              onKeyDown={(e) => { if (e.key === 'Enter') handleCreate() }}
            />
            <div className="flex justify-end gap-2">
              <button onClick={() => setShowCreate(false)} className="btn-secondary" disabled={creating}>
                Cancelar
              </button>
              <button onClick={handleCreate} className="btn-accent" disabled={creating}>
                {creating ? 'Criando…' : 'Criar token'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// ─────────── Documentation components ───────────

const methodColors: Record<string, string> = {
  GET:    'bg-emerald-600',
  POST:   'bg-forest-900',
  PATCH:  'bg-amber-600',
  PUT:    'bg-amber-600',
  DELETE: 'bg-rose-600',
}

function MethodBadge({ method }: { method: string }) {
  return (
    <span className={`inline-flex items-center justify-center px-2 py-0.5 rounded text-[10px] font-bold tracking-wider text-white shrink-0 w-14 ${methodColors[method] || 'bg-gray-600'}`}>
      {method}
    </span>
  )
}

interface EndpointProps {
  method: string
  path: string
  title: string
  description?: string
  curl: string
  response?: string
}

function Endpoint({ method, path, title, description, curl, response }: EndpointProps) {
  const [copied, setCopied] = useState(false)
  const [open, setOpen] = useState(false)
  return (
    <div className="border-t border-gray-100 first:border-t-0">
      <button
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center gap-3 px-5 py-3 hover:bg-gray-50/60 text-left transition-colors"
      >
        <MethodBadge method={method} />
        <code className="text-sm font-mono text-forest-950 truncate">{path}</code>
        <span className="text-xs text-gray-500 ml-2 truncate flex-1">{title}</span>
        <ChevronDown className={`w-4 h-4 text-gray-400 shrink-0 transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>
      {open && (
        <div className="px-5 pb-4 pt-1">
          {description && (
            <p className="text-xs text-gray-600 mb-3">{description}</p>
          )}
          <div className="mb-2 flex items-center justify-between">
            <span className="section-label">cURL</span>
            <button
              onClick={() => { navigator.clipboard.writeText(curl); setCopied(true); setTimeout(() => setCopied(false), 1500) }}
              className="text-xs text-forest-700 hover:text-forest-900 flex items-center gap-1"
            >
              <Copy className="w-3 h-3" />
              {copied ? 'Copiado!' : 'Copiar'}
            </button>
          </div>
          <pre className="bg-forest-950 text-gray-100 rounded-lg px-4 py-3 text-xs font-mono overflow-x-auto whitespace-pre">{curl}</pre>
          {response && (
            <>
              <div className="mt-3 mb-2 section-label">Resposta exemplo</div>
              <pre className="bg-gray-50 border border-gray-200 text-gray-800 rounded-lg px-4 py-3 text-xs font-mono overflow-x-auto whitespace-pre">{response}</pre>
            </>
          )}
        </div>
      )}
    </div>
  )
}

function Documentation({ baseURL }: { baseURL: string }) {
  const auth = `-H "Authorization: Bearer SEU_TOKEN"`
  const sections: { id: string; title: string; icon: React.ReactNode; description: string; endpoints: EndpointProps[] }[] = [
    {
      id: 'devices',
      title: 'Dispositivos · CRUD',
      icon: <Smartphone className="w-4 h-4" />,
      description: 'Operações básicas em cima dos dispositivos.',
      endpoints: [
        {
          method: 'GET', path: '/api/devices',
          title: 'Listar dispositivos do usuário',
          curl: `curl ${baseURL}/api/devices \\\n  ${auth}`,
          response: `{"data":[{"id":"...","name":"meu-device","status":"ready",...}]}`,
        },
        {
          method: 'POST', path: '/api/devices',
          title: 'Criar dispositivo (provisão automática)',
          description: 'Aceita opcionalmente "apk_id" pra escolher a versão. Sem isso, usa a padrão.',
          curl: `curl -X POST ${baseURL}/api/devices \\\n  ${auth} \\\n  -H "Content-Type: application/json" \\\n  -d '{"name":"meu-device-01"}'`,
          response: `{"data":{"id":"...","name":"meu-device-01","status":"creating",...}}`,
        },
        {
          method: 'GET', path: '/api/devices/{id}',
          title: 'Buscar um dispositivo específico',
          curl: `curl ${baseURL}/api/devices/<DEVICE_ID> \\\n  ${auth}`,
        },
        {
          method: 'PATCH', path: '/api/devices/{id}',
          title: 'Renomear dispositivo',
          curl: `curl -X PATCH ${baseURL}/api/devices/<DEVICE_ID> \\\n  ${auth} \\\n  -H "Content-Type: application/json" \\\n  -d '{"name":"novo-nome"}'`,
        },
        {
          method: 'DELETE', path: '/api/devices/{id}',
          title: 'Excluir dispositivo (para o container e remove)',
          curl: `curl -X DELETE ${baseURL}/api/devices/<DEVICE_ID> \\\n  ${auth}`,
        },
      ],
    },
    {
      id: 'control',
      title: 'Dispositivos · Controle',
      icon: <Layers className="w-4 h-4" />,
      description: 'Ciclo de vida e atualização de app.',
      endpoints: [
        {
          method: 'POST', path: '/api/devices/{id}/start',
          title: 'Iniciar dispositivo parado',
          curl: `curl -X POST ${baseURL}/api/devices/<DEVICE_ID>/start \\\n  ${auth}`,
        },
        {
          method: 'POST', path: '/api/devices/{id}/stop',
          title: 'Parar dispositivo (preserva dados)',
          curl: `curl -X POST ${baseURL}/api/devices/<DEVICE_ID>/stop \\\n  ${auth}`,
        },
        {
          method: 'POST', path: '/api/devices/{id}/update-app',
          title: 'Reinstalar APK atual no dispositivo (sem apagar dados)',
          curl: `curl -X POST ${baseURL}/api/devices/<DEVICE_ID>/update-app \\\n  ${auth}`,
          response: `{"ok":true,"installed":true,"size_bytes":138347345,"installed_apk_hash":"df1f..."}`,
        },
        {
          method: 'GET', path: '/api/devices/{id}/stats',
          title: 'Métricas (CPU, RAM, uptime)',
          curl: `curl ${baseURL}/api/devices/<DEVICE_ID>/stats \\\n  ${auth}`,
        },
      ],
    },
    {
      id: 'whatsapp',
      title: 'WhatsApp',
      icon: <Send className="w-4 h-4" />,
      description: 'Abrir o app, checar saúde, enviar mensagem (contato salvo ou não).',
      endpoints: [
        {
          method: 'POST', path: '/api/devices/{id}/whatsapp/open',
          title: 'Abrir WhatsApp e diagnosticar estado',
          description: 'Estados possíveis: ready · needs_registration · loading · banned · not_focused · unknown',
          curl: `curl -X POST ${baseURL}/api/devices/<DEVICE_ID>/whatsapp/open \\\n  ${auth}`,
          response: `{
  "ok": true,
  "state": "ready",
  "current_activity": "com.whatsapp/.Main",
  "version": "2.26.16.73",
  "package": "com.whatsapp",
  "note": "WhatsApp aberto e pronto"
}`,
        },
        {
          method: 'POST', path: '/api/devices/{id}/whatsapp/send',
          title: 'Enviar mensagem (abre chat e clica Enviar)',
          description: 'Funciona pra contato salvo OU não — usa deeplink wa.me. Detecta se o número não está no WhatsApp.',
          curl: `curl -X POST ${baseURL}/api/devices/<DEVICE_ID>/whatsapp/send \\\n  ${auth} \\\n  -H "Content-Type: application/json" \\\n  -d '{"phone":"5511999998888","message":"oi, tudo bem?"}'`,
          response: `{
  "ok": true,
  "phone": "5511999998888",
  "message": "oi, tudo bem?",
  "chat_loaded_after": "2.4s",
  "tapped_at": {"X":1010,"Y":1690},
  "note": "mensagem enviada"
}`,
        },
      ],
    },
    {
      id: 'apks',
      title: 'APKs · Versões',
      icon: <Package className="w-4 h-4" />,
      description: 'Gestão de versões do WhatsApp (também via página "Aplicativo APK").',
      endpoints: [
        {
          method: 'GET', path: '/api/apks',
          title: 'Listar todas as versões armazenadas',
          curl: `curl ${baseURL}/api/apks \\\n  ${auth}`,
        },
        {
          method: 'POST', path: '/api/apks',
          title: 'Subir novo APK (multipart, campo "apk")',
          curl: `curl -X POST ${baseURL}/api/apks \\\n  ${auth} \\\n  -F "apk=@/caminho/whatsapp.apk"`,
        },
        {
          method: 'POST', path: '/api/apks/check-update',
          title: 'Verificar nova versão no whatsapp.com (também roda 1× ao dia)',
          curl: `curl -X POST ${baseURL}/api/apks/check-update \\\n  ${auth}`,
          response: `{"ok":true,"message":"nova versão baixada · v2.26.17.73 (anterior: v2.26.16.73)","new_apk_id":"..."}`,
        },
        {
          method: 'PATCH', path: '/api/apks/{id}/default',
          title: 'Marcar uma versão como padrão pra novos dispositivos',
          curl: `curl -X PATCH ${baseURL}/api/apks/<APK_ID>/default \\\n  ${auth}`,
        },
      ],
    },
    {
      id: 'tokens',
      title: 'Tokens (auto-gestão)',
      icon: <Key className="w-4 h-4" />,
      description: 'Você também pode criar/revogar tokens via API (útil pra rotação automática).',
      endpoints: [
        {
          method: 'GET', path: '/api/tokens',
          title: 'Listar seus tokens',
          curl: `curl ${baseURL}/api/tokens \\\n  ${auth}`,
        },
        {
          method: 'POST', path: '/api/tokens',
          title: 'Criar novo token (retorna o plaintext UMA vez)',
          curl: `curl -X POST ${baseURL}/api/tokens \\\n  ${auth} \\\n  -H "Content-Type: application/json" \\\n  -d '{"name":"meu-bot"}'`,
          response: `{"id":"...","name":"meu-bot","token":"astra_336ffb5cb7d084780ec864b398c0f7b4"}`,
        },
        {
          method: 'DELETE', path: '/api/tokens/{id}',
          title: 'Revogar token',
          curl: `curl -X DELETE ${baseURL}/api/tokens/<TOKEN_ID> \\\n  ${auth}`,
        },
      ],
    },
  ]

  return (
    <div className="card overflow-hidden">
      <div className="px-5 py-4 border-b border-gray-100">
        <h2 className="text-base font-bold text-forest-950 tracking-tight">Como usar a API</h2>
        <p className="text-sm text-gray-500 mt-1">
          Inclua o header{' '}
          <code className="font-mono text-xs bg-gray-100 px-1.5 py-0.5 rounded text-forest-900">
            Authorization: Bearer &lt;token&gt;
          </code>
          {' '}em todas as requisições. Base URL:{' '}
          <code className="font-mono text-xs bg-gray-100 px-1.5 py-0.5 rounded text-forest-900">{baseURL}</code>
        </p>
      </div>

      {/* Index */}
      <div className="px-5 py-3 border-b border-gray-100 bg-gray-50/50 flex flex-wrap gap-2">
        {sections.map(s => (
          <a
            key={s.id}
            href={`#sec-${s.id}`}
            className="inline-flex items-center gap-1.5 text-xs font-medium text-gray-700 hover:text-forest-900 px-2.5 py-1 rounded-md bg-white border border-gray-200 hover:border-forest-300 transition-colors"
          >
            {s.icon}
            {s.title}
            <span className="text-[10px] font-mono text-gray-400">({s.endpoints.length})</span>
          </a>
        ))}
      </div>

      {/* Sections */}
      <div className="divide-y divide-gray-100">
        {sections.map(s => (
          <div key={s.id} id={`sec-${s.id}`} className="scroll-mt-4">
            <div className="px-5 py-3 bg-gray-50/50 border-b border-gray-100 flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-lg bg-forest-50 grid place-items-center text-forest-700">
                {s.icon}
              </div>
              <div className="flex-1">
                <div className="font-semibold text-forest-950 text-sm">{s.title}</div>
                <div className="text-xs text-gray-500">{s.description}</div>
              </div>
            </div>
            <div>
              {s.endpoints.map((ep, i) => (
                <Endpoint key={i} {...ep} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
