// Minimal typings for the parts of Telegram.WebApp we use.
interface TgWebApp {
  initData: string;
  initDataUnsafe: { start_param?: string };
  ready(): void;
  expand(): void;
  openTelegramLink(url: string): void;
  showConfirm(message: string, cb: (ok: boolean) => void): void;
  HapticFeedback?: {
    impactOccurred(style: 'light' | 'medium'): void;
    notificationOccurred(type: 'success' | 'error'): void;
  };
}

export const tg: TgWebApp | undefined = (window as unknown as { Telegram?: { WebApp?: TgWebApp } }).Telegram?.WebApp;

export function initTelegram() {
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
  ok: () => tg?.HapticFeedback?.notificationOccurred('success'),
  err: () => tg?.HapticFeedback?.notificationOccurred('error'),
};
