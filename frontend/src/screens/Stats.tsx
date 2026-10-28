import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Group, Stats } from '../types';
import { addDays, todayIn } from '../util';

const RANGES = [{ label: '7 дней', days: 6 }, { label: '30 дней', days: 29 }, { label: '90 дней', days: 89 }];

export default function StatsScreen({ group, isHeadman }: { group: Group; isHeadman: boolean }) {
  const [days, setDays] = useState(29);
  const [stats, setStats] = useState<Stats | null>(null);
  const [err, setErr] = useState('');

  useEffect(() => {
    const to = todayIn(group.timezone);
    setStats(null);
    api.stats(addDays(to, -days), to).then(setStats, (e: Error) => setErr(e.message));
  }, [days, group.timezone]);

  return (
    <div className="page">
      <h1>{isHeadman ? 'Посещаемость группы' : 'Моя посещаемость'}</h1>
      <div className="segmented">
        {RANGES.map((r) => <button key={r.days} className={days === r.days ? 'active' : ''} onClick={() => setDays(r.days)}>{r.label}</button>)}
      </div>
      {err && <p className="err">{err}</p>}
      {stats && <p className="muted small">Пар за период: {stats.lessons}</p>}
      {stats?.rows.map((r) => {
        const pct = r.total ? Math.round((r.present / r.total) * 100) : 0;
        return (
          <div key={r.user_id} className="card">
            <div className="row"><b className="grow">{r.full_name}</b><b>{pct}%</b></div>
            <div className="bar"><i style={{ width: pct + '%' }} /></div>
            <div className="muted small">был {r.present} · не был {r.absent} · не отмечено {r.unmarked}</div>
          </div>
        );
      })}
    </div>
  );
}
