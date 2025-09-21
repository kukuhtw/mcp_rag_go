-- 0001_init.sql
-- Tabel inti (tanpa FK; FK & index di 0002)
-- Semua tabel InnoDB utf8mb4

CREATE TABLE IF NOT EXISTS doc_chunks (
  id        INT AUTO_INCREMENT PRIMARY KEY,
  doc_id    VARCHAR(64),
  title     VARCHAR(255),
  url       TEXT,
  snippet   TEXT,
  page_no   INT,
  embedding JSON NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS work_orders (
  wo_id      VARCHAR(32) PRIMARY KEY,
  asset_id   VARCHAR(64),
  area       VARCHAR(64),
  priority   TINYINT,
  status     VARCHAR(32) NOT NULL,    -- open|in-progress|closed
  due_date   DATE,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS purchase_orders (
  po_number  VARCHAR(64) PRIMARY KEY,
  vendor     VARCHAR(128),
  status     VARCHAR(32) NOT NULL,    -- created|approved|in-transit|delivered|closed
  eta        DATE,
  amount     BIGINT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS hsse_incidents (
  id          INT AUTO_INCREMENT PRIMARY KEY,
  category    VARCHAR(64)  NOT NULL,
  description TEXT         NOT NULL,
  event_time  DATETIME     NOT NULL,
  location    VARCHAR(128)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS wells (
  well_id VARCHAR(64) PRIMARY KEY,
  name    VARCHAR(128),
  area    VARCHAR(64),
  type    VARCHAR(32),
  status  VARCHAR(32)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS drilling_events (
  id         INT AUTO_INCREMENT PRIMARY KEY,
  well_id    VARCHAR(64) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  sub_cause  VARCHAR(128),
  start_time DATETIME NOT NULL,
  end_time   DATETIME NOT NULL,
  cost_usd   DOUBLE DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ts_signal (
  tag_id      VARCHAR(128) PRIMARY KEY,
  asset_id    VARCHAR(128),
  tag_name    VARCHAR(255),
  unit        VARCHAR(32),
  description TEXT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ts_value (
  tag_id   VARCHAR(128) NOT NULL,
  ts_utc   DATETIME     NOT NULL,
  value    DOUBLE,
  quality  TINYINT DEFAULT 1,
  PRIMARY KEY (tag_id, ts_utc)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS prod_allocation_daily (
  date       DATE        NOT NULL,
  well_id    VARCHAR(64) NOT NULL,
  gas_mmscfd DOUBLE,
  PRIMARY KEY (date, well_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
