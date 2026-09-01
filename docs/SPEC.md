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
- **Operador opcional:** sim/não; se sim, valor hora, se o operador é o dono ou um nome indicado, e se o operador é obrigatório para aquela máquina.

Anúncio nasce como rascunho, depois **publicado** (visível na busca) ou **pausado**. Pesado (trator, guindaste e equivalentes) exige declaração de que o proprietário pode legalmente ceder o bem e, se operador for obrigatório, que o operador está identificado.

### 4.3 Descoberta

- Visitante (sem login) vê busca pública, filtros e ficha do anúncio (preço, fotos, localização aproximada, avaliações).
- Filtros: categoria, cidade, intervalo de datas, com/sem operador, faixa de preço, porte (leve vs pesado).
- Ficha mostra indisponibilidade no calendário sem revelar identidade de outros locatários.

### 4.4 Reserva e locação

1. Locatário autenticado escolhe intervalo e se quer o operador (quando opcional).
2. Sistema calcula: aluguel + horas de operador (se houver) + caução + comissão embutida no fluxo de pagamento (o locatário vê o total a pagar; o proprietário vê o líquido).
3. Locatário paga (ou autoriza) via **Stripe**.
4. Proprietário aceita ou recusa em prazo visível (padrão 12h). Sem resposta: reserva expira, pagamento estornado, caução liberada.
5. Aceite confirma a locação. Calendário bloqueia o intervalo. As duas partes veem dados de contato e instruções de retirada.

Cancelamento:

- Locatário cancela antes do aceite: estorno integral.
- Locatário cancela após aceite, com antecedência ≥ 24h: estorno do aluguel menos taxa de cancelamento (10% do aluguel, mínimo simbólico definido na ficha); caução liberada; operador não é pago.
- Menos de 24h: sem estorno do aluguel; caução liberada se o bem não saiu.
- Proprietário cancela após aceite: estorno integral ao locatário; penalidade de reputação no proprietário.

### 4.5 Pagamento, comissão e caução (Stripe)

- A plataforma **orquestra** o dinheiro. Proprietário não recebe fora da plataforma no v1.
- **Comissão da plataforma: 12%** sobre (aluguel do bem + horas de operador). Caução não entra na comissão.
- Caução é **autorização/retida** no momento do pagamento e só é capturada no todo ou em parte se houver avaria, atraso ou item não devolvido.
- Após devolução sem contestação: caução liberada; líquido do proprietário e do operador (se terceiro) é liberado menos comissão.
- Operador terceiro, quando distinto do proprietário, recebe as horas; o proprietário recebe só o aluguel do bem. Se o operador é o dono, o líquido soma aluguel + horas, menos comissão.
- Falha de pagamento = locação não existe.
- Recibo/comprovante visível para as duas partes.

### 4.6 Retirada, uso e devolução

- Na retirada, locatário e proprietário registram **estado de saída** (fotos + checklist). Sem esse registro, a plataforma assume o estado das fotos do anúncio.
- Na devolução, registram **estado de volta**. Atraso além da tolerância do anúncio (padrão 1 hora) gera cobrança proporcional da unidade de preço + possível uso da caução se o atraso bloquear outra reserva.
- Locação passa a “aguardando avaliação” depois da devolução confirmada pelas duas partes, ou após 48h da hora combinada de volta se uma parte não confirmar (a outra pode abrir contestação).

### 4.7 Avaria e cobrança do locatário

- Proprietário abre **pedido de avaria** até 48h após a devolução, com fotos, descrição e valor pretendido (até o limite da caução + estimativa extra).
- Locatário responde em 48h: concorda, contesta, ou propõe valor.
- Se concordam: plataforma captura caução até o valor; se faltar, **cobra o locatário** no mesmo meio de pagamento (Stripe) pela diferença. Sem pagamento da diferença em 5 dias: conta locatário suspensa para novas reservas; dívida permanece.
- Se contestam: staff decide com base nas evidências. Decisão é final no v1 (sem tribunal interno de segunda instância).
- Sem pedido no prazo: caução liberada, locação encerrada.
- Avaria comprovada **não** apaga a obrigação de avaliação, mas o staff pode ocultar texto ofensivo.

