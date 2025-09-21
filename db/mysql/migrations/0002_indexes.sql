-- 0002_indexes.sql
-- Foreign Keys + Indexes (idempotent-friendly)

-- ========== Guard: index work_orders(updated_at) hanya jika kolomnya ada ==========
SET @hascol := (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'work_orders'
    AND column_name = 'updated_at'
);

-- Drop idx_wo_updated jika ada dan kolomnya memang ada
SET @sql := IF(
  @hascol = 1,
  'DROP INDEX IF EXISTS `idx_wo_updated` ON `work_orders`',
  'SELECT 1'
);
PREPARE s1 FROM @sql; EXECUTE s1; DEALLOCATE PREPARE s1;

-- Buat idx_wo_updated jika kolomnya ada
SET @sql := IF(
  @hascol = 1,
  'CREATE INDEX `idx_wo_updated` ON `work_orders`(`updated_at`)',
  'SELECT 1'
);
PREPARE s2 FROM @sql; EXECUTE s2; DEALLOCATE PREPARE s2;

-- ===================== Guards helper: add FK only if not exists ===================
-- drilling_events -> wells
SET @fk_name := 'fk_drilling_well';
SET @ddl := (
  SELECT IF(
    EXISTS(
      SELECT 1 FROM information_schema.REFERENTIAL_CONSTRAINTS
      WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = @fk_name
    ),
    'SELECT 1',
    'ALTER TABLE drilling_events ADD CONSTRAINT fk_drilling_well
       FOREIGN KEY (well_id) REFERENCES wells(well_id)
       ON UPDATE CASCADE ON DELETE RESTRICT'
  )
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ts_value -> ts_signal
SET @fk_name := 'fk_ts_value_signal';
SET @ddl := (
  SELECT IF(
    EXISTS(
      SELECT 1 FROM information_schema.REFERENTIAL_CONSTRAINTS
      WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = @fk_name
    ),
    'SELECT 1',
    'ALTER TABLE ts_value ADD CONSTRAINT fk_ts_value_signal
       FOREIGN KEY (tag_id) REFERENCES ts_signal(tag_id)
       ON UPDATE CASCADE ON DELETE RESTRICT'
  )
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- prod_allocation_daily -> wells
SET @fk_name := 'fk_prod_well';
SET @ddl := (
  SELECT IF(
    EXISTS(
      SELECT 1 FROM information_schema.REFERENTIAL_CONSTRAINTS
      WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = @fk_name
    ),
    'SELECT 1',
    'ALTER TABLE prod_allocation_daily ADD CONSTRAINT fk_prod_well
       FOREIGN KEY (well_id) REFERENCES wells(well_id)
       ON UPDATE CASCADE ON DELETE RESTRICT'
  )
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ===================== Secondary Indexes (drop-if-exists -> create) ===============
-- helper: drop index if exists procedure (temp in-session)
DROP PROCEDURE IF EXISTS drop_index_if_exists;
DELIMITER $$
CREATE PROCEDURE drop_index_if_exists(IN tbl VARCHAR(255), IN idx VARCHAR(255))
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = tbl AND index_name = idx
  ) THEN
    SET @sql := CONCAT('DROP INDEX `', idx, '` ON `', tbl, '`');
    PREPARE x FROM @sql; EXECUTE x; DEALLOCATE PREPARE x;
  END IF;
END$$
DELIMITER ;

-- hsse_incidents
CALL drop_index_if_exists('hsse_incidents','idx_hsse_time');
CREATE INDEX idx_hsse_time ON hsse_incidents(event_time);

CALL drop_index_if_exists('hsse_incidents','idx_hsse_cat_time');
CREATE INDEX idx_hsse_cat_time ON hsse_incidents(category, event_time);

-- purchase_orders
CALL drop_index_if_exists('purchase_orders','idx_po_status');
CREATE INDEX idx_po_status ON purchase_orders(status);

CALL drop_index_if_exists('purchase_orders','idx_po_vendor_eta');
CREATE INDEX idx_po_vendor_eta ON purchase_orders(vendor, eta);

CALL drop_index_if_exists('purchase_orders','idx_po_eta');
CREATE INDEX idx_po_eta ON purchase_orders(eta);

-- work_orders (LAINNYA; updated_at SUDAH ditangani di guard atas)
CALL drop_index_if_exists('work_orders','idx_wo_status');
CREATE INDEX idx_wo_status ON work_orders(status);

CALL drop_index_if_exists('work_orders','idx_wo_due');
CREATE INDEX idx_wo_due ON work_orders(due_date);

CALL drop_index_if_exists('work_orders','idx_wo_priority_stat');
CREATE INDEX idx_wo_priority_stat ON work_orders(priority, status);

CALL drop_index_if_exists('work_orders','idx_wo_asset');
CREATE INDEX idx_wo_asset ON work_orders(asset_id);

CALL drop_index_if_exists('work_orders','idx_wo_area');
CREATE INDEX idx_wo_area ON work_orders(area);

-- drilling_events
-- JANGAN drop index yang dipakai FK. Buat jika belum ada saja.
SET @need := (
  SELECT COUNT(*) = 0
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name  = 'drilling_events'
    AND index_name  = 'idx_drill_well_start'
);
SET @sql := IF(
  @need,
  'CREATE INDEX idx_drill_well_start ON drilling_events(well_id, start_time)',
  'SELECT 1'
);
PREPARE xi FROM @sql; EXECUTE xi; DEALLOCATE PREPARE xi;

-- ts_value
CALL drop_index_if_exists('ts_value','idx_ts_value_tag_time');
CREATE INDEX idx_ts_value_tag_time ON ts_value(tag_id, ts_utc);

-- prod_allocation_daily
CALL drop_index_if_exists('prod_allocation_daily','idx_prod_date_well');
CREATE INDEX idx_prod_date_well ON prod_allocation_daily(date, well_id);

-- doc_chunks FULLTEXT
CALL drop_index_if_exists('doc_chunks','ft_title_snippet');
CREATE FULLTEXT INDEX ft_title_snippet ON doc_chunks(title, snippet);

-- cleanup helper proc
DROP PROCEDURE IF EXISTS drop_index_if_exists;
