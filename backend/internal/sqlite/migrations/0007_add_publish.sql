-- Publish layer: a "published" haul is exposed through hauler-ui's single front
-- door (host-routed registry + path-routed files). Reuse serve_processes to
-- track the internal registry backing a published haul.
ALTER TABLE serve_processes ADD COLUMN role TEXT DEFAULT 'manual'; -- 'manual' | 'published'
ALTER TABLE serve_processes ADD COLUMN hostname TEXT;              -- registry virtual host for published hauls
