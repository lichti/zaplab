// ── group-ranking.js ──────────────────────────────────────────────────────────
//
// HOWTO — Como configurar este script no zaplab:
//
//   1. Acesse a interface admin: http://127.0.0.1:8090/_/
//      Ou a interface zaplab: http://127.0.0.1:8090/
//
//   2. Crie o Script:
//      Menu → Scripts → New script
//        Name:        group-ranking
//        Description: Posta ranking de atividade do grupo via /ranking <dias>
//        Timeout:     30
//        Code:        (cole o conteúdo deste arquivo a partir da linha do IIFE)
//
//   3. Crie o Trigger:
//      Menu → Script Triggers → New trigger
//        Script:       group-ranking
//        Event type:   Message
//        Text pattern: /ranking
//        JID filter:   (deixe vazio para todos os grupos, ou cole um @g.us específico)
//        Enabled:      true
//
//   4. Envie /ranking ou /ranking 7 em qualquer grupo pelo WhatsApp do bot.
//      Só o bot consegue acionar (IsFromMe = true).
//
// ── Comportamento ──────────────────────────────────────────────────────────────
//   Trigger:  /ranking <dias>  (padrão: 30, máximo: 365)
//   Condições: somente em grupos, somente se enviado pelo próprio bot
//   Saída:     Top 5 mais ativos, top 5 menos ativos, contagem de silenciosos
//
// ─────────────────────────────────────────────────────────────────────────────

