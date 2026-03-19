package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(upRankingScript, nil)
}

// upRankingScript seeds the built-in /ranking command script and its trigger.
// The migration is idempotent: if a script named "group-ranking" already exists
// it is left untouched so user edits are preserved across restarts.
func upRankingScript(app core.App) error {
	scriptCol, err := app.FindCollectionByNameOrId("scripts")
	if err != nil {
		return nil // scripts collection not ready yet — skip
	}
	trigCol, err := app.FindCollectionByNameOrId("script_triggers")
	if err != nil {
		return nil // triggers collection not ready yet — skip
	}

	// Idempotency check: skip if a script with this name already exists.
	type idRow struct {
		ID string `db:"id"`
	}
	var existing []idRow
	_ = app.DB().Select("id").From("scripts").
		Where(dbx.HashExp{"name": "group-ranking"}).
		Limit(1).All(&existing)
	if len(existing) > 0 {
		return nil
	}

	script := core.NewRecord(scriptCol)
	script.Set("name", "group-ranking")
	script.Set("description", "Posts group activity ranking for /ranking <days> — groups only, bot user only.")
	script.Set("enabled", true)
	script.Set("timeout_secs", 30.0)
	script.Set("code", rankingScriptCode)
	if err := app.Save(script); err != nil {
		return err
	}

	trigger := core.NewRecord(trigCol)
	trigger.Set("script_id", script.Id)
	trigger.Set("event_type", "Message")
	trigger.Set("text_pattern", "/ranking")
	trigger.Set("enabled", true)
	return app.Save(trigger)
}

