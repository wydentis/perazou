import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Me } from '../types';

export default function Onboarding({ me, initialCode, onDone }: { me: Me; initialCode: string; onDone: () => void }) {
  const [mode, setMode] = useState<'join' | 'create'>(initialCode ? 'join' : 'create');
  const [code, setCode] = useState(initialCode);
  const [groupName, setGroupName] = useState<string | null>(null);
  const [fullName, setFullName] = useState('');
  const [name, setName] = useState('');
  const [week, setWeek] = useState<'odd' | 'even'>('odd');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'Europe/Moscow';

  useEffect(() => {
    setGroupName(null);
    if (mode !== 'join' || code.length < 8) return;
    api.preview(code).then((r) => setGroupName(r.name), () => setGroupName(null));
  }, [code, mode]);

  async function submit() {
    setBusy(true);
    setErr('');
    try {
      if (mode === 'join') await api.join({ code, full_name: fullName });
      else await api.createGroup({ name, full_name: fullName, timezone: tz, current_week: week });
      onDone();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const canSubmit = fullName.trim() && (mode === 'join' ? groupName : name.trim());
  return (
    <div className="page">
      <h1>Привет, {me.user.first_name}!</h1>
      <div className="segmented">
        <button className={mode === 'join' ? 'active' : ''} onClick={() => setMode('join')}>Я студент</button>
        <button className={mode === 'create' ? 'active' : ''} onClick={() => setMode('create')}>Я староста</button>
      </div>

      {mode === 'join' ? (
        <>
          <label>Код приглашения</label>
          <input value={code} onChange={(e) => setCode(e.target.value.toUpperCase().trim())} maxLength={8} placeholder="ABCD2345" />
          {code.length >= 8 && (groupName ? <p className="ok">Группа: {groupName}</p> : <p className="err">Группа не найдена</p>)}
        </>
      ) : (
        <>
          <label>Название группы</label>
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={64} placeholder="ИВТ-21" />
          <label>Сейчас идёт неделя</label>
          <div className="segmented">
            <button className={week === 'odd' ? 'active' : ''} onClick={() => setWeek('odd')}>Нечётная</button>
            <button className={week === 'even' ? 'active' : ''} onClick={() => setWeek('even')}>Чётная</button>
          </div>
          <p className="muted small">Часовой пояс группы: {tz}</p>
        </>
      )}

      <label>Ваши фамилия и имя</label>
      <input value={fullName} onChange={(e) => setFullName(e.target.value)} maxLength={100} placeholder="Иванов Иван" />
      {err && <p className="err">{err}</p>}
      <button className="primary" disabled={!canSubmit || busy} onClick={submit}>
        {mode === 'join' ? 'Вступить в группу' : 'Создать группу'}
      </button>
    </div>
  );
}
