CREATE TABLE bookmarks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    url         TEXT NOT NULL,
    icon_type   TEXT NOT NULL DEFAULT 'auto'
                CHECK (icon_type IN ('auto', 'iconify', 'emoji')),
    icon_value  TEXT,
    category    TEXT NOT NULL DEFAULT 'Other'
                CHECK (category IN (
                    'Monitoring', 'CI/CD', 'Logging',
                    'Code & Registry', 'Internal Tools', 'Other'
                )),
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bookmarks_user_id ON bookmarks(user_id);
CREATE INDEX idx_bookmarks_sort ON bookmarks(user_id, sort_order);
