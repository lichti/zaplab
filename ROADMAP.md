# ZapLab — Roadmap v1.0.0-beta.8+

> Documento de planejamento interno. Atualizado em 2026-03-17.
> Estado atual: **v1.0.0-beta.7** com Scripting Engine, Network Graph,
> App State Inspector, Session Comparator, Frame Capture, PCAP Export,
> Proto Schema Browser, DB Explorer, Annotations e Stats Heatmap já entregues.

---

## Resumo Executivo

O ZapLab está maduro como ferramenta de pesquisa de protocolo. O próximo grande release
deve expandir em três eixos:

1. **Scripting mais poderoso** — tornar o sandbox o centro da automação e pesquisa
2. **Inteligência sobre dados armazenados** — extrair mais valor dos eventos persistidos
3. **Novas superfícies de protocolo** — áreas do protocolo WhatsApp ainda não visualizadas

---

## EIXO 1 — Scripting Engine (Expansão)

### 1.1 Mais bindings no sandbox `wa.*`

**O que é:** Expor no sandbox JavaScript os mesmos recursos que a API REST já tem.
Atualmente `wa` só tem `sendText` e `status`. Todo o resto (imagens, reações, edição,
grupos, etc.) está no backend mas inacessível via script.

**Bindings propostos:**

| Binding | Implementação backend |
|---------|----------------------|
| `wa.sendImage(jid, base64, caption)` | `whatsapp.SendImage` |
| `wa.sendAudio(jid, base64)` | `whatsapp.SendAudio` |
| `wa.sendDocument(jid, base64, filename)` | `whatsapp.SendDocument` |
| `wa.sendLocation(jid, lat, lon, name)` | `whatsapp.SendLocation` |
| `wa.sendReaction(chat, sender, msgID, emoji)` | `whatsapp.SendReaction` |
| `wa.editMessage(chat, msgID, newText)` | `whatsapp.EditMessage` |
| `wa.revokeMessage(chat, sender, msgID)` | `whatsapp.RevokeMessage` |
| `wa.setTyping(jid, state)` | `whatsapp.SetTyping` |
| `wa.getContacts()` | query em `whatsmeow_contacts` via waDB |
| `wa.getGroups()` | `client.GetJoinedGroups()` |
| `wa.getGroupInfo(jid)` | `client.GetGroupInfo()` |
| `wa.createPoll(jid, question, options)` | `whatsapp.CreatePoll` |
| `wa.jid` | JID do dispositivo conectado (string) |

**Complexidade:** Baixa — wrappers simples sobre funções já existentes.
**Impacto:** Alto — o sandbox passa de "enviar texto" para automação completa.

---

### 1.2 Triggers de evento (Event Hooks)

**O que é:** Executar automaticamente um script quando um evento do WhatsApp chega,
com filtros configuráveis por tipo, JID, pattern de texto.

**Modelo proposto:**

```
Collection: script_triggers
Fields: script_id, event_type (e.g. "Message"), jid_filter, text_pattern, enabled
```

Quando um evento é persistido em `saveEvent()`, o engine verifica os triggers ativos
e executa os scripts correspondentes em goroutines. O contexto do evento é injetado
no script como variável global `event`:

```js
// Trigger: event_type = "Message", text_pattern contains "urgente"
console.log("Mensagem urgente de:", event.Info.Sender);
wa.sendText(event.Info.Chat, "Recebi sua mensagem urgente, aguarde.");
```

**Complexidade:** Média — requer nova collection, pipeline de dispatch em eventos,
e uma nova tab "Triggers" na seção Scripting.
**Impacto:** Muito alto — transforma o ZapLab em plataforma de automação real.

---

### 1.3 Agendamento de scripts (Cron)

**O que é:** Executar scripts em intervalos configuráveis, sem precisar de trigger externo.

**Modelo proposto:**

```
Collection: script_schedules
Fields: script_id, cron_expr (string), enabled, last_run, next_run
```

Implementado com `time.AfterFunc` ou loop goroutine. Expressões cron padrão
(`0 * * * *`, `*/5 * * * *`, etc.).

**UI:** Nova coluna "Schedule" no painel de scripts com badge indicando próxima execução.

**Complexidade:** Média.
**Impacto:** Alto — permite automações periódicas (relatórios, health checks, alertas).

