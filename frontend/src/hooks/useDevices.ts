import { useState, useEffect, useCallback, useRef } from 'react';
import { Device } from '../types';
import { listDevices } from '../api/devices';

const MAX_CONSECUTIVE_ERRORS = 3;
const POLL_INTERVAL = 5000;

export function useDevices() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const errorsRef = useRef(0);

  const refresh = useCallback(async () => {
    try {
      const res = await listDevices();
      setDevices(res.data ?? []);
      setError(null);
      errorsRef.current = 0;
    } catch (err: unknown) {
      errorsRef.current++;
      if (errorsRef.current >= MAX_CONSECUTIVE_ERRORS) {
        const message =
          err instanceof Error ? err.message : 'Failed to load devices';
        setError(message);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    intervalRef.current = setInterval(refresh, POLL_INTERVAL);
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [refresh]);

  return { devices, loading, error, refresh };
}
