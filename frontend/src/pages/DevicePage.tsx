import React, { useState, useEffect } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import {
  ArrowLeft,
  ChevronRight,
  Loader2,
  AlertCircle,
} from 'lucide-react';
import { Device, DeviceStatus } from '../types';
import { getDevice } from '../api/devices';
import { useAuth } from '../hooks/useAuth';
import DeviceControls from '../components/DeviceControls';
import DeviceStats from '../components/DeviceStats';
import { useStream } from '../hooks/useStream';
import { InputMessage } from '../types';

const statusLabels: Record<DeviceStatus, string> = {
  creating: 'Criando dispositivo...',
  booting: 'Inicializando Android...',
  installing: 'Instalando apps...',
  ready: 'Pronto',
  stopped: 'Parado',
  error: 'Erro',
};

const statusColors: Record<DeviceStatus, string> = {
  creating: 'bg-yellow-100 text-yellow-800',
  booting: 'bg-yellow-100 text-yellow-800',
  installing: 'bg-yellow-100 text-yellow-800',
  ready: 'bg-green-100 text-green-800',
  stopped: 'bg-gray-100 text-gray-800',
  error: 'bg-red-100 text-red-800',
};

export default function DevicePage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { token } = useAuth();
  const [device, setDevice] = useState<Device | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const fetchDevice = React.useCallback(async () => {
    if (!id) return;
    try {
      const res = await getDevice(id);
      setDevice(res.data);
      setError('');
    } catch {
      setError('Falha ao carregar dispositivo');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    if (!id) return;
    fetchDevice();
    const interval = setInterval(fetchDevice, 5000);
    return () => clearInterval(interval);
  }, [id, fetchDevice]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Loader2 className="w-8 h-8 animate-spin text-indigo-600" />
      </div>
    );
  }

  if (error || !device) {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <AlertCircle className="w-12 h-12 text-red-400 mb-3" />
        <p className="text-gray-600 mb-4">{error || 'Dispositivo não encontrado'}</p>
        <button onClick={() => navigate('/dashboard')} className="btn-primary">
          Voltar
        </button>
      </div>
    );
  }

  return (
    <div className="p-6 lg:p-8 max-w-7xl mx-auto">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm text-gray-500 mb-6">
        <Link
          to="/dashboard"
          className="hover:text-indigo-600 transition-colors"
        >
          Dispositivos
        </Link>
        <ChevronRight className="w-4 h-4" />
        <span className="text-gray-900 font-medium">{device.name}</span>
      </div>

      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4">
          <button
            onClick={() => navigate('/dashboard')}
            className="p-2 rounded-lg hover:bg-gray-200 text-gray-500 transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
          </button>
          <div>
            <h1 className="text-xl font-bold text-gray-900">{device.name}</h1>
            <p className="text-sm text-gray-500">
              Android {device.android_version || 'N/A'}
            </p>
          </div>
        </div>
        <span
          className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${
            statusColors[device.status] || statusColors.error
          }`}
        >
          {statusLabels[device.status] || device.status}
        </span>
      </div>

      {/* Content */}
      {device.status === 'ready' ? (
        <DeviceReadyView device={device} token={token!} onChanged={fetchDevice} />
      ) : (
        <DeviceNotReadyView device={device} />
      )}
    </div>
  );
}

function DeviceReadyView({
  device,
  token,
  onChanged,
}: {
  device: Device;
  token: string;
  onChanged: () => void;
}) {
  const { canvasRef, connected, sendInput } = useStream(device.id, token);

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
      {/* Screen - takes 2 columns */}
      <div className="lg:col-span-2 flex justify-center">
        <div className="w-full max-w-sm">
          <DeviceScreenInline
            canvasRef={canvasRef}
            connected={connected}
            sendInput={sendInput}
          />
        </div>
      </div>

      {/* Controls + Stats */}
      <div className="space-y-4">
        <DeviceControls device={device} sendInput={sendInput} connected={connected} onChanged={onChanged} />
        <DeviceStats deviceId={device.id} />
      </div>
    </div>
  );
}

function DeviceScreenInline({
  canvasRef,
  connected,
  sendInput,
}: {
  canvasRef: React.RefObject<HTMLCanvasElement>;
  connected: boolean;
  sendInput: (msg: InputMessage) => void;
}) {
  const pointerStart = React.useRef<{
    x: number;
    y: number;
    time: number;
  } | null>(null);
  const lastPos = React.useRef<{ x: number; y: number } | null>(null);

  const CANVAS_WIDTH = 360;
  const CANVAS_HEIGHT = 640;
  const SWIPE_THRESHOLD = 10;

  const getCanvasCoords = (
    e: React.MouseEvent | React.Touch
  ): { x: number; y: number } => {
    const canvas = canvasRef.current;
    if (!canvas) return { x: 0, y: 0 };
    const rect = canvas.getBoundingClientRect();
    const scaleX =
      canvas.width > 0 ? canvas.width / rect.width : CANVAS_WIDTH / rect.width;
    const scaleY =
      canvas.height > 0
        ? canvas.height / rect.height
        : CANVAS_HEIGHT / rect.height;
    return {
      x: Math.round(e.clientX - rect.left) * scaleX,
      y: Math.round(e.clientY - rect.top) * scaleY,
    };
  };

  const handlePointerDown = (coords: { x: number; y: number }) => {
    pointerStart.current = { ...coords, time: Date.now() };
    lastPos.current = coords;
  };

  const handlePointerMove = (coords: { x: number; y: number }) => {
    if (pointerStart.current) {
      lastPos.current = coords;
    }
  };

  const handlePointerUp = (coords: { x: number; y: number }) => {
    if (!pointerStart.current) return;
    const start = pointerStart.current;
    const dx = coords.x - start.x;
    const dy = coords.y - start.y;
    const distance = Math.sqrt(dx * dx + dy * dy);
    const duration = Date.now() - start.time;

    if (distance < SWIPE_THRESHOLD) {
      sendInput({
        type: 'tap',
        x: Math.round(coords.x),
        y: Math.round(coords.y),
      });
    } else {
      sendInput({
        type: 'swipe',
        start_x: Math.round(start.x),
        start_y: Math.round(start.y),
        end_x: Math.round(coords.x),
        end_y: Math.round(coords.y),
        duration: Math.max(duration, 100),
      });
    }
    pointerStart.current = null;
    lastPos.current = null;
  };

  return (
    <div
      className="relative bg-black rounded-2xl overflow-hidden shadow-2xl"
      style={{ aspectRatio: '9/16' }}
    >
      <canvas
        ref={canvasRef}
        width={CANVAS_WIDTH}
        height={CANVAS_HEIGHT}
        className="w-full h-full object-contain"
        onMouseDown={(e) => {
          e.preventDefault();
          handlePointerDown(getCanvasCoords(e));
        }}
        onMouseMove={(e) => handlePointerMove(getCanvasCoords(e))}
        onMouseUp={(e) => handlePointerUp(getCanvasCoords(e))}
        onMouseLeave={(e) => handlePointerUp(getCanvasCoords(e))}
        onTouchStart={(e) => {
          e.preventDefault();
          if (e.touches.length === 1) handlePointerDown(getCanvasCoords(e.touches[0]));
        }}
        onTouchMove={(e) => {
          e.preventDefault();
          if (e.touches.length === 1) handlePointerMove(getCanvasCoords(e.touches[0]));
        }}
        onTouchEnd={(e) => {
          e.preventDefault();
          handlePointerUp(lastPos.current || { x: 0, y: 0 });
        }}
      />
      {!connected && (
        <div className="absolute inset-0 bg-gray-900/80 flex flex-col items-center justify-center text-white">
          <Loader2 className="w-8 h-8 animate-spin mb-3 text-indigo-400" />
          <p className="text-sm font-medium">Conectando ao dispositivo...</p>
        </div>
      )}
    </div>
  );
}

function DeviceNotReadyView({ device }: { device: Device }) {
  const isActive = ['creating', 'booting', 'installing'].includes(
    device.status
  );

  return (
    <div className="card p-12 text-center">
      {isActive ? (
        <>
          <Loader2 className="w-12 h-12 animate-spin text-indigo-400 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-gray-900 mb-1">
            {statusLabels[device.status]}
          </h3>
          <p className="text-gray-500">
            Isso pode levar alguns minutos. A página atualiza automaticamente.
          </p>
        </>
      ) : device.status === 'error' ? (
        <>
          <AlertCircle className="w-12 h-12 text-red-400 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-gray-900 mb-1">
            Erro no dispositivo
          </h3>
          <p className="text-gray-500">
            {device.error_message || 'Ocorreu um erro desconhecido.'}
          </p>
        </>
      ) : (
        <>
          <div className="w-12 h-12 rounded-full bg-gray-100 mx-auto mb-4 flex items-center justify-center">
            <AlertCircle className="w-6 h-6 text-gray-400" />
          </div>
          <h3 className="text-lg font-semibold text-gray-900 mb-1">
            Dispositivo parado
          </h3>
          <p className="text-gray-500">
            Inicie o dispositivo na página de dispositivos para usá-lo.
          </p>
        </>
      )}
    </div>
  );
}

