import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Eye, Play, Square, Trash2, Smartphone, RotateCw } from 'lucide-react';
import { Device, DeviceStatus } from '../types';
import { startDevice, stopDevice, deleteDevice } from '../api/devices';
import { updateDeviceApp } from '../api/apk';

interface DeviceCardProps {
  device: Device;
  onRefresh: () => void;
  /** SHA-256 do APK atualmente uploaded. Se diferente do device.installed_apk_hash,
   *  mostramos o botão "Atualizar app". */
  currentAPKHash?: string;
}

const stageMap: Record<string, { pct: number; label: string }> = {
  creating: { pct: 20, label: 'Criando container' },
  booting: { pct: 55, label: 'Inicializando Android' },
  installing: { pct: 85, label: 'Instalando WhatsApp' },
};

function ProvisioningProgress({ status }: { status: DeviceStatus }) {
  const stage = stageMap[status] || { pct: 0, label: '' };
  return (
    <div className="mb-3">
      <div className="flex items-center justify-between mb-1">
        <span className="text-[11px] font-medium text-gray-700">{stage.label}</span>
        <span className="text-[11px] font-mono text-gray-500">{stage.pct}%</span>
      </div>
      <div className="h-1.5 w-full bg-gray-100 rounded-full overflow-hidden">
        <div
          className="h-full bg-gradient-to-r from-indigo-400 to-violet-500 rounded-full transition-all duration-500 relative overflow-hidden"
          style={{ width: `${stage.pct}%` }}
        >
          <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/30 to-transparent animate-pulse" />
        </div>
      </div>
    </div>
  );
}

const statusConfig: Record<
  DeviceStatus,
  { label: string; color: string; bgColor: string; dot: string }
> = {
  creating:   { label: 'Criando',       color: 'text-amber-800',  bgColor: 'bg-amber-50 border border-amber-200',   dot: 'bg-amber-500' },
  booting:    { label: 'Inicializando', color: 'text-amber-800',  bgColor: 'bg-amber-50 border border-amber-200',   dot: 'bg-amber-500' },
  installing: { label: 'Instalando',    color: 'text-amber-800',  bgColor: 'bg-amber-50 border border-amber-200',   dot: 'bg-amber-500' },
  ready:      { label: 'Pronto',        color: 'text-forest-900', bgColor: 'bg-lime-accent/30 border border-lime-accent/50', dot: 'bg-forest-700' },
  stopped:    { label: 'Parado',        color: 'text-gray-700',   bgColor: 'bg-gray-100 border border-gray-200',    dot: 'bg-gray-500' },
  error:      { label: 'Erro',          color: 'text-rose-800',   bgColor: 'bg-rose-50 border border-rose-200',     dot: 'bg-rose-500' },
};

