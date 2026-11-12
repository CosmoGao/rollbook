ALTER TABLE changes ADD COLUMN base_object_json TEXT CHECK(base_object_json IS NULL OR json_valid(base_object_json));