---

### 1.4 `wa.db` — acesso ao banco do whatsmeow no sandbox

**O que é:** Expor acesso somente-leitura ao banco `whatsapp.db` (whatsmeow SQLite)
via `wa.db.query(sql)`, separado do `db.query` que acessa o PocketBase.

```js
// Listar contatos do whatsmeow
const contacts = wa.db.query("SELECT jid, pushname FROM whatsmeow_contacts LIMIT 20");
contacts.forEach(c => console.log(c.jid, c.pushname));
```

**Complexidade:** Baixa — `waDB` já está disponível globalmente em `api`.
**Impacto:** Alto — desbloqueia acesso a dados de protocolo de baixo nível nos scripts.

---

### 1.5 Import/Export de scripts

**O que é:** Exportar todos os scripts como bundle JSON e reimportá-los em outra
instância. Permite compartilhar "script packs" entre pesquisadores.

**Formato:**
```json
{
  "version": 1,
  "exported_at": "2026-03-17T...",
  "scripts": [{ "name": "...", "description": "...", "code": "...", "timeout_secs": 10 }]
}
```

**Complexidade:** Baixa.
**Impacto:** Médio — importante para comunidade e portabilidade.

---

## EIXO 2 — Inteligência sobre Dados Armazenados

### 2.1 Full-text search sobre mensagens

**O que é:** Busca textual nos conteúdos de mensagens armazenadas em `events.raw`.
A coluna `raw` é JSON; SQLite suporta `json_extract` e `LIKE`.

**API proposta:** `GET /zaplab/api/search?q=...&type=Message&limit=50`

Busca em:
- `json_extract(raw, '$.Message.Conversation')` — texto simples
- `json_extract(raw, '$.Message.ExtendedTextMessage.Text')` — texto com preview
- `json_extract(raw, '$.Message.ImageMessage.Caption')` — legenda de imagem
- `msgID` exato

**UI:** Nova seção "Search" na sidebar, ou campo de busca global no header.

**Complexidade:** Baixa-Média.
**Impacto:** Alto — recurso fundamental que ainda falta.

---

### 2.2 Receipt Latency Tracker

**O que é:** Medir e visualizar o tempo entre envio de mensagem e os receipts
(delivered / read). O WhatsApp emite eventos `Receipt` com `Type: "delivery"` e
`Type: "read"`.

**Como funciona:** Cruzar eventos `Message` com `Receipt` pelo `msgID` e calcular delta.

**API:** `GET /zaplab/api/stats/receipts?days=7`

**UI (na seção Stats):**
- Histograma de latência de entrega (ms / segundos)
- Percentis p50, p90, p99
- Scatter plot: mensagem × tempo de leitura

**Complexidade:** Média.
**Impacto:** Alto — insight único sobre comportamento do protocolo.

---

### 2.3 Presence Timeline

**O que é:** Visualizar a linha do tempo de presença (`online`/`offline`/`typing`/`recording`)
de contatos ao longo do tempo. Já armazenamos eventos `Presence` em `events`.

**API:** `GET /zaplab/api/presence/timeline?jid=...&days=7`

**UI:** Gráfico de gantt horizontal por JID — barras coloridas indicando períodos online,
typing, recording.

**Complexidade:** Média.
**Impacto:** Alto — visualização única, muito útil para análise de comportamento.

---

### 2.4 Media Gallery

**O que é:** Browser visual de todas as mídias recebidas/enviadas (imagens, vídeos,
áudios, documentos) com preview inline e download. Os eventos `ImageMessage`,
`VideoMessage`, `AudioMessage` já são persistidos com o file attachment.

**API:** `GET /zaplab/api/media/gallery?type=image&chat=...&limit=50`

**UI:** Grid de thumbnails com lightbox; filtros por tipo, chat, data; download direto.

**Complexidade:** Média.
**Impacto:** Alto — muito pedido, prático para análise de conteúdo.

---

### 2.5 Conversation View

**O que é:** Visualizar mensagens de um chat específico em ordem cronológica, com
layout similar ao WhatsApp (bolhas direita/esquerda, timestamps, receipts).
Dados já existem em `events WHERE type='Message'` com `json_extract`.