(function () {
  // ── Guards ─────────────────────────────────────────────────────────────────
  var info = event && event.Info;
  if (!info || !info.IsFromMe || !info.IsGroup) return;

  // ── Parse command ──────────────────────────────────────────────────────────
  var msg = event.Message || {};
  var txt = (
    msg.conversation ||
    (msg.extendedTextMessage && msg.extendedTextMessage.text) ||
    ""
  ).trim();

  var parts = txt.split(/\s+/);
  if (!parts[0] || parts[0].toLowerCase() !== "/ranking") return;

  var days = parseInt(parts[1], 10);
  if (isNaN(days) || days < 1) days = 30;
  if (days > 365) days = 365;

  var chat    = info.Chat;
  var botUser = (wa.jid || "").split("@")[0].split(":")[0];

  // ── 1. Message stats per sender ────────────────────────────────────────────
  var stats = db.query(
    "SELECT json_extract(raw,'$.Info.Sender') AS jid, COUNT(*) AS cnt" +
    " FROM events" +
    " WHERE type LIKE '%Message%'" +
    "   AND json_extract(raw,'$.Info.Chat') = '" + chat + "'" +
    "   AND json_extract(raw,'$.Info.IsFromMe') = 0" +
    "   AND datetime(created) >= datetime('now','-" + days + " days')" +
    " GROUP BY jid"
  );

  // ── 2. Build msgMap — normalise device-suffix LIDs ("123:15@lid" → "123@lid")
  var msgMap = {};
  for (var i = 0; i < stats.length; i++) {
    var r = stats[i];
    if (!r.jid) continue;
    var k = r.jid.replace(/:[\d]+@/, "@");
    var c = parseInt(r.cnt, 10) || 0;
    msgMap[k] = (msgMap[k] || 0) + c;
    if (k !== r.jid) msgMap[r.jid] = msgMap[k];
  }

  // ── 3. LID ↔ PN alias normalisation (mirrors Go loadLIDMap) ───────────────
  var lids = wa.db.query(
    "SELECT lid || '@lid' AS l, pn || '@s.whatsapp.net' AS p FROM whatsmeow_lid_map"
  );
  for (var i = 0; i < lids.length; i++) {
    var l = lids[i].l, p = lids[i].p;
    var hl = msgMap[l] !== undefined, hp = msgMap[p] !== undefined;
    if (hl && hp) {
      var merged = msgMap[l] + msgMap[p];
      msgMap[l] = merged; msgMap[p] = merged;
    } else if (hl) { msgMap[p] = msgMap[l];
    } else if (hp) { msgMap[l] = msgMap[p]; }
  }

  // ── 4. Contact names ───────────────────────────────────────────────────────
  var names = {};
  var cn = wa.db.query(
    "SELECT their_jid AS j," +
    " COALESCE(NULLIF(push_name,''), NULLIF(full_name,''), their_jid) AS n" +
    " FROM whatsmeow_contacts"
  );
  for (var i = 0; i < cn.length; i++) names[cn[i].j] = cn[i].n;

  var ln = wa.db.query(
    "SELECT m.lid || '@lid' AS j," +
    " COALESCE(NULLIF(c.push_name,''), NULLIF(c.full_name,''), m.pn || '@s.whatsapp.net') AS n" +
    " FROM whatsmeow_lid_map m" +
    " LEFT JOIN whatsmeow_contacts c ON c.their_jid = m.pn || '@s.whatsapp.net'"
  );
  for (var i = 0; i < ln.length; i++) names[ln[i].j] = ln[i].n;

  function nm(jid) {
    if (names[jid]) return names[jid];
    var u = jid.split("@")[0].split(":")[0];
    return names[u + "@s.whatsapp.net"] || u;
  }

  // ── 5. Current members ─────────────────────────────────────────────────────
  // Primary: group_membership table (records join/leave events observed by the bot)
  // Fallback: active senders from events (covers pre-existing groups where
  //           group_membership may be empty)
  var seen = {}, members = [];
  function addMember(jid) {
    if (!jid || seen[jid]) return;
    var u = jid.split("@")[0].split(":")[0];
    if (u === botUser) return;
    seen[jid] = true;
    members.push({ jid: jid, name: nm(jid), count: msgMap[jid] || 0 });
  }

  var rows = db.query(
    "SELECT m.member_jid FROM group_membership m" +
    " INNER JOIN (" +
    "   SELECT member_jid, MAX(created) AS mc" +
    "   FROM group_membership WHERE group_jid = '" + chat + "' GROUP BY member_jid" +
    " ) lt ON m.member_jid = lt.member_jid AND m.created = lt.mc" +
    " WHERE m.group_jid = '" + chat + "' AND m.action NOT IN ('leave')" +
    " ORDER BY m.member_jid"
  );
  for (var i = 0; i < rows.length; i++) addMember(rows[i].member_jid);

  // Fallback: use observed senders when membership table is empty
  if (members.length === 0) {
    for (var jid in msgMap) {
      if (msgMap[jid] > 0) addMember(jid);
    }
  }

  // ── 6. Sort and slice ──────────────────────────────────────────────────────
  members.sort(function (a, b) { return b.count - a.count; });

  var total = members.length, silent = 0;
  for (var i = 0; i < members.length; i++) { if (members[i].count === 0) silent++; }

  var top5 = [], active = [];
  for (var i = 0; i < members.length; i++) {
    if (members[i].count > 0) {
      if (top5.length < 5) top5.push(members[i]);
      active.push(members[i]);
    }
  }
  var bot5 = active.length > 5 ? active.slice(active.length - 5).reverse() : [];

  // ── 7. Format and send ─────────────────────────────────────────────────────
  var medals = ["\uD83E\uDD47", "\uD83E\uDD48", "\uD83E\uDD49", "4\uFE0F\u20E3", "5\uFE0F\u20E3"];
  function row(i, m) {
    return (medals[i] || (i + 1) + ".") + " " + m.name + " \u2014 " + m.count + " msg" + (m.count !== 1 ? "s" : "");
  }

  var L = [
    "\uD83D\uDCCA *Ranking \u2014 últimos " + days + " dia" + (days === 1 ? "" : "s") + "*",
    ""
  ];

  L.push("\uD83D\uDD25 *Mais ativos*");
  if (top5.length === 0) { L.push("Nenhuma mensagem no período."); }
  else { for (var i = 0; i < top5.length; i++) L.push(row(i, top5[i])); }

  L.push("");
  L.push("\uD83D\uDC22 *Menos ativos*");
  if (bot5.length === 0) { L.push(active.length > 0 ? "(todos listados acima)" : "Nenhuma mensagem no período."); }
  else { for (var i = 0; i < bot5.length; i++) L.push(row(i, bot5[i])); }

  L.push("");
  L.push("\uD83D\uDE34 *Sem interação:* " + silent + " de " + total + " membro" + (total !== 1 ? "s" : ""));

  wa.sendText(chat, L.join("\n"));
})();
