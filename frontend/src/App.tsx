import { useCallback, useEffect, useMemo, useState } from 'react';
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
import MemberScreen, { OpenMemberContext } from './screens/Member';

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
  // A member's attendance page, opened on top of the current tab (which stays mounted).
  const [memberId, setMemberId] = useState<number | null>(null);
  const [scrollY, setScrollY] = useState(0);
  const [memberVersion, setMemberVersion] = useState(0);
  const openMember = useCallback((id: number) => {
    haptic.tap();
    setScrollY(window.scrollY);
    setMemberId(id);
    window.scrollTo(0, 0);
  }, []);
  const closeMember = useCallback(() => {
    setMemberId(null);
    requestAnimationFrame(() => window.scrollTo(0, scrollY));
  }, [scrollY]);
  const toast = useToast();

  const reload = useCallback(() => {
    api.me().then(setMe, (e) => setError(humanError(e)));
  }, []);
  const memberCtx = useMemo(() => ({ open: openMember, version: memberVersion }), [openMember, memberVersion]);
  const memberChanged = useCallback(() => { setMemberVersion((v) => v + 1); reload(); }, [reload]);
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
    <OpenMemberContext.Provider value={memberCtx}>
    <div className="app">
      <main key={tab} className="screen">
        <div hidden={memberId !== null}>
          {tab === 'today' && <Today me={me} onChanged={reload} />}
          {tab === 'schedule' && <Schedule group={me.group} isHeadman={isHeadman} />}
          {tab === 'stats' && <StatsScreen me={me} />}
          {tab === 'group' && <GroupScreen me={me} onChanged={reload} />}
          {tab === 'admin' && me.is_admin && <AdminScreen />}
        </div>
        {memberId !== null && <div className="screen"><MemberScreen key={memberId} me={me} userId={memberId} onBack={closeMember} onChanged={memberChanged} /></div>}
        <Footer />
      </main>
      <nav className="tabbar">
        {tabs.map((t) => (
          <button key={t.id} className={tab === t.id ? 'active' : ''}
            onClick={() => { if (t.id !== tab) haptic.select(); setMemberId(null); setTab(t.id); }}>
            <Icon name={t.icon} size={22} />
            <span>{t.label}</span>
          </button>
        ))}
      </nav>
    </div>
    </OpenMemberContext.Provider>
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
