import { tg } from './telegram';
import type { AttendanceRow, GroupDetails, Lesson, Me, Slot, Stats } from './types';

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

// Optional dev identity for running outside Telegram against a DEV_AUTH=true backend: ?dev=1:Name
const dev = new URLSearchParams(location.search).get('dev');

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (tg?.initData) headers.Authorization = 'tma ' + tg.initData;
  else if (dev) headers['X-Dev-User'] = encodeURIComponent(dev); // headers must be ASCII
  const res = await fetch('/api' + path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      msg = (await res.json()).error ?? msg;
    } catch {
      /* non-JSON error */
    }
    throw new ApiError(res.status, msg);
  }
  return res.status === 204 ? (undefined as T) : res.json();
}

export const hasCredentials = () => Boolean(tg?.initData || dev);

export const api = {
  me: () => request<Me>('GET', '/me'),
  preview: (code: string) => request<{ name: string }>('GET', `/groups/preview?code=${encodeURIComponent(code)}`),
  createGroup: (b: { name: string; full_name: string; timezone: string; current_week: string }) => request<void>('POST', '/groups', b),
  join: (b: { code: string; full_name: string }) => request<void>('POST', '/groups/join', b),
  group: () => request<GroupDetails>('GET', '/group'),
  updateGroup: (b: { name: string; timezone: string; current_week: string }) => request<void>('PATCH', '/group', b),
  regenerateInvite: () => request<{ invite_code: string; invite_link: string }>('POST', '/group/invite/regenerate'),
  leave: () => request<void>('POST', '/group/leave'),
  rename: (id: number, full_name: string) => request<void>('PATCH', `/members/${id}`, { full_name }),
  removeMember: (id: number) => request<void>('DELETE', `/members/${id}`),
  schedule: () => request<Slot[]>('GET', '/schedule'),
  saveSlot: (slot: Omit<Slot, 'id'>, id?: number) =>
    id ? request<Slot>('PUT', `/schedule/${id}`, slot) : request<Slot>('POST', '/schedule', slot),
  deleteSlot: (id: number) => request<void>('DELETE', `/schedule/${id}`),
  lessons: (date: string) => request<Lesson[]>('GET', `/lessons?date=${date}`),
  lessonAttendance: (slotId: number, date: string) =>
    request<AttendanceRow[]>('GET', `/lessons/attendance?slot_id=${slotId}&date=${date}`),
  cancelLesson: (slot_id: number, date: string, cancelled: boolean) => request<void>('PUT', '/lessons/cancel', { slot_id, date, cancelled }),
  mark: (b: { slot_id: number; date: string; status: 'present' | 'absent' | 'none'; user_id?: number }) => request<void>('PUT', '/attendance', b),
  stats: (from: string, to: string) => request<Stats>('GET', `/stats?from=${from}&to=${to}`),
};
