import React, { useEffect, useState } from 'react';
import { X, Star } from 'lucide-react';
import { createDevice } from '../api/devices';
import { listAPKs, APKListItem } from '../api/apk';

interface CreateDeviceModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}

export default function CreateDeviceModal({
  open,
  onClose,
  onCreated,
}: CreateDeviceModalProps) {
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [apks, setApks] = useState<APKListItem[]>([]);
  const [apkID, setApkID] = useState<string>(''); // '' = use server default

  useEffect(() => {
    if (!open) return;
    listAPKs()
      .then((r) => {
        setApks(r.data);
        // default selection: server-side default apk
        const def = r.data.find((a) => a.is_default);
        setApkID(def ? def.id : '');
      })
      .catch(() => setApks([]));
  }, [open]);

  if (!open) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError('Nome do dispositivo é obrigatório');
      return;
    }

    setLoading(true);
    setError('');

    try {
      await createDevice(name.trim(), apkID || undefined);
      setName('');
      onCreated();
      onClose();
    } catch (err: unknown) {
      if (
        err &&
        typeof err === 'object' &&
        'response' in err &&
        (err as { response?: { data?: { error?: string } } }).response?.data
          ?.error
      ) {
        setError(
          (err as { response: { data: { error: string } } }).response.data.error
        );
      } else {
        setError('Falha ao criar dispositivo. Tente novamente.');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="absolute inset-0 bg-forest-950/40 backdrop-blur-sm"
        onClick={onClose}
      />
      <div className="relative bg-white rounded-2xl shadow-soft-lg w-full max-w-md mx-4 p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-bold text-forest-950 tracking-tight">
            Criar novo dispositivo
          </h2>
          <button
            onClick={onClose}
            className="p-1 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit}>
          {error && (
            <div className="mb-4 p-3 text-sm text-red-700 bg-red-50 rounded-lg">
              {error}
            </div>
          )}

          <div className="mb-4">
            <label
              htmlFor="device-name"
              className="block text-sm font-medium text-gray-700 mb-1.5"
            >
              Nome do dispositivo
            </label>
            <input
              id="device-name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Meu dispositivo Android"
              className="input-field"
              autoFocus
              disabled={loading}
            />
          </div>

          <div className="mb-5">
            <label
              htmlFor="apk-version"
              className="block text-sm font-medium text-gray-700 mb-1.5"
            >
              Versão do WhatsApp (APK)
            </label>
            {apks.length === 0 ? (
              <div className="text-xs text-gray-500 italic px-3 py-2 rounded-lg border border-gray-200 bg-gray-50">
                Nenhuma versão cadastrada — vai usar o APK padrão da imagem.
                Adicione versões na página "Aplicativo (APK)".
              </div>
            ) : (
              <>
                <select
                  id="apk-version"
                  value={apkID}
                  onChange={(e) => setApkID(e.target.value)}
                  disabled={loading}
                  className="input-field"
                >
                  {apks.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.is_default ? '★ ' : ''}
                      v{a.version_name}
                      {a.app_label ? ` · ${a.app_label}` : ''}
                      {' · '}
                      {a.human_size}
                    </option>
                  ))}
                </select>
                <div className="text-xs text-gray-500 mt-1 flex items-center gap-1">
                  <Star className="w-3 h-3 fill-forest-700 text-forest-700" />
                  marca a versão padrão · você pode trocar pra qualquer outra
                </div>
              </>
            )}
          </div>

          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              disabled={loading}
              className="btn-secondary"
            >
              Cancelar
            </button>
            <button type="submit" disabled={loading} className="btn-accent">
              {loading ? (
                <span className="flex items-center gap-2">
                  <span className="animate-spin rounded-full h-4 w-4 border-b-2 border-white" />
                  Criando...
                </span>
              ) : (
                'Criar dispositivo'
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
