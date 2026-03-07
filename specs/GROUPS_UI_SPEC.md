# Groups UI — Plano de Melhoria

Baseado nas evidências (`evidencias/group-list.png`, `evidencias/group-info.png`) e no código atual de `pb_public/js/sections/groups.js` + HTML correspondente.

---

## 1. Bugs Críticos

### 1.1 Layout quebrando no painel de response (ambas as evidências)

**Causa raiz:**
O `<pre class="whitespace-pre">` não quebra linhas. O JSON de grupos contém strings longas (JIDs, URLs, payloads de participantes), fazendo o elemento crescer horizontalmente além do seu flex container. O container direito (`lg:flex-1`) não tem `min-w-0`, portanto ele se expande sem limite em vez de cortar o overflow.

**Arquivos afetados:**
- `pb_public/index.html` — linha `2007–2011` (div `overflow-auto` + `<pre class="whitespace-pre">`)
- `pb_public/js/sections/groups.js` — `groupsResultPreview()` (retorna HTML que alimenta o `<pre>`)

**Fix:**
```html
<!-- antes -->
<div class="overflow-auto min-h-[300px]">
  <pre class="p-4 text-xs leading-relaxed whitespace-pre" x-html="..."></pre>
</div>

<!-- depois -->
<div class="overflow-x-auto min-h-[300px]">
  <pre class="p-4 text-xs leading-relaxed whitespace-pre-wrap break-all" x-html="..."></pre>
</div>
```
E no container direito adicionar `min-w-0`:
```html
<!-- antes -->
<div class="w-full lg:flex-1 flex flex-col rounded-md border overflow-hidden ...">

<!-- depois -->
<div class="w-full lg:flex-1 min-w-0 flex flex-col rounded-md border overflow-hidden ...">
```

> Esta correção se aplica a **todas** as seções que usam `<pre>` com JSON de resposta (send, sendraw, ctrl, contacts — verificar se mesmo padrão).

---

## 2. Componente Compartilhado — Group Picker

**Descrição:** Mini-tabela com os grupos ingressados, carregada uma vez por sessão (cache em memória). Aparece em todas as sub-seções que requerem Group JID (info, participants, settings, leave, invitelink).

**Funcionalidades:**
- Carregado via `GET /groups` na primeira vez que qualquer uma dessas sub-seções for ativada
- Campo de busca filtra por nome ou JID (sem nova requisição)
- Colunas: **Name** | **JID** | **Members** | badges (Announce / Locked)
- Clique na linha → preenche `groups.jid` automaticamente
- Botão `Refresh` para recarregar a lista
- Colapsável: oculto quando `groups.jid` já está preenchido (toggle manual)

**Estado novo em `groups.js`:**
```js
groupsList:          [],   // cache de GET /groups
groupsListLoading:   false,
groupsListFilter:    '',
groupsPickerOpen:    true,
```

**Método novo:**
```js
async loadGroupsList() { /* GET /groups, preenche groupsList */ },
selectGroupFromPicker(jid) { this.groups.jid = jid; this.groupsPickerOpen = false; },
groupsListFiltered() { /* filtra groupsList por groupsListFilter */ },
```

---

## 3. Aba "Table" no painel de Response

Adicionar terceira aba **Table** entre cURL e Response (JSON).

### 3.1 List — tabela de grupos

| Coluna | Fonte |
|--------|-------|
| Name | `group.Name` |
| JID | `group.JID` (truncado, clicável para copiar) |
| Members | `group.Participants.length` |
| Topic | `group.Topic` (truncado, 60 chars) |
| Badges | `Announce`, `Locked` |

Botões de ação rápida por linha: **Info**, **Settings**, **Leave**, **Invite Link** — cada um navega para a sub-seção e preenche o JID.

### 3.2 Info — card + tabela de participantes

**Bloco superior (card):**

| Campo | Fonte |
|-------|-------|
| Name | `result.Name` |
| JID | `result.JID` |
| Owner JID | `result.OwnerJID` |
| Topic | `result.Topic` |
| Created | `result.GroupCreated` (formatado) |
| Announce | badge |
| Locked | badge |

**Tabela de participantes:**

| Coluna | Fonte |
|--------|-------|
| Phone | `p.JID.User` |
| JID | `p.JID` |
| Admin | badge `IsAdmin` |
| Super Admin | badge `IsSuperAdmin` |

### 3.3 Invite Link — display rico

- URL formatada com botão "Copy"
- QR Code gerado pelo endpoint `POST /wa/qrtext` (ver seção 7)
- Instrução "Expira quando o link for resetado"

### 3.4 Participants / Settings / Leave / Create / Join

Fallback para JSON com `whitespace-pre-wrap` — não há tabela estruturada útil para essas operações.

