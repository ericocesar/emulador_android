import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Home,
  ArrowLeft,
  LayoutGrid,
  Send,
  Power,
  RotateCw,
  Trash2,
  AlertTriangle,
  X,
} from 'lucide-react';
import { InputMessage } from '../types';
import { Device } from '../types';
import { startDevice, stopDevice, deleteDevice } from '../api/devices';

interface DeviceControlsProps {
  device: Device;
  sendInput: (msg: InputMessage) => void;
  connected: boolean;
  onChanged?: () => void;
}

interface NavButton {
  label: string;
  icon: React.ReactNode;
  keycode: number;
}

const navButtons: NavButton[] = [
  { label: 'Voltar', icon: <ArrowLeft className="w-5 h-5" />, keycode: 4 },
  { label: 'Início', icon: <Home className="w-5 h-5" />, keycode: 3 },
  { label: 'Recentes', icon: <LayoutGrid className="w-5 h-5" />, keycode: 187 },
];

export default function DeviceControls({
  device,
  sendInput,
  connected,
  onChanged,
}: DeviceControlsProps) {
  const navigate = useNavigate();
  const [text, setText] = useState('');
  const [busy, setBusy] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const sendKey = (keycode: number) => {
    sendInput({ type: 'keyevent', keycode });
  };

  const sendText = () => {
    if (text.trim()) {
      sendInput({ type: 'text', text: text.trim() });
      setText('');
    }
  };

  async function doStop() {
    setErr(null); setBusy('stop')
    try { await stopDevice(device.id); onChanged?.() }
    catch (e: any) { setErr(e?.response?.data?.error || 'Erro ao desligar') }
    finally { setBusy(null) }
  }
  async function doStart() {
    setErr(null); setBusy('start')
    try { await startDevice(device.id); onChanged?.() }
    catch (e: any) { setErr(e?.response?.data?.error || 'Erro ao ligar') }
    finally { setBusy(null) }
  }
  async function doRestart() {
    setErr(null); setBusy('restart')
    try {
      await stopDevice(device.id)
      await new Promise((r) => setTimeout(r, 1500))
      await startDevice(device.id)
      onChanged?.()
    } catch (e: any) {
      setErr(e?.response?.data?.error || 'Erro ao reiniciar')
    } finally { setBusy(null) }
  }
  async function doDelete() {
    setErr(null); setBusy('delete')
    try {
      await deleteDevice(device.id)
      navigate('/dashboard')
    } catch (e: any) {
      setErr(e?.response?.data?.error || 'Erro ao excluir')
      setBusy(null)
    }
  }

  const isReady = device.status === 'ready'
  const isStopped = device.status === 'stopped'

  return (
    <div className="space-y-4">
      {/* Navigation */}
      <div className="card p-4">
        <h3 className="section-label mb-3">Navegação</h3>
        <div className="flex gap-2">
          {navButtons.map((btn) => (
            <button
              key={btn.keycode}
              onClick={() => sendKey(btn.keycode)}
              disabled={!connected}
              title={btn.label}
              className="flex-1 flex flex-col items-center gap-1.5 py-3 rounded-lg bg-gray-50 hover:bg-forest-50 hover:text-forest-900 text-gray-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed border border-gray-100"
            >
              {btn.icon}
              <span className="text-[10px] font-medium">{btn.label}</span>
            </button>
          ))}
        </div>
      </div>

      {/* System actions */}
      <div className="card p-4">
        <h3 className="section-label mb-3">Sistema</h3>
        {err && (
          <div className="mb-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-800">
            {err}
          </div>
        )}
        <div className="grid grid-cols-3 gap-2">
          {/* Power toggle (Desligar/Ligar) */}
          {isReady && (
            <button
              onClick={doStop}
              disabled={busy !== null}
              title="Desligar dispositivo (preserva dados)"
              className="flex flex-col items-center gap-1.5 py-3 rounded-lg bg-amber-50 hover:bg-amber-100 text-amber-800 transition-colors disabled:opacity-40 border border-amber-200"
            >
              {busy === 'stop' ? (
                <span className="w-5 h-5 rounded-full border-2 border-amber-300 border-t-amber-700 animate-spin" />
              ) : (
                <Power className="w-5 h-5" />
              )}
              <span className="text-[10px] font-medium">Desligar</span>
            </button>
          )}
          {isStopped && (
            <button
              onClick={doStart}
              disabled={busy !== null}
              title="Ligar dispositivo"
              className="flex flex-col items-center gap-1.5 py-3 rounded-lg bg-lime-accent/30 hover:bg-lime-accent/50 text-forest-900 transition-colors disabled:opacity-40 border border-lime-accent/60"
            >
              {busy === 'start' ? (
                <span className="w-5 h-5 rounded-full border-2 border-forest-300 border-t-forest-700 animate-spin" />
              ) : (
                <Power className="w-5 h-5" />
              )}
              <span className="text-[10px] font-medium">Ligar</span>
            </button>
          )}
          {!isReady && !isStopped && (
            <div className="flex flex-col items-center gap-1.5 py-3 rounded-lg bg-gray-50 text-gray-400 border border-gray-100">
              <Power className="w-5 h-5" />
              <span className="text-[10px] font-medium">{device.status}</span>
            </div>
          )}

          {/* Restart */}
          <button
            onClick={doRestart}
            disabled={busy !== null || !isReady}
            title="Reiniciar dispositivo (desliga e liga)"
            className="flex flex-col items-center gap-1.5 py-3 rounded-lg bg-forest-50 hover:bg-forest-100 text-forest-800 transition-colors disabled:opacity-40 border border-forest-100"
          >
            {busy === 'restart' ? (
              <span className="w-5 h-5 rounded-full border-2 border-forest-300 border-t-forest-700 animate-spin" />
            ) : (
              <RotateCw className="w-5 h-5" />
            )}
            <span className="text-[10px] font-medium">Reiniciar</span>
          </button>

          {/* Delete */}
          <button
            onClick={() => setConfirmDelete(true)}
            disabled={busy !== null}
            title="Excluir dispositivo permanentemente"
            className="flex flex-col items-center gap-1.5 py-3 rounded-lg bg-rose-50 hover:bg-rose-100 text-rose-700 transition-colors disabled:opacity-40 border border-rose-200"
          >
            <Trash2 className="w-5 h-5" />
            <span className="text-[10px] font-medium">Excluir</span>
          </button>
        </div>
      </div>

      {/* Text input */}
      <div className="card p-4">
        <h3 className="section-label mb-3">Digitar texto</h3>
        <div className="flex gap-2">
          <input
            type="text"
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); sendText(); } }}
            disabled={!connected}
            placeholder="Digite o texto…"
            className="input-field text-sm flex-1"
          />
          <button
            onClick={sendText}
            disabled={!connected || !text.trim()}
            className="btn-primary px-3 py-2 disabled:opacity-50"
          >
            <Send className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Delete confirmation modal */}
      {confirmDelete && (
        <DeleteConfirmModal
          deviceName={device.name}
          onCancel={() => setConfirmDelete(false)}
          onConfirm={async () => {
            setConfirmDelete(false)
            await doDelete()
          }}
        />
      )}
    </div>
  );
}

