-- Settings table for global hauler flags and defaults
CREATE TABLE IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value TEXT,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Jobs table for tracking long-running operations
CREATE TABLE IF NOT EXISTS jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    command TEXT NOT NULL,
    args TEXT, -- JSON array of arguments
    env_overrides TEXT, -- JSON object of environment variable overrides
    status TEXT NOT NULL DEFAULT 'queued', -- queued, running, succeeded, failed
    exit_code INTEGER,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Job logs table for streaming log output
CREATE TABLE IF NOT EXISTS job_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL,
    stream TEXT NOT NULL, -- stdout or stderr
    content TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

-- Index for job logs queries
CREATE INDEX IF NOT EXISTS idx_job_logs_job_id ON job_logs(job_id, timestamp);

-- Saved manifests table for manifest library CRUD
CREATE TABLE IF NOT EXISTS saved_manifests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    yaml_content TEXT NOT NULL,
    tags TEXT, -- JSON array of tags
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Serve processes table for tracking background serve operations
CREATE TABLE IF NOT EXISTS serve_processes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    serve_type TEXT NOT NULL, -- registry or fileserver
    pid INTEGER,
    port INTEGER NOT NULL,
    args TEXT, -- JSON object of args passed to serve command
    status TEXT NOT NULL DEFAULT 'running', -- running, stopped, crashed
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    stopped_at DATETIME,
    exit_reason TEXT
);

-- Insert default settings
INSERT OR IGNORE INTO settings (key, value, description) VALUES ('log_level', 'info', 'Global log level for hauler operations');
INSERT OR IGNORE INTO settings (key, value, description) VALUES ('retries', '0', 'Default number of retries for failed operations');
INSERT OR IGNORE INTO settings (key, value, description) VALUES ('ignore_errors', 'false', 'Continue operations even when errors occur');
INSERT OR IGNORE INTO settings (key, value, description) VALUES ('default_platform', '', 'Default platform for multi-platform operations');
INSERT OR IGNORE INTO settings (key, value, description) VALUES ('default_key_path', '', 'Default path to cosign private key');
INSERT OR IGNORE INTO settings (key, value, description) VALUES ('temp_dir', '', 'Temporary directory for hauler operations');
