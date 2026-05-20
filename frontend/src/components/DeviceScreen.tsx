import React, { useRef, useCallback } from 'react';
import { useStream } from '../hooks/useStream';
import { InputMessage } from '../types';
import { Loader2 } from 'lucide-react';

interface DeviceScreenProps {
  deviceId: string;
  token: string;
}

const CANVAS_WIDTH = 360;
const CANVAS_HEIGHT = 640;
const SWIPE_THRESHOLD = 10;

export default function DeviceScreen({ deviceId, token }: DeviceScreenProps) {
  const { canvasRef, connected, sendInput } = useStream(deviceId, token);
  const pointerStart = useRef<{
    x: number;
    y: number;
    time: number;
  } | null>(null);
  const lastPos = useRef<{ x: number; y: number } | null>(null);

  const getCanvasCoords = useCallback(
    (e: React.MouseEvent | React.Touch): { x: number; y: number } => {
      const canvas = canvasRef.current;
      if (!canvas) return { x: 0, y: 0 };
      const rect = canvas.getBoundingClientRect();
      const scaleX = canvas.width > 0 ? canvas.width / rect.width : CANVAS_WIDTH / rect.width;
      const scaleY = canvas.height > 0 ? canvas.height / rect.height : CANVAS_HEIGHT / rect.height;
      return {
        x: Math.round(e.clientX - rect.left) * scaleX,
        y: Math.round(e.clientY - rect.top) * scaleY,
      };
    },
    [canvasRef]
  );

  const handlePointerDown = useCallback(
    (coords: { x: number; y: number }) => {
      pointerStart.current = { ...coords, time: Date.now() };
      lastPos.current = coords;
    },
    []
  );

  const handlePointerMove = useCallback(
    (coords: { x: number; y: number }) => {
      if (pointerStart.current) {
        lastPos.current = coords;
      }
    },
    []
  );

  const handlePointerUp = useCallback(
    (coords: { x: number; y: number }) => {
      if (!pointerStart.current) return;

      const start = pointerStart.current;
      const dx = coords.x - start.x;
      const dy = coords.y - start.y;
      const distance = Math.sqrt(dx * dx + dy * dy);
      const duration = Date.now() - start.time;

      let msg: InputMessage;
      if (distance < SWIPE_THRESHOLD) {
        msg = { type: 'tap', x: Math.round(coords.x), y: Math.round(coords.y) };
      } else {
        msg = {
          type: 'swipe',
          start_x: Math.round(start.x),
          start_y: Math.round(start.y),
          end_x: Math.round(coords.x),
          end_y: Math.round(coords.y),
          duration: Math.max(duration, 100),
        };
      }

      sendInput(msg);
      pointerStart.current = null;
      lastPos.current = null;
    },
    [sendInput]
  );

  const onMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      handlePointerDown(getCanvasCoords(e));
    },
    [getCanvasCoords, handlePointerDown]
  );

  const onMouseMove = useCallback(
    (e: React.MouseEvent) => {
      handlePointerMove(getCanvasCoords(e));
    },
    [getCanvasCoords, handlePointerMove]
  );

  const onMouseUp = useCallback(
    (e: React.MouseEvent) => {
      handlePointerUp(getCanvasCoords(e));
    },
    [getCanvasCoords, handlePointerUp]
  );

  const onTouchStart = useCallback(
    (e: React.TouchEvent) => {
      e.preventDefault();
      if (e.touches.length === 1) {
        handlePointerDown(getCanvasCoords(e.touches[0]));
      }
    },
    [getCanvasCoords, handlePointerDown]
  );

  const onTouchMove = useCallback(
    (e: React.TouchEvent) => {
      e.preventDefault();
      if (e.touches.length === 1) {
        handlePointerMove(getCanvasCoords(e.touches[0]));
      }
    },
    [getCanvasCoords, handlePointerMove]
  );

  const onTouchEnd = useCallback(
    (e: React.TouchEvent) => {
      e.preventDefault();
      const coords = lastPos.current || { x: 0, y: 0 };
      handlePointerUp(coords);
    },
    [handlePointerUp]
  );

  return (
    <div className="relative bg-black rounded-2xl overflow-hidden shadow-2xl"
      style={{ aspectRatio: '9/16', maxHeight: '80vh' }}
    >
      <canvas
        ref={canvasRef}
        width={CANVAS_WIDTH}
        height={CANVAS_HEIGHT}
        className="w-full h-full object-contain"
        onMouseDown={onMouseDown}
        onMouseMove={onMouseMove}
        onMouseUp={onMouseUp}
        onMouseLeave={onMouseUp}
        onTouchStart={onTouchStart}
        onTouchMove={onTouchMove}
        onTouchEnd={onTouchEnd}
      />

      {!connected && (
        <div className="absolute inset-0 bg-gray-900/80 flex flex-col items-center justify-center text-white">
          <Loader2 className="w-8 h-8 animate-spin mb-3 text-indigo-400" />
          <p className="text-sm font-medium">Connecting to device...</p>
          <p className="text-xs text-gray-400 mt-1">
            Establishing WebSocket stream
          </p>
        </div>
      )}
    </div>
  );
}
