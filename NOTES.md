# Notas

- Melhoria na tela de conversation
  - Updates sem precisar ficar clicando em refresh

- Frontend
  - Faz sentido usar algum framework como react, angular, vue ou astro? Qual?
  - O Front esta ficando pesado (processamento e memória), o que pode ser melhorado?

- Contact Overview Dashboard 
  - Esta muito pobre de informação
  - exibir foto
  - exibir nome
  - exibir status
  - trazer dados de presença
  - trazer mais informações de forma visual
  - o que mais for interessante para um dasboard rico

- Group Overview Dashboard
  - Esta muito pobre de informação
  - exibir foto
  - exibir nome
  - lista de admin
  - lista de membros
  - top 5 usuarios mais ativos
  - top 5 usuários menos ativos
  - lista de usuários que não tiveram interações
  - o que mais for interessante para um dasboard rico

  12:04:46.824 [Client WARN] Node handling took 7.470361779s for <message addressing_mode="lid" from="5511983677522-1407133922@g.us" id="3A1C77EAAB2A74753D71" notify="Arthur Capella" participant="253008000618611@lid" participant_pn="5521991441267@s.whatsapp.net" t="1773846278" type="text">
  <enc type="pkmsg" v="2"><!-- 311 bytes --></enc>
  <reporting>
    <reporting_tag>011104afdce303380bfa8bc6ec739e6e0882f451</reporting_tag>
    <reporting_token v="2">e5cc1fb69399559abebaf710d674c033</reporting_token>
  </reporting>
  <enc type="skmsg" v="2"><!-- 476 bytes --></enc>
</message>
12:04:49.294 [Main INFO] Received message id=3A8F16803CF31690321F from=253008000618611@lid in 5511983677522-1407133922@g.us meta=[pushname: Arthur Capella timestamp: 2026-03-18 12:04:48 -0300 -03 type: reaction]
12:05:00.568 [Main INFO] Received message id=3BA8C5CDDD7D649FDA95 from=236717323882740:26@lid in 236717323882740@lid meta=[pushname: Camilla Lichti timestamp: 2026-03-18 12:04:57 -0300 -03 type: media]
12:05:53.300 [Client WARN] Node handling is taking long for <message addressing_mode="lid" from="5511983677522-1407133922@g.us" id="3A39084C134F9AECCEAD" notify="~weber" participant="35304782229633@lid" participant_pn="5511981580100@s.whatsapp.net" t="1773846322" type="reaction"><enc decrypt-fail="hide" type="skmsg" v="2"><!-- 187 bytes --></enc></message> (started 30.121570177s ago)
12:06:23.212 [Client WARN] Node handling is taking long for <message addressing_mode="lid" from="5511983677522-1407133922@g.us" id="3A39084C134F9AECCEAD" notify="~weber" participant="35304782229633@lid" participant_pn="5511981580100@s.whatsapp.net" t="1773846322" type="reaction"><enc decrypt-fail="hide" type="skmsg" v="2"><!-- 187 bytes --></enc></message> (started 1m0.032679074s ago)




49344963182619@lid e 5511971008732@s.whatsapp.net é o mesmo contato e esta presente em dois grupos:
- 5511971008732@s.whatsapp.net -> 120363425621295665@g.us
- 49344963182619@lid -> 120363426369019996@g.us 

Em contact-overview, só é listado o grupo 120363425621295665@g.us

se necessário olhe para o DATA_DIR=$HOME/.zaplab-pessoal/






Group Overview
- Members precisa mostrar todos os membros e não apenas active members. Permitir ordenação por Nome e Messages
- Silent esta mostrando usuarios que que interagiram com no grupo apenas com o jid ou apenas com o lid, precisa considerar a unificaçào
se necessário olhe para o DATA_DIR=$HOME/.zaplab-pessoal/


Criar um script e uma triger para esse script.
Condições:
  - Só funciona em grupos
  - Só o usuário do bot pode acionar
Trigger: /ranking <dias>
Script deve postar os top 5 usuários com mais atividades, top 5 com menos atividades e quantidade de membros que não interagiram na quantidade de dias passadas no parametro.
- Se o usuário que passou a informação não for o bot, ignorar.


O Filtro 7 days, 30 days, 90 days e 1 year do /stats nao esta funcionando