**API:** `GET /zaplab/api/conversation?chat=...&limit=100&before=<timestamp>`

**UI:** Layout de chat com:
- Bolhas de mensagem (esquerda = recebido, direita = enviado)
- Avatars / nomes dos remetentes
- Indicadores de receipt (✓✓ lido)
- Preview de mídia inline (se disponível)
- Clique na mensagem → abre evento bruto no Event Browser

**Complexidade:** Média.
**Impacto:** Muito alto — transforma dados brutos em contexto legível.

---

### 2.6 Export de dados

**O que é:** Exportar seções do banco para análise externa.

| Formato | Conteúdo |
|---------|---------|
| CSV | Mensagens com metadados (chat, sender, timestamp, tipo) |
| JSON | Eventos brutos filtrados por tipo/período |
| HAR | Frame log no formato HTTP Archive (compatível com DevTools) |
| PCAP | Já existe — ampliar para incluir filtros de período/módulo |
| Markdown | Relatório de sessão: estatísticas, anomalias, anotações |

**Complexidade:** Baixa-Média por formato.
**Impacto:** Alto — essencial para pesquisadores que usam ferramentas externas.

---

## EIXO 3 — Novas Superfícies de Protocolo

### 3.1 Binary Node Inspector

**O que é:** O WhatsApp codifica todas as mensagens no protocolo binário WABinary
antes de criptografar com Noise. O `raw` armazenado já é o proto Go-decoded,
mas o WABinary intermediate ainda não é inspecionável.

O whatsmeow loga `waBinary.Node` quando `DATABASE` logging está ativo. Esses logs
já são capturados no `frames` collection — mas não são decodificados como árvore.

**Proposta:**
- Parser de WABinary no frontend (ou API que retorna árvore JSON)
- Viewer hierárquico com tag → nome usando o token dictionary do whatsmeow
- Highlight de campos desconhecidos (tag sem mapeamento no dicionário)

**Complexidade:** Alta.
**Impacto:** Muito alto para pesquisadores de protocolo.

---

### 3.2 IQ Node Analyzer

**O que é:** Nós IQ (Info/Query) são o mecanismo de request/response do protocolo
WhatsApp (similar ao XMPP IQ). Respondem por: registro, sincronização de chaves,
push de contatos, configurações, etc.

No frame log, IQ nodes aparecem como entradas do módulo `Client`. Ainda não são
categorizados ou visualizados de forma estruturada.

**Proposta:**
- Filtrar frames do tipo `iq` no frame log
- Parsear e categorizar: `get`, `set`, `result`, `error`
- Mostrar namespace (ex: `w:sync:app:state`, `w:web`, `encrypt`)
- Timeline de IQ round-trips com latência

**Complexidade:** Média-Alta.
**Impacto:** Alto — IQ nodes são o protocolo de controle do WhatsApp.

---

### 3.3 Pre-key Health Monitor

**O que é:** O Signal Protocol requer que o dispositivo mantenha um pool de pre-keys
no servidor. Quando depleta, novos remetentes não conseguem iniciar sessão.
O whatsmeow gerencia isso automaticamente, mas não há visibilidade sobre o estado.

**Proposta:**
- `GET /zaplab/api/prekeys/status` — conta quantas pre-keys estão marcadas como
  não-utilizadas em `whatsmeow_pre_keys`, e quantas foram consumidas
- Dashboard widget mostrando: `Disponíveis: N`, `Usadas: M`, `Taxa de consumo: X/dia`
- Alerta visual quando disponíveis < threshold configurável
- Botão "Refresh pre-keys" (força upload de novo lote para o servidor)

**Complexidade:** Baixa-Média.
**Impacto:** Médio — importante para sessões de longa duração.

---

### 3.4 Connection Stability Dashboard

**O que é:** Visualização dedicada ao ciclo de vida da conexão WebSocket/Noise —
reconexões, duração de sessões, erros de handshake, keepalives.

Dados já existem em:
- `events` → `type = 'Connected'`, `'Disconnected'`, `'LoggedOut'`, `'StreamReplaced'`
- `errors` → eventos de reconexão
- `frames` → `module = 'Socket'`, `level = 'WARN'/'ERROR'`