function DeleteConfirmModal({
  deviceName,
  onCancel,
  onConfirm,
}: {
  deviceName: string;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const [typed, setTyped] = useState('');
  const matches = typed.trim() === deviceName.trim();

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
      <div className="absolute inset-0 bg-forest-950/40 backdrop-blur-sm" onClick={onCancel} />
      <div className="relative bg-white rounded-2xl shadow-soft-lg w-full max-w-md p-6">
        <div className="flex items-start gap-3 mb-4">
          <div className="w-10 h-10 rounded-xl bg-rose-50 grid place-items-center text-rose-700 shrink-0">
            <AlertTriangle className="w-5 h-5" />
          </div>
          <div className="flex-1">
            <h2 className="text-lg font-bold text-forest-950 tracking-tight">Excluir dispositivo</h2>
            <p className="text-sm text-gray-600 mt-1">
              Esta ação <strong className="text-rose-700">não pode ser desfeita</strong>. O container Docker, dados do
              Android (incluindo a sessão do WhatsApp) e o registro no banco serão removidos.
            </p>
          </div>
          <button onClick={onCancel} className="p-1 rounded text-gray-400 hover:text-gray-700">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="rounded-lg bg-rose-50 border border-rose-200 px-3 py-2 mb-4 text-xs text-rose-800">
          Pra confirmar, digite o nome do dispositivo abaixo:
          <div className="font-mono font-bold mt-1 text-rose-900">{deviceName}</div>
        </div>

        <input
          type="text"
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          placeholder="Digite o nome exato"
          className="input-field font-mono mb-4"
          autoFocus
          onKeyDown={(e) => {
            if (e.key === 'Enter' && matches) onConfirm();
            if (e.key === 'Escape') onCancel();
          }}
        />

        <div className="flex justify-end gap-2">
          <button onClick={onCancel} className="btn-secondary">Cancelar</button>
          <button
            onClick={onConfirm}
            disabled={!matches}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
              matches
                ? 'bg-rose-600 text-white hover:bg-rose-700 active:bg-rose-800'
                : 'bg-rose-200 text-rose-400 cursor-not-allowed'
            }`}
          >
            Excluir definitivamente
          </button>
        </div>
      </div>
    </div>
  );
}
