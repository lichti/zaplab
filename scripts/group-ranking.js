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
//        JID filter:   (deixe vazio para todos os grupos, ou informe um @g.us específico)
//        Enabled:      true
//
//   4. Envie /ranking ou /ranking 7 em qualquer grupo pelo WhatsApp do bot.
//      Somente o bot consegue acionar (IsFromMe = true).
//
// ── Comportamento ──────────────────────────────────────────────────────────────
//   Trigger:  /ranking <dias>  (padrão: 30, máximo: 365)
//   Condições: somente em grupos, somente se enviado pelo próprio bot
//   Saída:     Top 5 mais ativos, top 5 menos ativos, contagem de silenciosos
//
//   Fonte de dados: GET /zaplab/api/groups/<jid>/overview?period=<dias>
//   Os dados são os mesmos exibidos na aba Group Overview da interface.
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

  var chat = info.Chat; // e.g. "120363316801436179@g.us"

  // ── Fetch overview from the internal API ───────────────────────────────────
  // Uses zaplab.api() which automatically injects the X-API-Token header.
  var data = zaplab.api(
    "/zaplab/api/groups/" + encodeURIComponent(chat) + "/overview?period=" + days
  );

  // ── Extract fields ─────────────────────────────────────────────────────────
  var top5    = data.top5_active        || [];
  var least5  = data.top5_least_active  || [];
  var summary = data.summary            || {};
  var silent  = summary.silent_members  || 0;
  var total   = summary.known_members   || 0;

  // ── Format ─────────────────────────────────────────────────────────────────
  var medals = ["\uD83E\uDD47", "\uD83E\uDD48", "\uD83E\uDD49", "4\uFE0F\u20E3", "5\uFE0F\u20E3"];

  function row(i, m) {
    var c = m.msg_count || 0;
    return (medals[i] || (i + 1) + ".") + " " + (m.name || m.jid) +
           " \u2014 " + c + " msg" + (c !== 1 ? "s" : "");
  }

  var L = [
    "\uD83D\uDCCA *Ranking \u2014 últimos " + days + " dia" + (days === 1 ? "" : "s") + "*",
    ""
  ];

  L.push("\uD83D\uDD25 *Mais ativos*");
  if (top5.length === 0) {
    L.push("Nenhuma mensagem no período.");
  } else {
    for (var i = 0; i < top5.length; i++) L.push(row(i, top5[i]));
  }

  L.push("");
  L.push("\uD83D\uDC22 *Menos ativos*");
  if (least5.length === 0) {
    L.push("Nenhuma mensagem no período.");
  } else {
    for (var i = 0; i < least5.length; i++) L.push(row(i, least5[i]));
  }

  L.push("");
  L.push(
    "\uD83D\uDE34 *Sem interação:* " + silent + " de " + total + " membro" + (total !== 1 ? "s" : "")
  );

  wa.sendText(chat, L.join("\n"));
})();