export default function DeviceCard({ device, onRefresh, currentAPKHash }: DeviceCardProps) {
  const navigate = useNavigate();
  const [actionLoading, setActionLoading] = useState(false);
  const [updateMsg, setUpdateMsg] = useState<{ kind: 'ok' | 'err'; text: string } | null>(null);
  const status = statusConfig[device.status] || statusConfig.error;

  // Botão de atualizar app só aparece se:
  //  1. device está pronto pra receber comando ADB
  //  2. existe um APK uploaded (currentAPKHash não vazio)
  //  3. o hash do APK é diferente do que tá instalado no device
  //     (= device foi criado antes do APK atual OU usuário subiu APK novo
  //      e ainda não rodou update neste device)
  const showUpdateApp =
    device.status === 'ready' &&
    !!currentAPKHash &&
    currentAPKHash !== device.installed_apk_hash;

  const handleUpdateApp = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirm(`Atualizar o WhatsApp em "${device.name}" com o APK atual? Isso reinstala o app sem apagar dados.`)) return;
    setActionLoading(true);
    setUpdateMsg(null);
    try {
      await updateDeviceApp(device.id);
      setUpdateMsg({ kind: 'ok', text: 'App atualizado' });
      setTimeout(() => setUpdateMsg(null), 4000);
      // refresh pra puxar novo installed_apk_hash → botão some
      onRefresh();
    } catch (err: any) {
      const msg = err?.response?.data?.error || err?.message || 'Erro ao atualizar';
      setUpdateMsg({ kind: 'err', text: msg });
      setTimeout(() => setUpdateMsg(null), 6000);
    } finally {
      setActionLoading(false);
    }
  };

  const handleAction = async (
    action: () => Promise<unknown>,
    e: React.MouseEvent
  ) => {
    e.stopPropagation();
    setActionLoading(true);
    try {
      await action();
      onRefresh();
    } catch {
      // error handled silently; refresh will show current state
    } finally {
      setActionLoading(false);
    }
  };

  const createdDate = new Date(device.created_at).toLocaleDateString(
    'pt-BR',
    {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    }
  );

  return (
    <div className="card card-hover cursor-pointer group">
      <div className="p-5">
        <div className="flex items-start justify-between mb-3">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-forest-50 flex items-center justify-center">
              <Smartphone className="w-5 h-5 text-forest-700" />
            </div>
            <div>
              <h3 className="font-semibold text-forest-950">{device.name}</h3>
              <p className="text-xs text-gray-500">
                Android {device.android_version || 'N/A'}
              </p>
            </div>
          </div>
          <div className="flex flex-col items-end gap-1.5">
            <span
              className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-semibold ${status.bgColor} ${status.color}`}
            >
              <span className={`w-1.5 h-1.5 rounded-full ${status.dot}`} />
              {status.label}
            </span>
            {showUpdateApp && (
              <button
                onClick={handleUpdateApp}
                disabled={actionLoading}
                title="Atualização disponível · clique pra instalar o APK novo neste device"
                className="w-7 h-7 grid place-items-center rounded-full text-violet-700 bg-violet-100 hover:bg-violet-200 ring-2 ring-violet-300 ring-offset-1 transition-colors disabled:opacity-50 animate-pulse"
              >
                <RotateCw className={`w-3.5 h-3.5 ${actionLoading ? 'animate-spin' : ''}`} />
              </button>
            )}
          </div>
        </div>

        {device.error_message && (
          <p className="text-xs text-red-600 mb-3 bg-red-50 p-2 rounded">
            {device.error_message}
          </p>
        )}

        {updateMsg && (
          <p className={`text-xs mb-3 p-2 rounded ${updateMsg.kind === 'ok' ? 'text-green-700 bg-green-50' : 'text-red-600 bg-red-50'}`}>
            {updateMsg.text}
          </p>
        )}

        {/* Provisioning progress — derives from device.status, works for any source (panel/API) */}
        {(['creating', 'booting', 'installing'] as DeviceStatus[]).includes(device.status) && (
          <ProvisioningProgress status={device.status} />
        )}

        <p className="text-xs text-gray-400 mb-4">Criado em {createdDate}</p>

        <div className="flex items-center gap-2 border-t border-gray-100 pt-3">
          {device.status === 'ready' && (
            <>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  navigate(`/devices/${device.id}`);
                }}
                disabled={actionLoading}
                className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-indigo-700 bg-indigo-50 rounded-lg hover:bg-indigo-100 transition-colors disabled:opacity-50"
              >
                <Eye className="w-3.5 h-3.5" />
                Ver
              </button>
              <button
                onClick={(e) => handleAction(() => stopDevice(device.id), e)}
                disabled={actionLoading}
                className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-orange-700 bg-orange-50 rounded-lg hover:bg-orange-100 transition-colors disabled:opacity-50"
              >
                <Square className="w-3.5 h-3.5" />
                Parar
              </button>
            </>
          )}
          {device.status === 'stopped' && (
            <button
              onClick={(e) => handleAction(() => startDevice(device.id), e)}
              disabled={actionLoading}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-green-700 bg-green-50 rounded-lg hover:bg-green-100 transition-colors disabled:opacity-50"
            >
              <Play className="w-3.5 h-3.5" />
              Iniciar
            </button>
          )}
          <div className="flex-1" />
          <button
            onClick={(e) => handleAction(() => deleteDevice(device.id), e)}
            disabled={actionLoading}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-red-700 bg-red-50 rounded-lg hover:bg-red-100 transition-colors disabled:opacity-50"
          >
            <Trash2 className="w-3.5 h-3.5" />
            Excluir
          </button>
        </div>
      </div>
    </div>
  );
}
