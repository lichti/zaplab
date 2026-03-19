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
	script.Set("code", rankingScript)
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

// rankingScript is the JS code executed when /ranking <days> is sent by the bot
// inside a group. It posts top-5 most active, top-5 least active, and silent count.
// Wrapped in an IIFE — top-level return is a JS syntax error in goja.
var rankingScript = `(function() {
// /ranking <dias> — groups only, bot user only
// Usage: /ranking <days>  (default 30)
var info = event && event.Info;
if (!info || !info.IsFromMe || !info.IsGroup) return;

var msg = event.Message || {};
var txt = (msg.conversation || (msg.extendedTextMessage && msg.extendedTextMessage.text) || '').trim();
var parts = txt.split(/\s+/);
if (!parts[0] || parts[0].toLowerCase() !== '/ranking') return;

var days = parseInt(parts[1], 10);
if (isNaN(days) || days < 1) days = 30;
if (days > 365) days = 365;

var chat    = info.Chat;
var botUser = (wa.jid || '').split('@')[0].split(':')[0];

// 1. Message stats
var stats = db.query(
  "SELECT json_extract(raw,'$.Info.Sender') AS jid, COUNT(*) AS cnt FROM events" +
  " WHERE type LIKE '%Message%'" +
  " AND json_extract(raw,'$.Info.Chat')='" + chat + "'" +
  " AND json_extract(raw,'$.Info.IsFromMe')=0" +
  " AND datetime(created)>=datetime('now','-" + days + " days')" +
  " GROUP BY jid");

// 2. msgMap with device-suffix + LID/PN normalisation
var msgMap = {};
for (var i=0;i<stats.length;i++){
  var r=stats[i]; if(!r.jid)continue;
  var k=r.jid.replace(/:[\d]+@/,'@'), c=parseInt(r.cnt,10)||0;
  msgMap[k]=(msgMap[k]||0)+c;
  if(k!==r.jid) msgMap[r.jid]=msgMap[k];
}
var lids=wa.db.query("SELECT lid||'@lid' AS l,pn||'@s.whatsapp.net' AS p FROM whatsmeow_lid_map");
for(var i=0;i<lids.length;i++){
  var l=lids[i].l,p=lids[i].p,hl=msgMap[l]!==undefined,hp=msgMap[p]!==undefined;
  if(hl&&hp){var m2=msgMap[l]+msgMap[p];msgMap[l]=m2;msgMap[p]=m2;}
  else if(hl){msgMap[p]=msgMap[l];}else if(hp){msgMap[l]=msgMap[p];}
}

// 3. Contact names
var names={};
var cn=wa.db.query("SELECT their_jid AS j,COALESCE(NULLIF(push_name,''),NULLIF(full_name,''),their_jid) AS n FROM whatsmeow_contacts");
for(var i=0;i<cn.length;i++) names[cn[i].j]=cn[i].n;
var ln=wa.db.query("SELECT m.lid||'@lid' AS j,COALESCE(NULLIF(c.push_name,''),NULLIF(c.full_name,''),m.pn||'@s.whatsapp.net') AS n FROM whatsmeow_lid_map m LEFT JOIN whatsmeow_contacts c ON c.their_jid=m.pn||'@s.whatsapp.net'");
for(var i=0;i<ln.length;i++) names[ln[i].j]=ln[i].n;
function nm(jid){
  if(names[jid])return names[jid];
  var u=jid.split('@')[0].split(':')[0];
  return names[u+'@s.whatsapp.net']||u;
}

// 4. Members: group_membership first, then wa.getGroups() fallback
var seen={},members=[];
function addMember(jid){
  if(!jid||seen[jid])return;
  var u=jid.split('@')[0].split(':')[0];
  if(u===botUser)return;
  seen[jid]=true;
  members.push({jid:jid,name:nm(jid),count:msgMap[jid]||0});
}

var rows=db.query(
  "SELECT m.member_jid FROM group_membership m"+
  " INNER JOIN(SELECT member_jid,MAX(created) AS mc FROM group_membership WHERE group_jid='"+chat+"' GROUP BY member_jid) lt"+
  " ON m.member_jid=lt.member_jid AND m.created=lt.mc"+
  " WHERE m.group_jid='"+chat+"' AND m.action NOT IN('leave') ORDER BY m.member_jid");
for(var i=0;i<rows.length;i++) addMember(rows[i].member_jid);

// Fallback: live group participants (covers pre-existing groups)
if(members.length===0){
  var groups=wa.getGroups();
  for(var i=0;i<groups.length;i++){
    if(groups[i].jid===chat){
      // wa.getGroups() only returns jid/name/participant_count, not individual JIDs.
      // Use msgMap senders as best-effort member list.
      break;
    }
  }
  // Populate from senders observed in events (best-effort when membership unknown)
  for(var jid in msgMap){
    if(msgMap[jid]>0) addMember(jid);
  }
}

// 5. Sort and slice
members.sort(function(a,b){return b.count-a.count;});
var total=members.length, silent=0;
for(var i=0;i<members.length;i++){if(members[i].count===0)silent++;}

var top5=[],active=[];
for(var i=0;i<members.length;i++){if(members[i].count>0){if(top5.length<5)top5.push(members[i]);active.push(members[i]);}}
var bot5=active.length>5?active.slice(active.length-5).reverse():[];

// 6. Format
var md=['\uD83E\uDD47','\uD83E\uDD48','\uD83E\uDD49','\u0034\uFE0F\u20E3','\u0035\uFE0F\u20E3'];
function row(i,m){return (md[i]||(i+1)+'.')+' '+m.name+' \u2014 '+m.count+' msg'+(m.count!==1?'s':'');}
var L=['\uD83D\uDCCA *Ranking \u2014 últimos '+days+' dia'+(days===1?'':'s')+'*',''];
L.push('\uD83D\uDD25 *Mais ativos*');
if(top5.length===0){L.push('Nenhuma mensagem no período.');}
else{for(var i=0;i<top5.length;i++)L.push(row(i,top5[i]));}
L.push('');
L.push('\uD83D\uDC22 *Menos ativos*');
if(bot5.length===0){L.push(active.length>0?'(todos listados acima)':'Nenhuma mensagem no período.');}
else{for(var i=0;i<bot5.length;i++)L.push(row(i,bot5[i]));}
L.push('');
L.push('\uD83D\uDE34 *Sem interação:* '+silent+' de '+total+' membro'+(total!==1?'s':''));
wa.sendText(chat,L.join('\n'));
})();
`
