import { useCallback, useEffect, useState } from 'react';
import { api, hasCredentials } from './api';
import type { Me } from './types';
import { ensureWriteAccess, haptic, inviteCodeFromLaunch } from './telegram';
import { Icon, type IconName } from './icons';
import { Footer, humanError, useToast } from './ui';
import Onboarding from './screens/Onboarding';
import Today from './screens/Today';
import Schedule from './screens/Schedule';
import StatsScreen from './screens/Stats';
import GroupScreen from './screens/GroupScreen';
import AdminScreen from './screens/Admin';

type Tab = 'today' | 'schedule' | 'stats' | 'group' | 'admin';
const TABS: { id: Tab; label: string; icon: IconName }[] = [
  { id: 'today', label: 'Пары', icon: 'calendar' },
  { id: 'schedule', label: 'Расписание', icon: 'grid' },
  { id: 'stats', label: 'Итоги', icon: 'chart' },
  { id: 'group', label: 'Группа', icon: 'users' },
];
const ADMIN_TAB: { id: Tab; label: string; icon: IconName } = { id: 'admin', label: 'Админ', icon: 'shield' };

export default function App() {
  const [me, setMe] = useState<Me | null>(null);
  const [error, setError] = useState('');
  const [tab, setTab] = useState<Tab>('today');
  const [adminOutsideGroup, setAdminOutsideGroup] = useState(false);
  const toast = useToast();

  const reload = useCallback(() => {
    api.me().then(setMe, (e) => setError(humanError(e)));
  }, []);
  useEffect(() => {
    if (!hasCredentials()) {
      setError('Откройте приложение из Telegram');
      return;
    }
    reload();
  }, [reload]);

  // Once the user is in a group, make sure the bot may send them lesson reminders.
  const inGroup = Boolean(me?.member);
  useEffect(() => {
    if (inGroup) ensureWriteAccess(() => toast('Напоминания о парах включены', 'ok'));
  }, [inGroup, toast]);

  if (error) return <Splash text={error} />;
  if (!me) return <Splash loading />;
  if (!me.member || !me.group) {
    // An admin who is not in any group can still reach the admin screen from onboarding.
    if (adminOutsideGroup) return <AdminScreen onBack={() => setAdminOutsideGroup(false)} />;
    return <Onboarding me={me} initialCode={inviteCodeFromLaunch()} onDone={reload}
      onAdmin={me.is_admin ? () => setAdminOutsideGroup(true) : undefined} />;
  }
  const tabs = me.is_admin ? [...TABS, ADMIN_TAB] : TABS;

  const isHeadman = me.member.role === 'headman';
  return (
    <div className="app">
      <main key={tab} className="screen">
        {tab === 'today' && <Today me={me} />}
        {tab === 'schedule' && <Schedule group={me.group} isHeadman={isHeadman} />}
        {tab === 'stats' && <StatsScreen me={me} />}
        {tab === 'group' && <GroupScreen me={me} onChanged={reload} />}
        {tab === 'admin' && me.is_admin && <AdminScreen />}
        <Footer />
      </main>
      <nav className="tabbar">
        {tabs.map((t) => (
          <button key={t.id} className={tab === t.id ? 'active' : ''}
            onClick={() => { if (t.id !== tab) haptic.select(); setTab(t.id); }}>
            <Icon name={t.icon} size={22} />
            <span>{t.label}</span>
          </button>
        ))}
      </nav>
    </div>
  );
}

function Splash({ text, loading }: { text?: string; loading?: boolean }) {
  return (
    <div className="splash">
      <div className="logo"><Icon name="check" size={34} strokeWidth={3} /></div>
      {loading ? <div className="dots"><i /><i /><i /></div> : <p>{text}</p>}
    </div>
  );
}
