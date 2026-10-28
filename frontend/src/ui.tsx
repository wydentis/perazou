import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react';
import { Icon } from './icons';

// Flat pastel swatches; the initials stay dark on every one of them.
const SWATCHES = ['#d7ff3d', '#b9a8ff', '#ffb4a2', '#9fe8d0', '#ffd66b', '#a8d8ff', '#ff9ed2', '#c9c9c9'];

export function Avatar({ name, id, size = 40 }: { name: string; id: number; size?: number }) {
  const initials = name.split(' ').filter(Boolean).slice(0, 2).map((w) => w[0]).join('').toUpperCase();

  return (
    <div className="avatar" style={{ width: size, height: size, fontSize: size * 0.36,
      background: SWATCHES[Math.abs(id) % SWATCHES.length] }}>
      {initials || '?'}
    </div>
  );
}

export function Ring({ value, size = 56, stroke = 6, children }: { value: number; size?: number; stroke?: number; children?: ReactNode }) {
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  return (
    <div className="ring" style={{ width: size, height: size }}>
      <svg width={size} height={size}>
        <circle cx={size / 2} cy={size / 2} r={r} strokeWidth={stroke} className="ring-track" />
        <circle cx={size / 2} cy={size / 2} r={r} strokeWidth={stroke} className="ring-value"
          strokeDasharray={c} strokeDashoffset={c * (1 - Math.min(1, Math.max(0, value)))} />
      </svg>
      <div className="ring-label">{children}</div>
    </div>
  );
}

export function Empty({ title, text }: { title: string; text?: string }) {
  return (
    <div className="empty">
      <div className="empty-orb"><Icon name="sparkle" size={28} /></div>
      <h3>{title}</h3>
      {text && <p>{text}</p>}
    </div>
  );
}

export function Skeleton({ h = 96 }: { h?: number }) {
  return <div className="skeleton" style={{ height: h }} />;
}

// ---- toasts ----
type Toast = { id: number; text: string; kind: 'ok' | 'err' };
const ToastCtx = createContext<(text: string, kind?: Toast['kind']) => void>(() => {});
export const useToast = () => useContext(ToastCtx);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Toast[]>([]);
  const seq = useRef(0);
  const push = useCallback((text: string, kind: Toast['kind'] = 'err') => {
    const id = ++seq.current;
    setItems((xs) => [...xs, { id, text, kind }]);
    setTimeout(() => setItems((xs) => xs.filter((x) => x.id !== id)), 3200);
  }, []);
  return (
    <ToastCtx.Provider value={push}>
      {children}
      <div className="toasts">
        {items.map((t) => (
          <div key={t.id} className={'toast ' + t.kind}>
            <Icon name={t.kind === 'ok' ? 'check' : 'x'} size={16} />
            {t.text}
          </div>
        ))}
      </div>
    </ToastCtx.Provider>
  );
}

/** Translates backend error strings into Russian for display. */
export function humanError(e: unknown): string {
  const m = (e as Error)?.message ?? '';
  const map: Record<string, string> = {
    'marking is closed for this lesson': 'Отметиться можно только в первые 15 минут пары',
    'lesson has not started yet': 'Пара ещё не началась',
    'lesson is cancelled': 'Пара отменена',
    'invite not found': 'Приглашение не найдено',
    'already in a group': 'Вы уже состоите в группе',
    'headman only': 'Доступно только старосте',
    'headman cannot leave the group': 'Староста не может выйти из группы',
    'invalid slot': 'Проверьте поля пары',
    'init data expired': 'Сессия устарела — перезапустите приложение',
  };
  return map[m] ?? (m || 'Что-то пошло не так');
}
