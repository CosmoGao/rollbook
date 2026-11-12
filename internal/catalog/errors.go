package catalog

import (
	"fmt"
	"strings"
)

// FieldError 描述被拒绝的字段，不保存提交的值。
type FieldError struct {
	Field      string   `json:"field"`
	Reason     string   `json:"reason"`
	Expected   string   `json:"expected,omitempty"`
	References []string `json:"references,omitempty"`
}
type ValidationError struct {
	Fields []FieldError `json:"fields"`
	cause  error
}
type ConflictError struct {
	Fields []FieldError `json:"fields"`
}

func (e *ConflictError) Error() string        { return "field conflict" }
func (e *ConflictError) Is(target error) bool { return target == ErrConflict }

func (e *ValidationError) Error() string { return "validation failed" }
func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalid || (target == ErrSecret && e.cause == ErrSecret)
}
func invalidField(field, reason, expected string) error {
	return &ValidationError{Fields: []FieldError{{Field: field, Reason: reason, Expected: expected}}}
}

func secretField(field string) error {
	return &ValidationError{Fields: []FieldError{{Field: field, Reason: "疑似包含秘密", Expected: "使用凭据引用，不写秘密值"}}, cause: ErrSecret}
}

func checkCreateSecrets(o Object) error {
	for field, value := range map[string]any{"object.name": o.Name, "object.aliases": o.Aliases, "object.tags": o.Tags} {
		if CheckSecrets(value) != nil {
			return secretField(field)
		}
	}
	for field, value := range o.Fields {
		if CheckSecrets(map[string]any{field: value}) != nil {
			return secretField("fields." + field)
		}
	}
	return nil
}

func storageError(err error, o Object) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "PRIMARY KEY") {
		return fmt.Errorf("%w: object or field already exists", ErrConflict)
	}
	if strings.Contains(msg, "NOT NULL constraint failed") {
		field := strings.Fields(msg[strings.LastIndex(msg, ".")+1:])[0]
		return invalidField("fields."+field, "必填字段缺失", "非空值")
	}
	if strings.Contains(msg, "FOREIGN KEY constraint failed") {
		for k, v := range o.Fields {
			if strings.HasSuffix(k, "_id") && v != nil {
				return invalidField("fields."+k, "关联对象不存在", "现有对象 ID")
			}
		}
		return invalidField("fields", "关联对象不存在或仍被引用", "有效关联")
	}
	if strings.Contains(msg, "CHECK constraint failed") {
		return invalidField("fields", "字段值不符合模型约束", "允许的枚举或范围")
	}
	return err
}
