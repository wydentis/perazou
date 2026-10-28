import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import type { GroupDetails, Me } from '../types';
import { confirmDialog, haptic, tg } from '../telegram';
import { Icon } from '../icons';
import { Avatar, Skeleton, humanError, useToast } from '../ui';
import { Field } from './Onboarding';
import { plural } from '../util';

export default function GroupScreen({ me, onChanged }: { me: Me; onChanged: () => void }) {
  const isHeadman = me.member!.role === 'headman';
  const toast = useToast();
  const [d, setD] = useState<GroupDetails | null>(null);
  const [name, setName] = useState(me.group!.name);
  const [myName, setMyName] = useState(me.member!.full_name);
  // Headman renaming a student inline: whose row is open and the draft name.
  const [renaming, setRenaming] = useState<{ id: number; name: string } | null>(null);
  const load = useCallback(() => {
    api.group().then(setD, (e) => toast(humanError(e)));
  }, [toast]);
  useEffect(load, [load]);

  const run = (fn: () => Promise<unknown>, okText?: string, after?: () => void) => async () => {
    try {
      await fn();
      if (okText) { haptic.ok(); toast(okText, 'ok'); }
      (after ?? load)();
    } catch (e) {
      haptic.err();
      toast(humanError(e));
    }
  };

  function share() {
    if (!d?.invite_link) return;
    const text = encodeURIComponent(`Вступай в группу ${me.group!.name} — отмечаем посещаемость тут`);
    const url = `https://t.me/share/url?url=${encodeURIComponent(d.invite_link)}&text=${text}`;
    if (tg) tg.openTelegramLink(url);
    else window.open(url, '_blank');
  }
  async function copy() {
    if (!d?.invite_link) return;
    try {
      await navigator.clipboard.writeText(d.invite_link);
      haptic.ok();
      toast('Ссылка скопирована', 'ok');
    } catch {
      toast('Не удалось скопировать');
    }
  }

  return (
    <div className="page">
      <header className="top">
        <div>
          <div className="eyebrow">{isHeadman ? 'Вы староста' : 'Ваша группа'}</div>
          <h1 className="display">{me.group!.name}</h1>
        </div>
      </header>

      {isHeadman && (
        d?.invite_code ? (
          <div className="invite-card">
            <div className="invite-k">Код приглашения</div>
            <div className="invite-code">{d.invite_code}</div>
            {d.invite_link ? (
              <div className="invite-actions">
                <button className="btn ink" onClick={share}><Icon name="share" size={18} /> Поделиться</button>
                <button className="btn square" onClick={copy} aria-label="Скопировать"><Icon name="copy" size={18} /></button>
                <button className="btn square" aria-label="Сменить код" onClick={run(async () => {
                  if (await confirmDialog('Старая ссылка перестанет работать. Продолжить?')) await api.regenerateInvite();
                })}><Icon name="refresh" size={18} /></button>
              </div>
            ) : (
              <p className="invite-note">Задайте BOT_USERNAME на сервере, чтобы получить ссылку</p>
            )}
          </div>
        ) : <Skeleton h={190} />
      )}

      <section>
        <h3 className="section-title">
          {d ? `${d.members.length} ${plural(d.members.length, 'участник', 'участника', 'участников')}` : 'Участники'}
        </h3>
        {!d ? <Skeleton h={160} /> : (
          <div className="card list">
            {d.members.map((m) => renaming?.id === m.user_id ? (
              <form key={m.user_id} className="list-row rename-row" onSubmit={(e) => {
                e.preventDefault();
                run(() => api.rename(m.user_id, renaming.name), 'Имя изменено', () => { setRenaming(null); load(); })();
              }}>
                <Avatar name={renaming.name || m.full_name} id={m.user_id} />
                <input className="grow" value={renaming.name} maxLength={100} autoFocus
                  onChange={(e) => setRenaming({ id: m.user_id, name: e.target.value })} />
                <button type="submit" className="icon-btn ok" aria-label="Сохранить"
                  disabled={!renaming.name.trim() || renaming.name.trim() === m.full_name}><Icon name="check" size={18} /></button>
                <button type="button" className="icon-btn" aria-label="Отмена" onClick={() => setRenaming(null)}><Icon name="x" size={18} /></button>
              </form>
            ) : (
              <div key={m.user_id} className="list-row">
                <Avatar name={m.full_name} id={m.user_id} />
                <div className="grow">
                  <div className="row-title">
                    {m.full_name}
                    {m.user_id === me.member!.user_id && <span className="you">вы</span>}
                  </div>
                  <div className="meta">{m.role === 'headman' ? <><Icon name="crown" size={13} /> староста</> : m.username ? '@' + m.username : 'студент'}</div>
                </div>
                {isHeadman && m.user_id !== me.member!.user_id && (
                  <button className="icon-btn ghost" aria-label="Переименовать"
                    onClick={() => setRenaming({ id: m.user_id, name: m.full_name })}><Icon name="edit" size={17} /></button>
                )}
                {isHeadman && m.role !== 'headman' && (
                  <button className="icon-btn danger" aria-label="Исключить" onClick={run(async () => {
                    if (await confirmDialog(`Исключить ${m.full_name}? Его отметки будут удалены.`)) await api.removeMember(m.user_id);
                  })}><Icon name="x" size={18} /></button>
                )}
              </div>
            ))}
          </div>
        )}
      </section>

      <section>
        <h3 className="section-title">Профиль</h3>
        <div className="card form">
          <Field label="Как вас видит группа">
            <div className="inline">
              <input value={myName} onChange={(e) => setMyName(e.target.value)} maxLength={100} />
              <button className="btn primary sm" disabled={!myName.trim() || myName === me.member!.full_name}
                onClick={run(() => api.rename(me.member!.user_id, myName), 'Имя обновлено', onChanged)}>OK</button>
            </div>
          </Field>
        </div>
      </section>

      {isHeadman ? (
        <section>
          <h3 className="section-title">Настройки группы</h3>
          <div className="card form">
            <Field label="Название">
              <input value={name} onChange={(e) => setName(e.target.value)} maxLength={64} />
            </Field>
            <p className="hint">Часовой пояс: {me.group!.timezone}</p>
          </div>
          <button className="btn primary" onClick={run(() => api.updateGroup({ name, timezone: me.group!.timezone }), 'Сохранено', onChanged)}>
            Сохранить
          </button>
        </section>
      ) : (
        <button className="btn danger" onClick={run(async () => {
          if (await confirmDialog('Выйти из группы? Ваши отметки будут удалены.')) await api.leave();
        }, undefined, onChanged)}><Icon name="logout" size={18} /> Выйти из группы</button>
      )}
    </div>
  );
}
