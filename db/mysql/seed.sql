-- Tambahan wells
INSERT INTO wells (well_id, name, type, status)
VALUES
  ('WELL_D01', 'Well D-01', 'oil', 'producing'),
  ('WELL_D02', 'Well D-02', 'oil', 'producing'),
  ('WELL_E05', 'Well E-05', 'gas', 'testing'),
  ('WELL_F10', 'Well F-10', 'gas', 'shut-in')
ON DUPLICATE KEY UPDATE name=VALUES(name);

-- Tambahan signal tags
INSERT INTO ts_signal (tag_id, asset_id, tag_name, unit, description)
VALUES
  ('OIL_D01', 'WELL_D01', 'Oil rate', 'BOPD', 'Dummy oil rate for Well D-01'),
  ('OIL_D02', 'WELL_D02', 'Oil rate', 'BOPD', 'Dummy oil rate for Well D-02'),
  ('FLOW_E05', 'WELL_E05', 'Gas flow', 'MMSCFD', 'Gas test flow rate'),
  ('FLOW_F10', 'WELL_F10', 'Gas flow', 'MMSCFD', 'Shut-in gas tag')
ON DUPLICATE KEY UPDATE tag_name=VALUES(tag_name);
