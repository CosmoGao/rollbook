package catalog_test

import (
	"errors"
	"testing"

	"github.com/CosmoGao/rollbook/internal/catalog"
)

func TestURLProtocolValidation(t *testing.T) {
	ctx, _, service, _, owner := serviceSetup(t)
	for _, value := range []string{"javascript:alert(1)", "//example.invalid", "https://", "data:text/html,hi"} {
		_, err := service.CreateAsset(ctx, owner, catalog.Object{ID: "endpoint:bad", Type: "endpoint", Name: "入口", Fields: map[string]any{"url": value}}, 0, "测试", "rest")
		var validation *catalog.ValidationError
		if !errors.As(err, &validation) || len(validation.Fields) == 0 || validation.Fields[0].Field != "fields.url" {
			t.Fatalf("%q: %v", value, err)
		}
	}
	for _, value := range []string{"https://example.invalid/a", "http://example.invalid"} {
		if !catalog.ValidHTTPURL(value) {
			t.Fatal(value)
		}
	}
}
