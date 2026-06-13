ALTER TABLE bookmarks ADD CONSTRAINT bookmarks_category_check
    CHECK (category IN (
        'Monitoring', 'CI/CD', 'Logging',
        'Code & Registry', 'Internal Tools', 'Other'
    ));
