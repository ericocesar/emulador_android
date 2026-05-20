import React, { useEffect, useState } from 'react';
import { Plus, Smartphone, Activity, CheckCircle2, AlertCircle, RefreshCw } from 'lucide-react';
import { useDevices } from '../hooks/useDevices';
import DeviceCard from '../components/DeviceCard';
import CreateDeviceModal from '../components/CreateDeviceModal';
import { getAPKInfo } from '../api/apk';
import { listCampaigns } from '../api/campaigns';

export default function DashboardPage() {
  const { devices, loading, refresh } = useDevices();
  const [modalOpen, setModalOpen] = useState(false);
  const [currentAPKHash, setCurrentAPKHash] = useState<string>('');
  const [campaignsRunning, setCampaignsRunning] = useState(0);
  const [totalSent, setTotalSent] = useState(0);

  useEffect(() => {
    const fetchAPK = () => {
      getAPKInfo()
        .then((info) => setCurrentAPKHash(info.exists && info.sha256 ? info.sha256 : ''))
        .catch(() => setCurrentAPKHash(''));
    };
    fetchAPK();
    const onFocus = () => fetchAPK();
    window.addEventListener('focus', onFocus);
    return () => window.removeEventListener('focus', onFocus);
  }, []);

  useEffect(() => {
    const fetchCamp = () => {
      listCampaigns()
        .then((r) => {
          setCampaignsRunning(r.data.filter((c) => c.status === 'running').length);
          setTotalSent(r.data.reduce((acc, c) => acc + c.sent, 0));
        })
        .catch(() => {});
    };
    fetchCamp();
    const t = setInterval(fetchCamp, 10_000);
    return () => clearInterval(t);
  }, []);

  const ready = devices.filter((d) => d.status === 'ready').length;
  const provisioning = devices.filter((d) =>
    ['creating', 'booting', 'installing'].includes(d.status)
  ).length;
  const errors = devices.filter((d) => d.status === 'error').length;

  return (
    <div className="p-6 lg:p-8">
      {/* Header */}
      <div className="flex items-start justify-between mb-6 gap-4 flex-wrap">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold text-forest-950 tracking-tight">Dispositivos</h1>
          <p className="text-sm text-gray-500 mt-1">
            Gerencie seus emuladores Android e dispare campanhas de WhatsApp.
          </p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={refresh}
            className="p-2 rounded-lg text-gray-500 hover:text-forest-900 hover:bg-white border border-transparent hover:border-gray-200 transition-colors"
            title="Recarregar"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => setModalOpen(true)}
            className="btn-accent inline-flex items-center gap-2"
          >
            <Plus className="w-4 h-4" />
            Novo dispositivo
          </button>
        </div>
      </div>

      {/* Hero card */}
      <div className="rounded-2xl bg-forest-900 text-white p-6 mb-6 relative overflow-hidden">
        {/* decorative orbs */}
        <div className="absolute -top-20 -right-20 w-64 h-64 rounded-full bg-lime-accent/10 blur-3xl" />
        <div className="absolute -bottom-32 -left-10 w-72 h-72 rounded-full bg-forest-700/40 blur-3xl" />

        <div className="relative grid grid-cols-1 md:grid-cols-4 gap-6 items-center">
          <div className="md:col-span-2">
            <div className="text-xs font-bold uppercase tracking-[0.18em] text-lime-accent/80 mb-1.5">
              Painel · Visão geral
            </div>
            <div className="text-5xl md:text-6xl font-extrabold tracking-tight leading-none">
              {devices.length}
              <span className="text-2xl font-medium text-white/60 ml-2">dispositivos</span>
            </div>
            <div className="mt-3 flex items-center gap-3 text-sm">
              <span className="inline-flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-lime-accent" />
                <span className="font-medium">{ready}</span>
                <span className="text-white/60">prontos</span>
              </span>
              {provisioning > 0 && (
                <span className="inline-flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-yellow-400 animate-pulse" />
                  <span className="font-medium">{provisioning}</span>
                  <span className="text-white/60">provisionando</span>
                </span>
              )}
              {errors > 0 && (
                <span className="inline-flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-rose-400" />
                  <span className="font-medium">{errors}</span>
                  <span className="text-white/60">erro</span>
                </span>
              )}
            </div>
          </div>

          <div className="border-l border-white/10 pl-6">
            <div className="text-xs uppercase tracking-widest text-white/50">Campanhas ativas</div>
            <div className="text-3xl font-bold mt-1 flex items-center gap-2">
              {campaignsRunning}
              {campaignsRunning > 0 && (
                <span className="text-[10px] uppercase tracking-widest font-bold bg-lime-accent text-forest-950 px-2 py-0.5 rounded-full">
                  rodando
                </span>
              )}
            </div>
          </div>

          <div className="border-l border-white/10 pl-6">
            <div className="text-xs uppercase tracking-widest text-white/50">Mensagens enviadas</div>
            <div className="text-3xl font-bold mt-1">{totalSent.toLocaleString('pt-BR')}</div>
            <div className="text-[11px] text-white/50 mt-0.5">total de todas as campanhas</div>
          </div>
        </div>
      </div>

      {/* Stats strip */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <StatChip
          icon={<CheckCircle2 className="w-4 h-4" />}
          label="Prontos"
          value={ready}
          tone="emerald"
        />
        <StatChip
          icon={<Activity className="w-4 h-4" />}
          label="Provisionando"
          value={provisioning}
          tone="amber"
        />
        <StatChip
          icon={<AlertCircle className="w-4 h-4" />}
          label="Com erro"
          value={errors}
          tone="rose"
        />
      </div>

      {/* Section title */}
      <div className="flex items-center justify-between mb-3">
        <div>
          <div className="section-label">Frota</div>
          <div className="text-base font-semibold text-forest-950 mt-0.5">Seus dispositivos</div>
        </div>
      </div>

      {/* Loading skeleton */}
      {loading && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="card p-5 animate-pulse">
              <div className="flex items-start gap-3 mb-4">
                <div className="w-10 h-10 rounded-lg bg-gray-100" />
                <div className="flex-1">
                  <div className="h-4 bg-gray-100 rounded w-2/3 mb-2" />
                  <div className="h-3 bg-gray-100 rounded w-1/3" />
                </div>
              </div>
              <div className="h-3 bg-gray-100 rounded w-1/4 mb-4" />
              <div className="border-t border-gray-100 pt-3">
                <div className="h-8 bg-gray-100 rounded w-1/2" />
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Empty */}
      {!loading && devices.length === 0 && (
        <div className="card p-10 text-center">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-forest-50 text-forest-700 mb-4">
            <Smartphone className="w-7 h-7" />
          </div>
          <h3 className="text-base font-semibold text-forest-950 mb-1">
            Nenhum dispositivo ainda
          </h3>
          <p className="text-sm text-gray-500 mb-4">
            Crie seu primeiro emulador Android pra começar.
          </p>
          <button
            onClick={() => setModalOpen(true)}
            className="btn-accent inline-flex items-center gap-2"
          >
            <Plus className="w-4 h-4" />
            Criar dispositivo
          </button>
        </div>
      )}

      {/* Grid */}
      {!loading && devices.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {devices.map((device) => (
            <DeviceCard
              key={device.id}
              device={device}
              onRefresh={refresh}
              currentAPKHash={currentAPKHash}
            />
          ))}
        </div>
      )}

      <CreateDeviceModal open={modalOpen} onClose={() => setModalOpen(false)} onCreated={refresh} />
    </div>
  );
}

function StatChip({
  icon, label, value, tone,
}: {
  icon: React.ReactNode;
  label: string;
  value: number;
  tone: 'emerald' | 'amber' | 'rose';
}) {
  const tones: Record<string, string> = {
    emerald: 'bg-emerald-50 text-emerald-700',
    amber: 'bg-amber-50 text-amber-700',
    rose: 'bg-rose-50 text-rose-700',
  };
  return (
    <div className="card px-4 py-3 flex items-center gap-3">
      <div className={`w-9 h-9 rounded-lg grid place-items-center ${tones[tone]}`}>{icon}</div>
      <div className="flex-1">
        <div className="text-[11px] uppercase tracking-widest text-gray-500 font-bold">{label}</div>
        <div className="text-xl font-bold text-forest-950">{value}</div>
      </div>
    </div>
  );
}
