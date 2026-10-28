// Minimal typings for the parts of Telegram.WebApp we use.
interface TgWebApp {
  initData: string;
  initDataUnsafe: { start_param?: string };
  colorScheme?: 'light' | 'dark';
  ready(): void;
  expand(): void;
  openTelegramLink(url: string): void;
  showConfirm(message: string, cb: (ok: boolean) => void): void;
  setHeaderColor?(color: string): void;
  setBackgroundColor?(color: string): void;
  setBottomBarColor?(color: string): void;
  onEvent?(event: 'themeChanged', cb: () => void): void;
  HapticFeedback?: {
    impactOccurred(style: 'light' | 'medium'): void;
    notificationOccurred(type: 'success' | 'error'): void;
    selectionChanged(): void;
  };
}

export const tg: TgWebApp | undefined = (window as unknown as { Telegram?: { WebApp?: TgWebApp } }).Telegram?.WebApp;

// Telegram's own scheme wins; outside Telegram fall back to the OS preference.
function applyTheme() {
  const dark = tg?.colorScheme ? tg.colorScheme === 'dark' : matchMedia('(prefers-color-scheme: dark)').matches;
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  const bg = dark ? '#0b0b10' : '#f3f2f8';
  tg?.setHeaderColor?.(bg);
  tg?.setBackgroundColor?.(bg);
  tg?.setBottomBarColor?.(bg);
}

export function initTelegram() {
  applyTheme();
  tg?.onEvent?.('themeChanged', applyTheme);
  tg?.ready();
  tg?.expand();
}

/** Invite code from the deep link (startapp) or the ?code= fallback used by /start CODE. */
export function inviteCodeFromLaunch(): string {
  const fromTg = tg?.initDataUnsafe?.start_param;
  return (fromTg || new URLSearchParams(location.search).get('code') || '').toUpperCase();
}

export function confirmDialog(message: string): Promise<boolean> {
  return new Promise((resolve) => {
    if (tg?.showConfirm) tg.showConfirm(message, resolve);
    else resolve(window.confirm(message));
  });
}

export const haptic = {
  tap: () => tg?.HapticFeedback?.impactOccurred('light'),
  select: () => tg?.HapticFeedback?.selectionChanged(),
  ok: () => tg?.HapticFeedback?.notificationOccurred('success'),
  err: () => tg?.HapticFeedback?.notificationOccurred('error'),
};
