import { useCallback, useEffect, useState } from 'react';
import { api, hasCredentials } from './api';
import type { Me } from './types';
import { inviteCodeFromLaunch } from './telegram';
import Onboarding from './screens/Onboarding';
import Today from './screens/Today';
import Schedule from './screens/Schedule';
import StatsScreen from './screens/Stats';
import GroupScreen from './screens/GroupScreen';

type Tab = 'today' | 'schedule' | 'stats' | 'group';
const TABS: { id: Tab; label: string; icon: string }[] = [
  { id: 'today', label: 'Пары', icon: '📅' },
  { id: 'schedule', label: 'Расписание', icon: '🗓' },
  { id: 'stats', label: 'Итоги', icon: '📊' },
  { id: 'group', label: 'Группа', icon: '👥' },
];

export default function App() {
  const [me, setMe] = useState<Me | null>(null);
  const [error, setError] = useState('');
  const [tab, setTab] = useState<Tab>('today');

  const reload = useCallback(() => {
    api.me().then(setMe, (e: Error) => setError(e.message));
  }, []);
  useEffect(() => {
    if (!hasCredentials()) {
      setError('Откройте приложение из Telegram.');
      return;
    }
    reload();
  }, [reload]);

  if (error) return <div className="center muted">{error}</div>;
  if (!me) return <div className="center muted">Загрузка…</div>;
  if (!me.member || !me.group) return <Onboarding me={me} initialCode={inviteCodeFromLaunch()} onDone={reload} />;

  const isHeadman = me.member.role === 'headman';
  return (
    <div className="app">
      <main>
        {tab === 'today' && <Today me={me} />}
        {tab === 'schedule' && <Schedule group={me.group} isHeadman={isHeadman} />}
        {tab === 'stats' && <StatsScreen group={me.group} isHeadman={isHeadman} />}
        {tab === 'group' && <GroupScreen me={me} onChanged={reload} />}
      </main>
      <nav className="tabbar">
        {TABS.map((t) => (
          <button key={t.id} className={tab === t.id ? 'active' : ''} onClick={() => setTab(t.id)}>
            <span>{t.icon}</span>
            {t.label}
          </button>
        ))}
      </nav>
    </div>
  );
}