---

## 4. Sub-seção: Info

- **Group Picker** pré-carregado (seção 2)
- **Response**: adicionar aba Table (seção 3.2)

---

## 5. Sub-seção: Participants

### 5.1 Group Picker (seção 2)

### 5.2 Member Picker (para remove / promote / demote)

Quando `groups.jid` está preenchido **e** `participantAction` é `remove`, `promote` ou `demote`:

- Botão **"Load Members"** → chama `GET /groups/{jid}/participants` (endpoint novo, seção 8.1)
- Exibe lista/tabela de membros:

| Coluna | Detalhe |
|--------|---------|
| Initials avatar | 2 letras do número |
| Phone | `p.JID.User` |
| JID | completo |
| Admin badge | se `IsAdmin` |
| Botão `+ Add` | appenda JID no textarea de Participants |

- Ao clicar `+ Add`, o JID entra no textarea `groups.participantsList` (um por linha), e o botão vira `✓ Added` temporariamente
- Contador de selecionados: `"2 selected"`

**Estado novo:**
```js
groupsMembersList:        [],
groupsMembersLoading:     false,
groupsMembersAddedSet:    new Set(),
```

**Método novo:**
```js
async loadGroupMembers() { /* GET /groups/{jid}/participants */ },
toggleGroupMember(jid)   { /* add/remove do textarea */ },
```

---

## 6. Sub-seção: Settings

### 6.1 Group Picker (seção 2)

### 6.2 Botão "Load current settings"

Quando `groups.jid` está preenchido, exibir botão **"Load current settings"** que:
1. Chama `GET /groups/{jid}` (endpoint existente)
2. Preenche os campos do form:
   - `groups.newName` ← `result.Name`
   - `groups.newTopic` ← `result.Topic`
   - `groups.announce` ← `result.Announce` + ativa `groups.setAnnounce = true`
   - `groups.locked` ← `result.Locked` + ativa `groups.setLocked = true`
3. Exibe toast "Settings loaded" em verde

**Método novo:**
```js
async loadCurrentSettings() { /* GET /groups/{jid}, preenche campos */ },
```

---

## 7. Sub-seções: Leave e Invite Link

Apenas adicionar o **Group Picker** (seção 2) — sem outras mudanças nos formulários.

---

## 8. Melhorias Adicionais

### 8.1 Quick Actions direto da tabela (List)

Cada linha da tabela de grupos (aba Table na resposta de List) exibe ícones de ação rápida:

| Ícone | Ação |
|-------|------|
| ℹ️ | Info → define `groups.type = 'info'` + `groups.jid` |
| ⚙️ | Settings → define `groups.type = 'settings'` + `groups.jid` |
| 🚪 | Leave → define `groups.type = 'leave'` + `groups.jid` |
| 🔗 | Invite Link → define `groups.type = 'invitelink'` + `groups.jid` |

Cada ação auto-scroll para o topo do form.

### 8.2 Badges de status na tabela

- `Announce` — badge laranja (`ffa657`) quando `Announce === true`
- `Locked` — badge azul (`58a6ff`) quando `IsLocked === true`
- `Admin` / `Super Admin` — na tabela de participantes

### 8.3 Histórico de JIDs recentes (localStorage)

- Salvar os últimos 5 JIDs usados com sucesso em `localStorage['zaplab-groups-recent-jids']`
- Exibir dropdown no campo Group JID com os recentes
- Cada item mostra o nome do grupo (se disponível no cache)

### 8.4 Exportar grupos como CSV

- Botão **"Export CSV"** visível na aba Table do resultado de List
- Gera arquivo: `groups-{timestamp}.csv`
- Colunas: `Name,JID,Members,Topic,Announce,Locked`

### 8.5 Contador de admins no Group Picker

Na coluna Members do picker: `"12 👤 / 3 👑"` (total / admins)

### 8.6 Auto-refresh do Group Picker após operações

Após `create`, `leave`, `join` bem-sucedidos: limpa `groupsList` e re-chama `loadGroupsList()` automaticamente.

### 8.7 Confirmação antes de Leave

Antes de executar `leave`, exibir modal de confirmação:
> "Tem certeza que quer sair do grupo **{name}**? Esta ação não pode ser desfeita."

Botões: Cancel / Leave Group (vermelho)

---

## 9. Novos Endpoints de Backend

### 9.1 `GET /groups/{jid}/participants`

**Propósito:** Retornar apenas os participantes sem os metadados completos (mais leve que `GET /groups/{jid}` para o Member Picker).

**Response 200:**
```json
{
  "jid": "123456789-000@g.us",
  "participants": [
    {
      "jid":            "5511999999999@s.whatsapp.net",
      "phone":          "5511999999999",
      "is_admin":       true,
      "is_super_admin": false
    }
  ]
}
```

