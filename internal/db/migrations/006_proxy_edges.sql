-- 旧版把入口的第二跳及后续反代写成了全局依赖。入口自身的 via_json 保留完整链。
CREATE TEMP TABLE obsolete_proxy_edges AS
SELECT d.id FROM dependencies d JOIN endpoints e
  ON d.id LIKE 'dependency:auto:' || e.id || ':%'
WHERE d.generated=1 AND d.kind='反代'
  AND (d.from_id<>e.service_id OR d.to_id<>json_extract(e.via_json, '$[0]'));

DELETE FROM dependencies WHERE id IN (SELECT id FROM obsolete_proxy_edges);
DELETE FROM objects WHERE id IN (SELECT id FROM obsolete_proxy_edges);
DROP TABLE obsolete_proxy_edges;
