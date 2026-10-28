import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Me, Stats } from '../types';
import { addDays, plural, todayIn } from '../util';
import { haptic } from '../telegram';
import { Avatar, Empty, Ring, Skeleton, humanError, useToast } from '../ui';

const RANGES = [{ label: 'Неделя', days: 6 }, { label: 'Месяц', days: 29 }, { label: '3 месяца', days: 89 }];

export default function StatsScreen({ me }: { me: Me }) {
  const group = me.group!;
  const isHeadman = me.member!.role === 'headman';
  const toast = useToast();
  const [days, setDays] = useState(29);
  const [stats, setStats] = useState<Stats | null>(null);

  useEffect(() => {
    const to = todayIn(group.timezone);
    setStats(null);
    api.stats(addDays(to, -days), to).then(setStats, (e) => toast(humanError(e)));
  }, [days, group.timezone, toast]);

  const rows = stats?.rows ?? [];
  const mine = rows.find((r) => r.user_id === me.member!.user_id);
  const sum = rows.reduce((a, r) => ({ p: a.p + r.present, t: a.t + r.total }), { p: 0, t: 0 });
  const heroPct = isHeadman ? (sum.t ? sum.p / sum.t : 0) : mine?.total ? mine.present / mine.total : 0;
  const sorted = [...rows].sort((a, b) => (a.total ? a.present / a.total : 0) - (b.total ? b.present / b.total : 0));

  return (
    <div className="page">
      <header className="top">
        <div>
          <div className="eyebrow">{isHeadman ? group.name : 'Личная статистика'}</div>
          <h1 className="display">Итоги</h1>
        </div>
      </header>

      <div className="segmented">
        {RANGES.map((r) => (
          <button key={r.days} className={days === r.days ? 'active' : ''} onClick={() => { haptic.select(); setDays(r.days); }}>{r.label}</button>
        ))}
      </div>

      {!stats && <Skeleton h={180} />}
      {stats && (
        <div className="hero-card">
          <Ring value={heroPct} size={112} stroke={10}>
            <b>{Math.round(heroPct * 100)}%</b>
          </Ring>
          <div>
            <div className="hero-k">{isHeadman ? 'Средняя посещаемость' : 'Ваша посещаемость'}</div>
            <div className="hero-v">{stats.lessons} {plural(stats.lessons, 'пара', 'пары', 'пар')}</div>
            {!isHeadman && mine && (
              <div className="hero-chips">
                <span>✓ {mine.present}</span><span>✕ {mine.absent}</span><span>? {mine.unmarked}</span>
              </div>
            )}
            {isHeadman && <div className="hero-chips"><span>{rows.length} {plural(rows.length, 'студент', 'студента', 'студентов')}</span></div>}
          </div>
        </div>
      )}

      {stats && stats.lessons === 0 && <Empty title="Нет прошедших пар" text="Статистика появится после первых занятий" />}

      {isHeadman && stats && stats.lessons > 0 && (
        <section>
          <h3 className="section-title">Студенты · сначала отстающие</h3>
          <div className="card list">
            {sorted.map((r) => {
              const pct = r.total ? r.present / r.total : 0;
              return (
                <div key={r.user_id} className="list-row">
                  <Avatar name={r.full_name} id={r.user_id} />
                  <div className="grow">
                    <div className="row-title">{r.full_name}</div>
                    <div className="meta">
                      <span className="c-ok">✓ {r.present}</span><span className="c-bad">✕ {r.absent}</span><span>? {r.unmarked}</span>
                    </div>
                  </div>
                  <Ring value={pct} size={44} stroke={5}><small>{Math.round(pct * 100)}</small></Ring>
                </div>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}
