-- Scope saved manifests to a haul so each workspace owns its own manifest
-- library. Manifest names are now unique per haul rather than globally.
DROP TABLE IF EXISTS saved_manifests;
CREATE TABLE saved_manifests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    haul_id INTEGER,
    name TEXT NOT NULL,
    description TEXT,
    yaml_content TEXT NOT NULL,
    tags TEXT, -- JSON array of tags
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(haul_id, name)
);
CREATE INDEX IF NOT EXISTS idx_saved_manifests_haul ON saved_manifests(haul_id);
