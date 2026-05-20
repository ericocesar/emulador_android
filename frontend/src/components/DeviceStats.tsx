import React, { useState, useEffect, useRef } from 'react';
import { Cpu, HardDrive, Clock } from 'lucide-react';
import { DeviceStats as DeviceStatsType } from '../types';
import { getDeviceStats } from '../api/devices';

interface DeviceStatsProps {
  deviceId: string;
}

export default function DeviceStats({ deviceId }: DeviceStatsProps) {
  const [stats, setStats] = useState<DeviceStatsType | null>(null);
  const [error, setError] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const res = await getDeviceStats(deviceId);
        setStats(res.data);
        setError(false);
      } catch {
        setError(true);
      }
    };

    fetchStats();
    intervalRef.current = setInterval(fetchStats, 10000);

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [deviceId]);

  if (error || !stats) {
    return (
      <div className="card p-4">
        <h3 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">
          Estatísticas
        </h3>
        <p className="text-sm text-gray-400">
          {error ? 'Falha ao carregar estatísticas' : 'Carregando...'}
        </p>
      </div>
    );
  }

  const memoryPercent =
    stats.memory_limit_mb > 0
      ? (stats.memory_usage_mb / stats.memory_limit_mb) * 100
      : 0;

  return (
    <div className="card p-4">
      <h3 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">
        Statistics
      </h3>
      <div className="space-y-4">
        {/* CPU */}
        <div>
          <div className="flex items-center justify-between mb-1.5">
            <div className="flex items-center gap-1.5 text-sm text-gray-600">
              <Cpu className="w-3.5 h-3.5" />
              CPU
            </div>
            <span className="text-sm font-medium text-gray-900">
              {stats.cpu_percent.toFixed(1)}%
            </span>
          </div>
          <div className="w-full h-2 bg-gray-100 rounded-full overflow-hidden">
            <div
              className="h-full rounded-full transition-all duration-500 bg-indigo-500"
              style={{ width: `${Math.min(stats.cpu_percent, 100)}%` }}
            />
          </div>
        </div>

        {/* Memory */}
        <div>
          <div className="flex items-center justify-between mb-1.5">
            <div className="flex items-center gap-1.5 text-sm text-gray-600">
              <HardDrive className="w-3.5 h-3.5" />
              RAM
            </div>
            <span className="text-sm font-medium text-gray-900">
              {stats.memory_usage_mb.toFixed(0)} / {stats.memory_limit_mb.toFixed(0)} MB
            </span>
          </div>
          <div className="w-full h-2 bg-gray-100 rounded-full overflow-hidden">
            <div
              className="h-full rounded-full transition-all duration-500 bg-emerald-500"
              style={{ width: `${Math.min(memoryPercent, 100)}%` }}
            />
          </div>
        </div>

        {/* Uptime */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1.5 text-sm text-gray-600">
            <Clock className="w-3.5 h-3.5" />
            Tempo ativo
          </div>
          <span className="text-sm font-medium text-gray-900">
            {stats.uptime}
          </span>
        </div>
      </div>
    </div>
  );
}