### 4.8 Avaliação

- Após locação encerrada (com ou sem avaria resolvida), cada lado avalia o outro (1–5) e texto opcional.
- Locatário avalia o bem e o proprietário; se houve operador, avalia o operador à parte.
- Proprietário avalia o locatário (cuidado com o bem, pontualidade).
- Avaliação só de quem participou da locação. Sem locação paga, sem review.
- Média visível no anúncio, no perfil do proprietário, do locatário e do operador.
- Staff pode remover review que viole termos (ameaça, dado pessoal, falso evidente).

### 4.9 Operador

- Anúncio declara: sem operador / operador opcional / operador obrigatório.
- Valor hora é do anúncio, não negociado em chat no v1.
- Horas cobradas = horas do período reservado (ou fração mínima declarada no anúncio, ex. 4h).
- Operador obrigatório em pesado quando o anúncio assim marcar; locatário não conclui reserva sem aceitar.
- Se o operador indicado recusar (proprietário cancela a parte operador), trata-se como cancelamento do proprietário se a máquina não puder ir sem ele.

## 5. Regras de negócio (resumo)

1. Sem login Google não há reserva nem anúncio.
2. Visitante lê anúncios públicos; dados de contato só depois do aceite.
3. Comissão 12% sobre aluguel + horas de operador; caução fora da comissão.
4. Caução é obrigatória em todo anúncio publicado.
5. Pagamento só pela plataforma (Stripe). Combinado por fora invalida proteção de avaria.
6. Proprietário não publica sem dados de recebimento.
7. Calendário é a fonte de disponibilidade; overlap é recusado.
8. Pesado pode exigir operador obrigatório; o anúncio decide, a busca respeita.
9. Pedido de avaria só na janela de 48h pós-devolução.
10. Conta com dívida de avaria extra não paga não reserva de novo até quitar ou staff perdoar.
11. Moeda: Real (BRL).
12. Superfície pública da aplicação: **canteiro.brenon.cloud**.

## 6. Critérios de aceite (produto)

- Dado um Google válido, o usuário cria conta e vê busca em até uma interação após o retorno do provedor.
- Proprietário publica furadeira (leve) e trator (pesado com operador obrigatório); os dois aparecem na busca com filtro de categoria e de operador.
- Locatário reserva a furadeira por 2 dias, paga, proprietário aceita, ambos veem contato; após devolução sem avaria, caução some do pendente e o proprietário vê o líquido (aluguel − 12%).
- Locatário reserva o trator com operador; o total mostra aluguel + horas + caução; o líquido do dono/operador respeita a regra da §4.5.
- Proprietário abre avaria com foto; locatário concorda; caução cobre em parte; o restante é cobrado do locatário.
- As duas partes deixam avaliação 1–5; a média aparece na ficha.
- Recusa ou expiração de aceite estorna o locatário.
- Host público da entrega: `https://canteiro.brenon.cloud` responde o produto (não 404 de zona vazia) quando o épico de publicação for aceito.

## 7. Fora de escopo (v1)

- Venda de equipamento (só aluguel).
- Cadastro e-mail/senha, Apple ou Microsoft.
- Chat livre além do necessário para retirada (combinado de horário pode ser campo na locação).
- Seguro formal / apólice.
- PIX como trilho paralelo (Stripe é o trilho; o que o Stripe mostrar no checkout é detalhe de pagamento, não um produto PIX da plataforma).
- Frota própria Canteiro, logística de entrega pela plataforma, motorista da plataforma.
- App nativo iOS/Android (web responsivo basta).
- Multi-moeda, anúncio internacional.
- Sublocação.

## 8. Não-objetivos de produto

- Não é ERP de locadora.
- Não é classificado (OLX) sem escrow.
- Não compete com o site da empresa (`brenon.cloud`) nem mora no git `home.cloud`.
