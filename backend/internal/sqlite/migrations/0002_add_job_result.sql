-- Add result column to jobs table for storing operation metadata
-- This will store JSON like {"archivePath": "/data/haul.tar.zst"} for store save operations
ALTER TABLE jobs ADD COLUMN result TEXT;