// rankingScriptCode is the JS code executed when /ranking <days> is sent by the bot
// inside a group. It posts top-5 most active, top-5 least active, and silent count.
const rankingScriptCode = `// ── /ranking <dias> ──────────────────────────────────────────────────────────
// Conditions:
//   - Only works in groups (event.Info.IsGroup)
//   - Only the bot user can trigger (event.Info.IsFromMe)
// Usage: /ranking <days>  (e.g. /ranking 30)

if (!event || !event.Info) return;
if (!event.Info.IsFromMe) return;   // ignore messages from other users
if (!event.Info.IsGroup)  return;   // ignore private chats

// ── Extract message text ──────────────────────────────────────────────────────
var msgText = '';
if (event.Message) {
  msgText = event.Message.conversation
    || (event.Message.extendedTextMessage && event.Message.extendedTextMessage.text)
    || '';
}
msgText = msgText.trim();

// ── Parse command ─────────────────────────────────────────────────────────────
var parts = msgText.split(/\s+/);
if (!parts[0] || parts[0].toLowerCase() !== '/ranking') return;

var days = parseInt(parts[1], 10);
if (isNaN(days) || days < 1) days = 30;
if (days > 365) days = 365;

var chatJID = event.Info.Chat;   // e.g. "120363425621295665@g.us"
var botJID  = wa.jid;            // e.g. "5511999999999@s.whatsapp.net"
var botUser = botJID ? botJID.split('@')[0].split(':')[0] : '';

// ── 1. Message stats per sender (PocketBase events table) ─────────────────────
var statsRows = db.query(
  "SELECT json_extract(raw,'$.Info.Sender') AS jid, COUNT(*) AS cnt " +
  "FROM events " +
  "WHERE type LIKE '%Message%' " +
  "  AND json_extract(raw,'$.Info.Chat') = '" + chatJID + "' " +
  "  AND json_extract(raw,'$.Info.IsFromMe') = 0 " +
  "  AND datetime(created) >= datetime('now','-" + days + " days') " +
  "GROUP BY jid"
);

// ── 2. Build msgMap with device-suffix normalisation ("123:15@lid" → "123@lid") ─
var msgMap = {};
for (var i = 0; i < statsRows.length; i++) {
  var r = statsRows[i];
  if (!r.jid) continue;
  var normJID = r.jid.replace(/:[\d]+@/, '@');
  var cnt = parseInt(r.cnt, 10) || 0;
  msgMap[normJID] = (msgMap[normJID] || 0) + cnt;
  if (normJID !== r.jid) msgMap[r.jid] = msgMap[normJID];
}

// ── 3. LID ↔ PN alias normalisation (same as Go loadLIDMap) ──────────────────
var lidMapRows = wa.db.query(
  "SELECT lid || '@lid' AS lid_jid, pn || '@s.whatsapp.net' AS pn_jid FROM whatsmeow_lid_map"
);
for (var i = 0; i < lidMapRows.length; i++) {
  var lm = lidMapRows[i];
  var lid = lm.lid_jid, pn = lm.pn_jid;
  var hasLID = msgMap[lid] !== undefined;
  var hasPN  = msgMap[pn]  !== undefined;
  if (hasLID && hasPN) {
    var merged = msgMap[lid] + msgMap[pn];
    msgMap[lid] = merged; msgMap[pn] = merged;
  } else if (hasLID) { msgMap[pn] = msgMap[lid];
  } else if (hasPN)  { msgMap[lid] = msgMap[pn]; }
}

// ── 4. Current members from group_membership (last action per member) ─────────
var memberRows = db.query(
  "SELECT m.member_jid " +
  "FROM group_membership m " +
  "INNER JOIN (" +
  "  SELECT member_jid, MAX(created) AS max_created " +
  "  FROM group_membership WHERE group_jid = '" + chatJID + "' " +
  "  GROUP BY member_jid" +
  ") latest ON m.member_jid = latest.member_jid AND m.created = latest.max_created " +
  "WHERE m.group_jid = '" + chatJID + "' AND m.action NOT IN ('leave') " +
  "ORDER BY m.member_jid ASC"
);

// ── 5. Contact names from whatsapp DB ─────────────────────────────────────────
var nameMap = {};
var nameRows = wa.db.query(
  "SELECT their_jid AS jid, " +
  "       COALESCE(NULLIF(push_name,''), NULLIF(full_name,''), their_jid) AS name " +
  "FROM whatsmeow_contacts"
);
for (var i = 0; i < nameRows.length; i++) nameMap[nameRows[i].jid] = nameRows[i].name;

var lidNameRows = wa.db.query(
  "SELECT m.lid || '@lid' AS jid, " +
  "       COALESCE(NULLIF(c.push_name,''), NULLIF(c.full_name,''), m.pn || '@s.whatsapp.net') AS name " +
  "FROM whatsmeow_lid_map m " +
  "LEFT JOIN whatsmeow_contacts c ON c.their_jid = m.pn || '@s.whatsapp.net'"
);
for (var i = 0; i < lidNameRows.length; i++) nameMap[lidNameRows[i].jid] = lidNameRows[i].name;

function getName(jid) {
  if (nameMap[jid]) return nameMap[jid];
  var user = jid.split('@')[0].split(':')[0];
  if (nameMap[user + '@s.whatsapp.net']) return nameMap[user + '@s.whatsapp.net'];
  return user;
}

// ── 6. Build ranked member list, skipping the bot itself ──────────────────────
var members = [];
var seen = {};
for (var i = 0; i < memberRows.length; i++) {
  var mJID = memberRows[i].member_jid;
  if (!mJID || seen[mJID]) continue;
  var mUser = mJID.split('@')[0].split(':')[0];
  if (mUser === botUser) continue; // skip self
  seen[mJID] = true;
  members.push({ jid: mJID, name: getName(mJID), count: msgMap[mJID] || 0 });
}

// Sort descending by message count
members.sort(function(a, b) { return b.count - a.count; });

var totalMembers = members.length;
var silent = 0;
for (var i = 0; i < members.length; i++) { if (members[i].count === 0) silent++; }

// Top 5 most active
var top5 = [];
for (var i = 0; i < members.length && top5.length < 5; i++) {
  if (members[i].count > 0) top5.push(members[i]);
}

// Bottom 5 least active (with at least 1 message, not already in top5)
var active = [];
for (var i = 0; i < members.length; i++) { if (members[i].count > 0) active.push(members[i]); }
var bottom5 = [];
if (active.length > 5) {
  bottom5 = active.slice(active.length - 5).reverse();
}

// ── 7. Format and send ────────────────────────────────────────────────────────
var medals = ['\uD83E\uDD47','\uD83E\uDD48','\uD83E\uDD49','\u0034\uFE0F\u20E3','\u0035\uFE0F\u20E3'];

function fmtMember(i, m) {
  return (medals[i] || (i + 1) + '.') + ' ' + m.name + ' \u2014 ' + m.count + ' msg' + (m.count !== 1 ? 's' : '');
}

var lines = [
  '\uD83D\uDCCA *Ranking do grupo \u2014 últimos ' + days + ' dia' + (days === 1 ? '' : 's') + '*',
  ''
];

lines.push('\uD83D\uDD25 *Mais ativos*');
if (top5.length === 0) {
  lines.push('Nenhuma mensagem no período.');
} else {
  for (var i = 0; i < top5.length; i++) lines.push(fmtMember(i, top5[i]));
}

lines.push('');
lines.push('\uD83D\uDC22 *Menos ativos*');
if (bottom5.length === 0) {
  lines.push('Nenhuma mensagem no período.' + (active.length > 0 && active.length <= 5 ? ' (todos listados acima)' : ''));
} else {
  for (var i = 0; i < bottom5.length; i++) lines.push(fmtMember(i, bottom5[i]));
}

lines.push('');
lines.push('\uD83D\uDE34 *Sem interação:* ' + silent + ' de ' + totalMembers + ' membro' + (totalMembers !== 1 ? 's' : ''));

wa.sendText(chatJID, lines.join('\n'));
`