**Proposta:**
- Timeline de uptime/downtime (gantt horizontal)
- Contagem de reconexões por período
- Duração média de sessão contínua
- Causas de desconexão mais frequentes
- Tabela de erros de handshake com stack

**Complexidade:** Baixa.
**Impacto:** Médio-Alto — útil para instâncias que precisam de estabilidade.

---

### 3.5 Group Membership Tracker

**O que é:** Rastrear mudanças de participação em grupos ao longo do tempo.
O WhatsApp emite eventos `GroupInfo` quando alguém entra, sai, é adicionado
ou promovido a admin. Esses eventos já são armazenados.

**Proposta:**
- `GET /zaplab/api/groups/{jid}/history` — timeline de eventos de membership
- UI: linha do tempo por grupo mostrando adições, remoções, mudanças de admin
- Comparar "grupo agora" vs "grupo há N dias"

**Complexidade:** Média.
**Impacto:** Médio — relevante para pesquisa de dinâmica de grupos.

---

### 3.6 Message Secret Inspector

**O que é:** A tabela `whatsmeow_message_secrets` armazena chaves simétricas por
mensagem usadas para decriptação de mídia e mensagens efêmeras (view-once).
Atualmente visível via DB Explorer mas sem semântica.

**Proposta:**
- `GET /zaplab/api/secrets?chat=...&msgID=...` — retorna secret com metadados
- UI na seção DB Explorer: coluna "Decrypt" que tenta usar o secret para
  descriptografar o payload de mídia inline
- Mostrar: `key_id`, `key (hex)`, `msg_type`, `timestamp`, se já foi usado

**Complexidade:** Média.
**Impacto:** Alto para pesquisadores de criptografia.

---

## EIXO 4 — UX e Infraestrutura

### 4.1 Notificações em tempo real (Server-Sent Events / WebSocket)

**O que é:** Push de eventos para o frontend sem polling. Atualmente o dashboard
faz refresh a cada 60s. Com SSE/WebSocket, eventos aparecem instantaneamente.

**Proposta:**
- `GET /zaplab/api/events/stream` — SSE endpoint que emite eventos em tempo real
- Frontend: badge de notificação na sidebar quando um novo evento chega na seção inativa
- Dashboard: contador atualiza em tempo real sem refresh manual

**Complexidade:** Média.
**Impacto:** Alto — melhora dramaticamente a experiência de monitoramento.

---

### 4.2 Global Search

**O que é:** Campo de busca global no header que pesquisa simultaneamente em:
eventos (por tipo, msgID, remetente), scripts (por nome), anotações (por texto),
frames (por módulo + mensagem).

**UI:** `Ctrl+K` abre um modal de busca estilo command palette; resultados agrupados
por seção; clique navega diretamente para o item.

**Complexidade:** Média.
**Impacto:** Alto — especialmente com o volume de dados que o ZapLab acumula.

---

### 4.3 Atalhos de teclado

**O que é:** Navegação completa via teclado para fluxos frequentes.

| Atalho | Ação |
|--------|------|
| `Ctrl+K` | Global search |
| `G D` | Ir para Dashboard |
| `G E` | Ir para Event Browser |
| `G S` | Ir para Scripting |
| `G M` | Ir para Message History |
| `R` | Refresh seção atual |
| `Esc` | Fechar modal/painel |
| `Ctrl+Enter` | Executar (script, envio de mensagem, etc.) |

**Complexidade:** Baixa.
**Impacto:** Médio — qualidade de vida para uso intenso.

---

### 4.4 Multi-instância / Multi-device

**O que é:** Suporte a múltiplas sessões WhatsApp simultaneamente. Cada sessão
seria um "dispositivo" independente com seu próprio `whatsapp.db`, pool de eventos
e estado de conexão.

**Modelo proposto:**
```
Collection: devices
Fields: name, description, db_path, status, jid
```

Cada request carregaria o contexto do dispositivo ativo (selecionado via dropdown
no header). Scripts, webhooks e eventos seriam segregados por device.

**Complexidade:** Muito Alta — requer refactoring profundo da camada de estado global.
**Impacto:** Muito Alto — habilita pesquisa comparativa entre contas.

---

### 4.5 Audit Log

