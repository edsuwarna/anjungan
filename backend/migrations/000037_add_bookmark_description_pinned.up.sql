ALTER TABLE bookmarks ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE bookmarks ADD COLUMN IF NOT EXISTS pinned BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_bookmarks_pinned ON bookmarks(user_id, pinned, sort_order);