**Response 503** — não conectado
**Response 400** — JID inválido
**Response 500** — erro ao buscar grupo

**Implementação (Go):**
```go
// GET /groups/{jid}/participants
func getGroupParticipants(e *core.RequestEvent) error {
    jidStr := e.Request.PathValue("jid")
    jid, ok := whatsapp.ParseJID(jidStr)
    if !ok {
        return apis.NewBadRequestError("invalid JID", nil)
    }
    info, err := whatsapp.GetGroupInfo(jid)
    if err != nil {
        return e.JSON(http.StatusInternalServerError, map[string]any{"message": err.Error()})
    }
    type participant struct {
        JID          string `json:"jid"`
        Phone        string `json:"phone"`
        IsAdmin      bool   `json:"is_admin"`
        IsSuperAdmin bool   `json:"is_super_admin"`
    }
    out := make([]participant, len(info.Participants))
    for i, p := range info.Participants {
        out[i] = participant{
            JID:          p.JID.String(),
            Phone:        p.JID.User,
            IsAdmin:      p.IsAdmin,
            IsSuperAdmin: p.IsSuperAdmin,
        }
    }
    return e.JSON(http.StatusOK, map[string]any{
        "jid":          info.JID.String(),
        "participants": out,
    })
}
```

### 9.2 `POST /wa/qrtext`

**Propósito:** Gerar QR Code PNG para qualquer string (invite link, texto arbitrário). Reutiliza a lib `rsc.io/qr` já presente.

**Body:**
```json
{ "text": "https://chat.whatsapp.com/AbCdEf123456" }
```

**Response 200:**
```json
{ "image": "data:image/png;base64,..." }
```

**Uso imediato:** aba Table do Invite Link → exibe QR do link de convite.

**Implementação (Go):**
```go
func postQRText(e *core.RequestEvent) error {
    var req struct {
        Text string `json:"text"`
    }
    if err := e.BindBody(&req); err != nil || req.Text == "" {
        return apis.NewBadRequestError("text is required", nil)
    }
    qrCode, err := qr.Encode(req.Text, qr.L)
    if err != nil {
        return e.JSON(http.StatusInternalServerError, map[string]any{"message": "failed to encode QR"})
    }
    b64 := base64.StdEncoding.EncodeToString(qrCode.PNG())
    return e.JSON(http.StatusOK, map[string]any{"image": "data:image/png;base64," + b64})
}
```

---

## 10. Ordem de Implementação

| Prioridade | Item | Esforço | Impacto |
|-----------|------|---------|---------|
| P0 | Bug fix: `whitespace-pre-wrap` + `min-w-0` | Mínimo | Imediato |
| P1 | Group Picker (componente compartilhado) | Médio | Desbloqueia P2–P6 |
| P2 | Settings → Load current settings | Pequeno | Alto (UX diário) |
| P2 | Participants → Member Picker | Médio | Alto |
| P3 | Backend: `GET /groups/{jid}/participants` | Pequeno | Necessário para P2 |
| P3 | Response Table: List + Info | Médio | Alto |
| P4 | Quick Actions da tabela | Pequeno | Médio |
| P4 | Backend: `POST /wa/qrtext` | Pequeno | Médio |
| P4 | Invite Link → Table com QR | Pequeno | Médio |
| P5 | Confirmação antes de Leave | Pequeno | Segurança |
| P5 | Histórico de JIDs recentes | Pequeno | Conforto |
| P5 | Export CSV | Pequeno | Conforto |
| P6 | Auto-refresh após create/leave/join | Pequeno | Polimento |
| P6 | Badges + contagem admins no picker | Mínimo | Polimento |

---

## 11. Resumo das Mudanças por Arquivo

| Arquivo | Mudança |
|---------|---------|
| `pb_public/index.html` | Corrigir `whitespace-pre` → `whitespace-pre-wrap`; adicionar `min-w-0`; adicionar HTML do Group Picker, Member Picker, abas Table, confirmação de Leave, Quick Actions |
| `pb_public/js/sections/groups.js` | Novos estados (`groupsList`, `groupsMembersList`, etc.); novos métodos (`loadGroupsList`, `loadGroupMembers`, `loadCurrentSettings`, `selectGroupFromPicker`, `groupsListFiltered`, exportCSV) |
| `internal/api/api.go` | Registrar `GET /groups/{jid}/participants` e `POST /wa/qrtext` |
| `internal/whatsapp/groups.go` | Sem mudanças (já existe `GetGroupInfo` que resolve o participants endpoint) |
| `specs/API_SPEC.md` | Documentar os 2 novos endpoints |
| `README.md` / `README.pt-BR.md` | Seção API com novos endpoints |
