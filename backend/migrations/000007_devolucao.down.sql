-- Rollback for F5 migration 000006.
DROP INDEX IF EXISTS dividas_due_at_idx;
DROP INDEX IF EXISTS dividas_renter_state_idx;
DROP TABLE IF EXISTS dividas;
DROP INDEX IF EXISTS avaria_pedidos_state_idx;
DROP INDEX IF EXISTS avaria_pedidos_renter_idx;
DROP INDEX IF EXISTS avaria_pedidos_owner_idx;
DROP TABLE IF EXISTS avaria_pedidos;
DROP INDEX IF EXISTS devolucoes_state_idx;
DROP TABLE IF EXISTS devolucoes;
