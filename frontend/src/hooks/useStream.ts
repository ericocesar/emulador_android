import { useRef, useState, useEffect, useCallback } from 'react';
import { InputMessage } from '../types';

export function useStream(deviceId: string, token: string | null) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);
  const [frameCount, setFrameCount] = useState(0);

  const sendInput = useCallback((msg: InputMessage) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg));
    }
  }, []);

  useEffect(() => {
    if (!token || !deviceId) return;

    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let alive = true;

    function connectWs() {
      if (!alive) return;

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/api/devices/${deviceId}/stream?token=${token}`;

      const ws = new WebSocket(wsUrl);
      ws.binaryType = 'arraybuffer';
      wsRef.current = ws;

      ws.onopen = () => {
        if (alive) setConnected(true);
      };

      ws.onmessage = (event) => {
        if (!alive) return;
        if (!(event.data instanceof ArrayBuffer)) return;

        const blob = new Blob([event.data], { type: 'image/jpeg' });
        const url = URL.createObjectURL(blob);
        const img = new Image();
        img.onload = () => {
          const canvas = canvasRef.current;
          if (canvas) {
            canvas.width = img.width;
            canvas.height = img.height;
            const ctx = canvas.getContext('2d');
            if (ctx) {
              ctx.drawImage(img, 0, 0);
            }
          }
          URL.revokeObjectURL(url);
          if (alive) setFrameCount((c) => c + 1);
        };
        img.onerror = () => {
          URL.revokeObjectURL(url);
        };
        img.src = url;
      };

      ws.onclose = () => {
        if (alive) {
          setConnected(false);
          reconnectTimer = setTimeout(connectWs, 3000);
        }
      };

      ws.onerror = () => {
        ws.close();
      };
    }

    connectWs();

    return () => {
      alive = false;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      if (wsRef.current) {
        wsRef.current.onclose = null; // prevent reconnect on cleanup
        wsRef.current.close();
        wsRef.current = null;
      }
      setConnected(false);
    };
  }, [deviceId, token]);

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.onclose = null;
      wsRef.current.close();
      wsRef.current = null;
    }
    setConnected(false);
  }, []);

  return { canvasRef, connected, sendInput, disconnect, frameCount };
}
