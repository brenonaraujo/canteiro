> Histórico do bootstrap (`gmh seed`). Movido de `.github/ISSUE_0.md`
> para não competir com os issue forms YAML. Spec de produto: `docs/SPEC.md`.

## Issue 0 — specification that starts the loop

This issue is the **principal** of the first delivery cycle.
The host agent that created the repo MUST NOT implement the product.
`team-manager` (Hermes cron pooling) owns the flow.

### Project
- Name: canteiro
- Domain: `domain/canteiro`
- GitHub: brenonaraujo/canteiro

### Spec (source of truth: `docs/SPEC.md`)

# Canteiro — SPEC funcional

Produto público: **https://canteiro.brenon.cloud**  
Repositório: `brenonaraujo/canteiro`  
Documento técnico (modelagem, API, stack): a definir em `TECHNICAL_SPEC.md` / `ARCHITECTURE.md` depois desta SPEC.

Canteiro é um marketplace de **aluguel de ferramentas e máquinas entre pessoas**. Proprietários publicam desde equipamento básico (furadeira, andaime, compactador) até maquinário pesado (trator, retroescavadeira, guindaste). Locatários reservam, pagam, avaliam. A plataforma **orquestra o pagamento**, **segura caução**, **cobra o locatário em caso de quebra**, **repassa o proprietário** e **fica com uma parte da negociação**. Um anúncio pode incluir **operador cobrado por hora**. Cadastro com **Google**.

## 1. Visão e objetivos

- Dar a quem tem ferramenta ou máquina ociosa um canal para alugar com pagamento e caução confiáveis.
- Dar a quem precisa (obra, fazenda, evento, manutenção) acesso pontual a equipamento — com ou sem operador.
- Proteger as duas partes: caução, evidência de estado, avaliação mútua, cobrança de avaria.
- Sustentar a operação com comissão da plataforma sobre o valor da locação (e da hora do operador, quando houver).

Não é loja de venda. Não é classificados sem pagamento. Não é locadora B2B clássica com frota própria.

## 2. Personas

| Persona | Quem é | Objetivo |
|---|---|---|
| **Proprietário** | Pessoa ou MEI/empresa com equipamento próprio | Publicar, precificar, receber, proteger o bem |
| **Locatário** | Quem precisa do equipamento por um período | Encontrar, reservar, pagar, devolver, avaliar |
| **Operador** | Profissional que opera a máquina | Ser oferecido junto ao anúncio, receber as horas |
| **Staff da plataforma** | Operação Canteiro | Mediação de avaria, suspensão de conta, ajuste de comissão |

Uma conta Google pode ser proprietário e locatário. Operador pode ser o próprio proprietário ou um terceiro indicado no anúncio.

## 3. Glossário

- **Anúncio:** oferta de um bem para aluguel, com preço, disponibilidade, localização e regras.
- **Bem / equipamento:** item anunciado (ferramenta, máquina, acessório).
- **Reserva:** pedido de aluguel para um intervalo, ainda não pago.
- **Locação:** reserva aceita e paga (ou com pagamento autorizado).
- **Caução:** valor retido do locatário para cobrir avaria, atraso ou item faltante.
- **Operador:** pessoa cobrada por hora, opcional no anúncio, para ir junto com a máquina.
- **Comissão:** percentual da plataforma sobre o valor da locação e das horas de operador.
- **Avaria:** dano, perda ou uso fora do combinado, apurado na devolução.
- **Período:** intervalo combinado de retirada e devolução (hora/dia, conforme o anúncio).

## 4. Escopo funcional (v1)

### 4.1 Conta e acesso

- Qualquer pessoa entra com **Google**. Sem Google, não há conta no v1.
- Após o primeiro login, completa nome visível e telefone.
- Pode atuar como locatário imediatamente.
- Para publicar anúncio, confirma dados de recebimento (conta de pagamento) e aceita os termos de proprietário.
- Logout encerra a sessão. Conta pode ser desativada pelo próprio usuário (anúncios saem do ar; locações em curso não são canceladas automaticamente).

### 4.2 Publicar equipamento

O proprietário cria um anúncio com:

- Título, descrição, categoria (manual, elétrico, obra leve, agrícola, pesado — incl. trator e guindaste).
- Fotos (obrigatório pelo menos uma).
- Local de retirada (cidade, bairro ou ponto; área de cobertura se houver entrega).
- Preço do período (por hora e/ou por dia — o anúncio declara a unidade).
- Caução sugerida (obrigatória; o proprietário define o valor, a plataforma valida mínimo por categoria).
- Disponibilidade (calendário: bloqueios e antecedência mínima).
- Regras (documento exigido, idade mínima, experiência, restrição de deslocamento).
- **Operador opcional:** sim/não; se sim, valor hora, se o operad

…(truncated; full text in docs/SPEC.md)

### Acceptance (this issue)
- [ ] `docs/SPEC.md` committed
- [ ] Personas materialized (`gmh agents list` shows 7 profiles, specialized domain-expert)
- [ ] `.github/workflows/ci.yml` and `release.yml` present
- [ ] Canonical labels exist
- [ ] `gmh loop doctor` exits 0
- [ ] Hermes cron `canteiro-loop` (no_agent, no monitor) and `canteiro-orchestrator` installed
- [ ] team-manager tick does **not** write feature code
- [ ] First type/feature sub-issue opened from the spec (not implemented in the parent chat)

### Routing
`type/feature` → domain-expert-canteiro → solutions-architect → builders → qa → human → merge.

### Guardrail
If a worker process dies, **fix spawn/profile**. Do not implement in the orchestrator session.

