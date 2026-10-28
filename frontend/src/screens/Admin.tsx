import { useEffect, useMemo, useState } from 'react';
import { api } from '../api';
import type { AdminGroup, AdminUser } from '../types';
import { Icon } from '../icons';
import { Avatar, Empty, Skeleton, humanError, useToast } from '../ui';
import { plural } from '../util';
import { haptic } from '../telegram';

const fmt = (iso: string) => new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' });

export default function AdminScreen({ onBack }: { onBack?: () => void }) {
  const toast = useToast();
  const [data, setData] = useState<{ users: AdminUser[]; groups: AdminGroup[] } | null>(null);
  const [view, setView] = useState<'users' | 'groups'>('users');
  const [q, setQ] = useState('');

  useEffect(() => {
    api.adminOverview().then(setData, (e) => toast(humanError(e)));
  }, [toast]);

  const query = q.trim().toLowerCase();
  const users = useMemo(() => (data?.users ?? []).filter((u) =>
    !query || [u.full_name, u.first_name, u.last_name, u.username, u.group_name, String(u.id)].some((s) => s.toLowerCase().includes(query)),
  ), [data, query]);
  const groups = useMemo(() => (data?.groups ?? []).filter((g) =>
    !query || [g.name, g.headman].some((s) => s.toLowerCase().includes(query)),
  ), [data, query]);

  const total = data?.users.length ?? 0;
  const inGroups = data?.users.filter((u) => u.group_id).length ?? 0;

  return (
    <div className="page">
      <header className="top">
        {onBack && <button className="icon-btn" onClick={onBack} aria-label="Назад"><Icon name="left" /></button>}
        <div className="grow">
          <div className="eyebrow">Только для администратора</div>
          <h1 className="display">Админка</h1>
        </div>
      </header>

      {!data ? <Skeleton h={96} /> : (
        <div className="stat-tiles">
          <div className="stat-tile accent"><b>{total}</b><span>{plural(total, 'пользователь', 'пользователя', 'пользователей')}</span></div>
          <div className="stat-tile"><b>{data.groups.length}</b><span>{plural(data.groups.length, 'группа', 'группы', 'групп')}</span></div>
          <div className="stat-tile"><b>{inGroups}</b><span>в группах</span></div>
          <div className="stat-tile"><b>{total - inGroups}</b><span>без группы</span></div>
        </div>
      )}

      <div className="search">
        <Icon name="search" size={18} />
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Имя, @username, группа или ID" />
      </div>

      <div className="segmented">
        <button className={view === 'users' ? 'active' : ''} onClick={() => { haptic.select(); setView('users'); }}>Пользователи</button>
        <button className={view === 'groups' ? 'active' : ''} onClick={() => { haptic.select(); setView('groups'); }}>Группы</button>
      </div>

      {data && view === 'users' && (
        <section>
          <h3 className="section-title">{users.length} {plural(users.length, 'пользователь', 'пользователя', 'пользователей')}</h3>
          {users.length === 0 ? <Empty title="Никого не нашли" /> : (
            <div className="card list">
              {users.map((u) => {
                const name = u.full_name || [u.first_name, u.last_name].filter(Boolean).join(' ') || `ID ${u.id}`;
                return (
                  <div key={u.id} className="list-row">
                    <Avatar name={name} id={u.id} />
                    <div className="grow">
                      <div className="row-title">{name}{u.role === 'headman' && <Icon name="crown" size={14} />}</div>
                      <div className="meta wrap">
                        {u.username && <span>@{u.username}</span>}
                        <span>{u.group_name || 'без группы'}</span>
                        <span>ID {u.id}</span>
                      </div>
                    </div>
                    <span className="hint">{fmt(u.created_at)}</span>
                  </div>
                );
              })}
            </div>
          )}
        </section>
      )}

      {data && view === 'groups' && (
        <section>
          <h3 className="section-title">{groups.length} {plural(groups.length, 'группа', 'группы', 'групп')}</h3>
          {groups.length === 0 ? <Empty title="Групп нет" /> : (
            <div className="card list">
              {groups.map((g) => (
                <div key={g.id} className="list-row">
                  <div className="grow">
                    <div className="row-title">{g.name}</div>
                    <div className="meta wrap">
                      <span><Icon name="crown" size={13} /> {g.headman || '—'}</span>
                      <span>{g.members} {plural(g.members, 'участник', 'участника', 'участников')}</span>
                      <span>{g.slots} {plural(g.slots, 'пара', 'пары', 'пар')} в расписании</span>
                      <span>{g.marks} {plural(g.marks, 'отметка', 'отметки', 'отметок')} за 30 дней</span>
                    </div>
                  </div>
                  <span className="hint">{fmt(g.created_at)}</span>
                </div>
              ))}
            </div>
          )}
        </section>
      )}
    </div>
  );
}
