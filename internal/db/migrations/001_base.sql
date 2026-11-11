CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

-- 业务对象及其索引由后续数据模型迁移添加。
CREATE VIRTUAL TABLE search_documents USING fts5(
    object_id UNINDEXED,
    name,
    aliases,
    tags,
    description,
    url
);
