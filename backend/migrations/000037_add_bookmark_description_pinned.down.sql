DROP INDEX IF EXISTS idx_bookmarks_pinned;
ALTER TABLE bookmarks DROP COLUMN IF EXISTS pinned;
ALTER TABLE bookmarks DROP COLUMN IF EXISTS description;
