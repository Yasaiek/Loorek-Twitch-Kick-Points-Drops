// Loorek panel preview for the project site.
// The page next to this file is the program's own panel (ui/index.html, copied by tools/site_demo.py).
// This script stands in for the program: it answers the panel's /api/ requests from made-up data
// kept in the page. Nothing here talks to Twitch or Kick, and nothing is saved.
(() => {
  const params = new URLSearchParams(location.search);
  const LANGS = ['en', 'pl', 'de', 'fr', 'es', 'it', 'pt_BR', 'ru', 'uk', 'tr', 'ja', 'zh_CN'];
  let siteLang = LANGS.includes(params.get('lang')) ? params.get('lang') : 'en';
  const IMG = '../img/', DAY = 86400000, START = Date.now();
  const realFetch = window.fetch.bind(window);
  window.close = () => {};

  // ---- journal texts come from the panel's own language files ----
  let dict = {}, dictLang = '';
  async function loadDict(l) { if (dictLang === l) return; dictLang = l; try { dict = await (await realFetch('lang/' + l + '.json', { cache: 'no-store' })).json(); } catch (e) {} }
  const say = (key, ...a) => { let i = 0; return (dict[key] || key).replace(/%[sdv]/g, () => a[i++]); };
  const log = [];
  const note = (level, key, ...a) => { log.push({ t: Date.now(), l: level, m: say(key, ...a) }); if (log.length > 300) log.shift(); };

  // ---- made-up world ----
  const game = (id, slug, name) => ({ id, slug, name, art: IMG + 'games/' + slug + '.png' });
  const GAMES = {
    g21: game('g21', 'apex-legends', 'Apex Legends'), g23: game('g23', 'cyberpunk-2077', 'Cyberpunk 2077'), g24: game('g24', 'delta-force', 'Delta Force'),
    g25: game('g25', 'genshin-impact', 'Genshin Impact'), g26: game('g26', 'rainbow-six-siege', 'Rainbow Six Siege'), g27: game('g27', 'sea-of-thieves', 'Sea of Thieves'),
    g22: game('g22', 'crimson-desert', 'Crimson Desert'), // known game, no campaign right now
  };
  const drop = (id, name, req, cur, claimed, img) => ({ id, name, req, cur: cur || 0, claimed: !!claimed, img: img ? IMG + 'drops/' + img + '.png' : '' });
  const many = (p, n, req, img) => Array.from({ length: n }, (_, i) => drop(p + (i + 1), 'Reward ' + (i + 1), typeof req === 'function' ? req(i + 1) : req, 0, false, img ? img + (i + 1) : ''));
  const sot = many('sot', 7, (i) => (i < 3 ? 30 : 60), 'sot'); sot[0].cur = 30; sot[0].claimed = true; sot[1].cur = 30; sot[1].claimed = true; sot[2].cur = 12; sot[2].name = 'Face, Meet Palm Emote';
  const camp = (id, name, g, linked, ch, drops, days) => ({ id, name, game: g, linked, ch, drops, start: START - 2 * DAY, end: START + days * DAY, link: linked ? '' : 'https://www.twitch.tv/drops/campaigns' });
  const CAMPS = [
    camp('R1', 'Apex Legends | Godzilla', 'g21', true, { login: 'playapex', name: 'PlayApex' }, [drop('ax1', 'Godzilla Weapon Charm', 60)], 17),
    camp('R3', 'Quadra Paintjobs', 'g23', true, { login: 'cdprojektred', name: 'CDPROJEKTRED' }, [drop('cp1', 'Quadra Paintjobs', 120, 47, false, 'cp1')], 5),
    camp('R4', 'Delta Force Drops', 'g24', false, { login: 'deltaforcegame', name: 'DeltaForceGame' }, many('df', 2, 60), 12),
    camp('R6', 'V7.2 Genshin Impact Drops', 'g25', true, { login: 'genshinimpactofficial', name: 'GenshinImpactOfficial' }, [drop('gi1', 'Primogem ×30', 60)], 10),
    camp('R7', 'Twitch Drops', 'g26', true, { login: 'rainbow6', name: 'Rainbow6' }, many('r6', 6, 60, 'r6'), 3),
    camp('R8', 'Twitch Drops', 'g27', true, { login: 'seaofthieves', name: 'SeaOfThieves' }, sot, 1),
  ];
  const tw = (login, name, online, balance, followers) => ({ login, name, img: IMG + 'avatars/user-' + login + '.png', online, game: online ? 'Just Chatting' : '', balance, followers, earned: 0, claims: 0, watchedSec: 0, known: true });
  const TWITCH = [tw('h2p_gucio', 'H2P_Gucio', true, 48210, 412000), tw('izakooo', 'IzakOOO', false, 8340, 1900000), tw('ewroon', 'EWROON', true, 27600, 980000), tw('pago3', 'PAGO3', false, 1920, 760000),
    tw('xayoo_', 'xayoo_', true, 12750, 1400000), tw('cdprojektred', 'CDPROJEKTRED', true, 0, 870000), tw('seaofthieves', 'SeaOfThieves', true, 0, 1300000), tw('rainbow6', 'Rainbow6', true, 0, 2400000)];
  const kk = (login, name, online, balance, followers) => ({ login, name, img: IMG + 'avatars/kuser-' + login + '.png', online, game: online ? 'Just Chatting' : '', balance, followers, earned: 0, watchedSec: 0, known: true });
  const KICK = [kk('delordione', 'delordione', true, 3480, 610000), kk('azdus', 'Azdus', true, 1210, 84000), kk('demonzz1', 'demonzz1', false, 640, 230000), kk('pajalock', 'pajalock', true, 90, 152000)];

  const S = {
    auth: { state: 'ok', login: 'viewer', note: '', mode: 'auto' },
    set: { pointsOn: true, chatOn: true, dropsOn: true, autoClaim: true, rotateMin: 0, stallMin: 15, autostart: false, openOnStart: true, kickOn: true, kickMax: 2, autoUpdate: true, lang: 'auto' },
    chans: ['h2p_gucio', 'izakooo', 'ewroon', 'pago3'].map((l) => ({ login: l, on: true })),
    kick: { state: 'ok', user: 'viewer_k', login: false, chans: ['delordione', 'azdus', 'demonzz1', 'pajalock'].map((l) => ({ login: l, on: true })) },
    queue: [], tracked: [], cur: null, upd: { state: '', checked: 0 }, tick: 0,
  };
  const lang = () => (LANGS.includes(S.set.lang) ? S.set.lang : siteLang);
  const twOf = (l) => TWITCH.find((c) => c.login === l), kkOf = (l) => KICK.find((c) => c.login === l);

  // ---- drops ----
  const campsOf = (id) => CAMPS.filter((c) => c.game === id);
  const earnable = (c) => (c.linked ? c.drops.filter((d) => !d.claimed && d.cur < d.req) : []);
  function status(id) {
    const cs = campsOf(id); if (!cs.length) return 'nocamp';
    if (cs.some((c) => earnable(c).length)) return 'ready';
    const ds = cs.flatMap((c) => c.drops);
    if (ds.some((d) => !d.claimed && d.cur >= d.req)) return 'claim';
    if (ds.every((d) => d.claimed)) return 'done';
    return cs.some((c) => !c.linked) ? 'nolink' : 'blocked';
  }
  const dropView = (c, d) => ({ id: d.id, name: d.name, img: d.img, cur: Math.floor(d.cur), req: d.req, claimed: d.claimed, pre: [], blocked: false, end: c.end });
  const campView = (c) => ({ id: c.id, name: c.name, start: c.start, end: c.end, active: true, linked: c.linked, link: c.link, channels: [], channelCount: 0,
    drops: c.drops.map((d) => dropView(c, d)), left: c.drops.reduce((n, d) => n + (d.claimed || d.cur >= d.req ? 0 : d.req - Math.floor(d.cur)), 0) });
  const queuePos = (id) => S.queue.indexOf(id) + 1;
  function entry(id) {
    const g = GAMES[id], cs = campsOf(id), farming = !!S.cur && S.cur.game === id;
    return { id: g.id, slug: g.slug, name: g.name, art: g.art, status: status(id), note: '', farming, channel: farming ? S.cur.camp.ch.name : '',
      drops: cs.flatMap((c) => c.drops.map((d) => dropView(c, d))), camps: cs.map(campView), link: (cs.find((c) => !c.linked) || {}).link || '',
      end: cs.length ? Math.min(...cs.map((c) => c.end)) : 0, tracked: S.tracked.includes(id), queued: queuePos(id) };
  }
  function pickFarm() {
    if (!S.set.dropsOn || S.auth.state !== 'ok') { S.cur = null; return; }
    for (const id of S.queue.concat(S.tracked.filter((t) => !S.queue.includes(t)))) {
      for (const c of campsOf(id)) {
        const d = earnable(c)[0]; if (!d) continue;
        if (!S.cur || S.cur.game !== id) { S.cur = { game: id, camp: c, since: Date.now() }; note('info', 'L.farm', GAMES[id].name, c.ch.name); }
        S.cur.camp = c; S.cur.drop = d; return;
      }
    }
    S.cur = null;
  }

  // ---- who is being watched ----
  function watchers() {
    if (S.auth.state !== 'ok' || !S.set.pointsOn) return [];
    const free = S.cur ? 1 : 2;
    return S.chans.filter((c) => c.on && twOf(c.login) && twOf(c.login).online).slice(0, free).map((c) => c.login);
  }
  const kickWatchers = () => (S.kick.state === 'ok' && S.set.kickOn ? S.kick.chans.filter((c) => c.on && kkOf(c.login) && kkOf(c.login).online).slice(0, S.set.kickMax).map((c) => c.login) : []);

  // ---- time passes (much faster than in real life, so there is something to see) ----
  function tick() {
    if (document.hidden) return;
    S.tick++;
    pickFarm();
    if (S.cur) {
      const d = S.cur.drop, g = GAMES[S.cur.game];
      d.cur = Math.min(d.req, d.cur + 0.5);
      if (d.cur >= d.req) {
        note('ok', 'L.dropDone', g.name, d.name);
        if (S.set.autoClaim) { d.claimed = true; note('ok', 'L.claimed', d.name, g.name); }
        if (status(g.id) !== 'ready') { note('info', 'L.gameDone', g.name); S.cur = null; pickFarm(); }
      }
    }
    watchers().forEach((l, i) => {
      const c = twOf(l); c.watchedSec += 20;
      if (S.tick % 6 === i) { c.balance += 10; c.earned += 10; }
      if (S.tick % 45 === 20 + i * 11) { c.balance += 50; c.earned += 50; c.claims++; note('ok', 'L.bonus', c.name); }
    });
    kickWatchers().forEach((l, i) => { const c = kkOf(l); c.watchedSec += 20; if (S.tick % 8 === i) { c.balance += 10; c.earned += 10; } });
  }
  setInterval(tick, 1000);

  // ---- what the panel sees ----
  function view() {
    const w = watchers(), kw = kickWatchers(), signed = S.auth.state === 'ok';
    const channels = S.chans.map((c) => { const t = twOf(c.login); return { login: c.login, name: t.name, img: t.img, on: c.on, online: signed && t.online, watching: w.includes(c.login), game: t.game, balance: t.balance, balKnown: signed && t.known,
      earned: t.earned, claims: t.claims, missing: !t.known, viewers: 0, watchedMin: Math.floor(t.watchedSec / 60) }; });
    const kchannels = S.kick.chans.map((c) => { const t = kkOf(c.login); return { login: c.login, name: t.name, img: t.img, on: c.on, online: t.online, watching: kw.includes(c.login), game: t.game, balance: t.balance, balKnown: t.known,
      earned: t.earned, missing: !t.known, watchedMin: Math.floor(t.watchedSec / 60) }; });
    const games = Object.values(GAMES).filter((g) => campsOf(g.id).length).map((g) => { const cs = campsOf(g.id); return { id: g.id, slug: g.slug, name: g.name, art: g.art, camps: cs.length, camp: cs[0].name,
      end: Math.min(...cs.map((c) => c.end)), upcoming: false, start: cs[0].start, status: status(g.id), queued: queuePos(g.id), tracked: S.tracked.includes(g.id) }; }).sort((a, b) => a.name.localeCompare(b.name));
    let current = null;
    if (S.cur) { const g = GAMES[S.cur.game], d = S.cur.drop; current = { game: g.name, art: g.art, channel: S.cur.camp.ch.name, login: S.cur.camp.ch.login, since: S.cur.since, drop: d.name, cur: Math.floor(d.cur), req: d.req, img: d.img }; }
    const all = (list) => list.reduce((n, c) => n + c.earned, 0);
    return {
      version: window.LOOREK_VERSION || '0.6', app: 'Loorek', lang: lang(), winGen: 0, usage: { cpu: +(0.3 + Math.random() * 0.5).toFixed(2), ram: 140 + Math.round(Math.random() * 10), ramPct: 0.9 }, upSec: Math.floor((Date.now() - START) / 1000) * 20,
      update: { state: S.upd.state, latest: '', checked: S.upd.checked, doneAt: 0 },
      auth: S.auth, settings: S.set,
      points: { channels, earned: all(channels), claims: channels.reduce((n, c) => n + c.claims, 0), watching: w.length },
      chat: { state: signed && S.set.chatOn ? 'on' : 'off', joined: signed && S.set.chatOn ? S.chans.filter((c) => c.on).length : 0 },
      kick: { state: S.kick.state === 'ok' && !S.set.kickOn ? 'off' : S.kick.state, user: S.kick.user, on: S.set.kickOn, max: S.set.kickMax, channels: kchannels, earned: all(kchannels), watching: kw.length, login: S.kick.login, loginNote: '' },
      drops: { fetchedAt: signed ? START : 0, err: '', games: signed ? games : [], queue: S.queue.map(entry), tracked: S.tracked.map(entry), current },
      log: log.slice(),
    };
  }
  function gameView(id) {
    const g = GAMES[id]; if (!g) return { id, camps: [] };
    return { id: g.id, slug: g.slug, name: g.name, art: g.art, status: status(id), queued: queuePos(id), tracked: S.tracked.includes(id), camps: campsOf(id).map(campView) };
  }

  // ---- what the panel asks for ----
  const clean = (raw, extra) => { let s = String(raw || '').trim().toLowerCase(); s = s.replace(/^.*(twitch\.tv|kick\.com)\//, '').replace(/^@/, '').split(/[/?# ]/)[0]; return new RegExp('^[a-z0-9_' + extra + ']{2,40}$').test(s) ? s : ''; };
  const reorder = (list, order, key) => list.sort((a, b) => ((order.indexOf(key(a)) + 1) || 1e9) - ((order.indexOf(key(b)) + 1) || 1e9));
  const toggle = (list, id) => { const i = list.indexOf(id); if (i >= 0) list.splice(i, 1); else if (GAMES[id]) list.push(id); };
  function op(q) {
    const out = { ok: true };
    switch (q.op) {
      case 'login': case 'loginManual':
        S.auth = { state: 'login', login: '', note: '', mode: 'auto' }; note('info', 'L.loginWin');
        setTimeout(() => { if (S.auth.state === 'login') { S.auth = { state: 'ok', login: 'viewer', note: '', mode: 'auto' }; note('info', 'L.loginGot'); note('ok', 'L.signedIn', 'viewer'); } }, 1800); break;
      case 'cancelLogin': S.auth = { state: 'out', login: '', note: '', mode: 'auto' }; break;
      case 'logout': S.auth = { state: 'out', login: '', note: '', mode: 'auto' }; S.cur = null; note('info', 'L.signedOut'); break;
      case 'token': out.result = 'bad'; break;
      case 'addChannel': case 'kickAdd': {
        const kick = q.op === 'kickAdd', login = clean(q.login, kick ? '-' : ''), list = kick ? S.kick.chans : S.chans, pool = kick ? KICK : TWITCH;
        if (!login) { out.result = 'bad'; break; }
        if (list.some((c) => c.login === login)) { out.result = 'dup'; break; }
        if (!pool.some((c) => c.login === login)) pool.push({ login, name: q.name || login, img: '', online: false, game: '', balance: 0, followers: 0, earned: 0, claims: 0, watchedSec: 0, known: false });
        list.push({ login, on: true }); out.result = 'ok'; break;
      }
      case 'removeChannel': S.chans = S.chans.filter((c) => c.login !== q.login); break;
      case 'kickRemove': S.kick.chans = S.kick.chans.filter((c) => c.login !== q.login); break;
      case 'toggleChannel': S.chans.forEach((c) => { if (c.login === q.login) c.on = !!q.on; }); break;
      case 'kickToggle': S.kick.chans.forEach((c) => { if (c.login === q.login) c.on = !!q.on; }); break;
      case 'reorderChannels': reorder(S.chans, q.order || [], (c) => c.login); break;
      case 'kickReorder': reorder(S.kick.chans, q.order || [], (c) => c.login); break;
      case 'kickLogin': S.kick.login = true; note('info', 'L.kickWin'); setTimeout(() => { if (S.kick.login) { S.kick.login = false; S.kick.state = 'ok'; note('info', 'L.kickGot'); note('ok', 'L.kickSignedIn', S.kick.user); } }, 1800); break;
      case 'kickLoginCancel': S.kick.login = false; break;
      case 'kickToken': out.result = 'bad'; break;
      case 'kickLogout': S.kick.state = 'out'; S.kick.login = false; note('info', 'L.kickOut'); break;
      case 'queueToggle': toggle(S.queue, (q.game || {}).id); break;
      case 'queueReorder': reorder(S.queue, q.order || [], (id) => id); break;
      case 'trackToggle': toggle(S.tracked, (q.game || {}).id); break;
      case 'settings':
        for (const k in q.patch || {}) {
          const v = q.patch[k];
          if (k === 'lang') S.set.lang = LANGS.includes(v) ? v : 'auto';
          else if (k === 'kickMax') S.set.kickMax = Math.min(5, Math.max(1, Number(v) || 1));
          else if (k === 'rotateMin') S.set.rotateMin = Math.min(720, Math.max(0, Number(v) || 0));
          else if (k === 'stallMin') S.set.stallMin = Math.min(120, Math.max(5, Number(v) || 5));
          else if (k in S.set) S.set[k] = !!v;
        }
        loadDict(lang()); break;
      case 'updateCheck': case 'updateInstall': S.upd.state = 'checking'; setTimeout(() => { S.upd = { state: 'current', checked: Date.now() }; }, 900); break;
      case 'clearLog': log.length = 0; break;
      case 'refresh': case 'quit': break;
      default: out.ok = false;
    }
    pickFarm();
    return out;
  }
  function search(kind, q) {
    q = String(q || '').trim().toLowerCase(); if (q.length < 2) return [];
    if (kind === 'game') return Object.values(GAMES).filter((g) => g.name.toLowerCase().includes(q)).map((g) => ({ id: g.id, slug: g.slug, name: g.name, art: g.art }));
    return (kind === 'kick' ? KICK : TWITCH).filter((c) => c.known && (c.login.includes(q) || c.name.toLowerCase().includes(q))).slice(0, 8).map((c) => ({ login: c.login, name: c.name, img: c.img, live: c.online, followers: c.followers }));
  }

  const json = (v) => new Response(JSON.stringify(v), { status: 200, headers: { 'Content-Type': 'application/json' } });
  const wait = (ms) => new Promise((r) => setTimeout(r, ms));
  window.fetch = async (input, init) => {
    const u = new URL(typeof input === 'string' ? input : input.url, location.href);
    if (!u.pathname.startsWith('/api/')) return realFetch(input, init);
    await loadDict(lang());
    if (u.pathname === '/api/state') return json(view());
    if (u.pathname === '/api/game') return json(gameView(u.searchParams.get('id')));
    if (u.pathname === '/api/search') { await wait(220); return json({ items: search(u.searchParams.get('kind'), u.searchParams.get('q')), failed: false }); }
    if (u.pathname === '/api/op') { let q = {}; try { q = JSON.parse((init && init.body) || '{}'); } catch (e) {} return json(op(q)); }
    return json({});
  };

  // the site tells the preview when its language changes
  addEventListener('message', (ev) => { const l = ev.data && ev.data.loorekLang; if (LANGS.includes(l)) { siteLang = l; loadDict(lang()); } });

  loadDict(lang()).then(() => {
    note('info', 'L.started', 'Loorek', window.LOOREK_VERSION || '');
    note('ok', 'L.signedIn', 'viewer'); note('info', 'L.chatOn');
    note('info', 'L.camps', CAMPS.length, 2);
    note('ok', 'L.kickSignedIn', 'viewer_k');
    ['delordione', 'Azdus'].forEach((n) => note('ok', 'L.kickWatch', n));
  });
})();