**O que é:** Registrar todas as ações administrativas realizadas via UI ou API:
quem executou qual script, quais mensagens foram enviadas, quais configurações
foram alteradas, quais dados foram exportados.

```
Collection: audit_log
Fields: user_id, action, target_type, target_id, payload (JSON), ip, created
```

**Complexidade:** Baixa — middleware que intercepta mutações e grava.
**Impacto:** Médio — importante para uso em equipe ou conformidade.

---

### 4.6 Theme e personalização

**O que é:** Além do dark/light mode já existente:
- Selecionar fonte mono do editor (JetBrains Mono, Fira Code, etc.) via CDN
- Ajustar tamanho da fonte base
- Escolher cor de destaque (accent color) para a sidebar/badges
- Compactar sidebar: modo ultra-compact com apenas ícones (sem labels mesmo expandida)

**Complexidade:** Baixa.
**Impacto:** Baixo-Médio — qualidade de vida.

---

## Tabela de Priorização

| # | Feature | Eixo | Impacto | Complexidade | Prioridade |
|---|---------|------|---------|-------------|-----------|
| 1 | Conversation View | 2 | Muito Alto | Média | **★★★★★** |
| 2 | Script Triggers (Event Hooks) | 1 | Muito Alto | Média | **★★★★★** |
| 3 | Full-text search de mensagens | 2 | Alto | Baixa | **★★★★★** |
| 4 | `wa.*` bindings expandidos | 1 | Alto | Baixa | **★★★★★** |
| 5 | SSE / Notificações real-time | 4 | Alto | Média | **★★★★☆** |
| 6 | Media Gallery | 2 | Alto | Média | **★★★★☆** |
| 7 | Receipt Latency Tracker | 2 | Alto | Média | **★★★★☆** |
| 8 | `wa.db` no sandbox | 1 | Alto | Baixa | **★★★★☆** |
| 9 | Cron scheduling de scripts | 1 | Alto | Média | **★★★★☆** |
| 10 | Export CSV/JSON/HAR | 2 | Alto | Baixa | **★★★★☆** |
| 11 | Presence Timeline | 2 | Alto | Média | **★★★☆☆** |
| 12 | Connection Stability Dashboard | 3 | Médio-Alto | Baixa | **★★★☆☆** |
| 13 | Global Search (`Ctrl+K`) | 4 | Alto | Média | **★★★☆☆** |
| 14 | Import/Export de scripts | 1 | Médio | Baixa | **★★★☆☆** |
| 15 | Pre-key Health Monitor | 3 | Médio | Baixa | **★★★☆☆** |
| 16 | Atalhos de teclado | 4 | Médio | Baixa | **★★★☆☆** |
| 17 | IQ Node Analyzer | 3 | Alto | Média-Alta | **★★★☆☆** |
| 18 | Binary Node Inspector | 3 | Muito Alto | Alta | **★★☆☆☆** |
| 19 | Group Membership Tracker | 3 | Médio | Média | **★★☆☆☆** |
| 20 | Message Secret Inspector | 3 | Alto | Média | **★★☆☆☆** |
| 21 | Audit Log | 4 | Médio | Baixa | **★★☆☆☆** |
| 22 | Multi-instância | 4 | Muito Alto | Muito Alta | **★☆☆☆☆** |
| 23 | Theme/personalização | 4 | Baixo | Baixa | **★☆☆☆☆** |

---

## Sugestão de Escopo para beta.8

Para um release significativo sem over-engineering, sugerimos um conjunto coeso:

### Opção A — "Automação & Busca" (foco em scripting + usabilidade)
- `wa.*` bindings expandidos
- `wa.db` no sandbox
- Import/Export de scripts
- Script Triggers (Event Hooks)
- Full-text search de mensagens
- Export CSV/JSON

### Opção B — "Dados & Visualização" (foco em análise de dados armazenados)
- Conversation View
- Media Gallery
- Full-text search de mensagens
- Receipt Latency Tracker
- Presence Timeline
- Export CSV/JSON/HAR

### Opção C — "Protocolo Profundo" (foco em pesquisa de protocolo)
- Connection Stability Dashboard
- Pre-key Health Monitor
- IQ Node Analyzer
- Message Secret Inspector
- Group Membership Tracker
- `wa.db` no sandbox

---

*Última atualização: 2026-03-17*
