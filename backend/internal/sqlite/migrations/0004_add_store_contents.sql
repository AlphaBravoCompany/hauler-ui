-- store_contents: tracks items in the store with their source haul
CREATE TABLE IF NOT EXISTS store_contents (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  content_type TEXT NOT NULL, -- 'image', 'chart', 'file'
  name TEXT NOT NULL,         -- e.g., 'nginx:latest', 'ingress-nginx:4.11.3'
  digest TEXT,                 -- content digest for deduplication
  source_haul TEXT,           -- which haul this came from (e.g., 'test-new.tar.zst')
  loaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(content_type, name, digest)
);

-- Index for faster lookups
CREATE INDEX IF NOT EXISTS idx_store_contents_type ON store_contents(content_type);
CREATE INDEX IF NOT EXISTS idx_store_contents_source ON store_contents(source_haul);
