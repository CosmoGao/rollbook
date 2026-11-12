package backup

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/CosmoGao/rollbook/internal/catalog"
	"gopkg.in/yaml.v3"
)

var csvFields = map[string][]string{
	"host":    {"id", "name", "rev", "status", "source", "confidence", "kind", "location_id", "net_role", "uplink_id", "parent_host_id", "model", "os", "os_version", "resources_json", "mgmt_url"},
	"network": {"id", "name", "rev", "status", "source", "confidence", "cidr", "location_id", "router_id", "uplink_id", "vlan_id", "gateway", "dhcp_range", "purpose"},
	"ip":      {"id", "name", "rev", "status", "source", "confidence", "address", "network_id", "host_id", "interface", "mac", "assign"},
	"service": {"id", "name", "rev", "status", "source", "confidence", "host_id", "kind", "deploy_method", "upstream_json", "owner", "config_path", "docs_url"},
}
var csvRequired = map[string][]string{
	"host":    {"id", "name", "kind", "location_id"},
	"network": {"id", "name", "cidr", "location_id"},
	"ip":      {"id", "name", "address", "network_id", "host_id"},
	"service": {"id", "name", "host_id"},
}

func CSVTemplate(kind string) ([]byte, error) {
	fields, ok := csvFields[kind]
	if !ok {
		return nil, ErrInvalid
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if err := w.Write(fields); err != nil {
		return nil, err
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

func csvFieldError(line int, column, reason string) error {
	return &catalog.ValidationError{Fields: []catalog.FieldError{{Field: fmt.Sprintf("第 %d 行 %s 列", line, column), Reason: reason}}}
}

// CSVToMerge 把单一对象类型的 UTF-8 CSV 转成现有合并导入文件，后续仍走同一审批路径。
func CSVToMerge(kind string, data []byte) ([]byte, error) {
	fields, ok := csvFields[kind]
	if !ok {
		return nil, ErrInvalid
	}
	if len(data) == 0 || len(data) > MaxSize || !utf8.Valid(data) {
		return nil, fmt.Errorf("%w: CSV 必须为 UTF-8 且不超过 16 MiB", ErrInvalid)
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = false
	header, err := r.Read()
	if err != nil {
		return nil, csvFieldError(1, "表头", "CSV 表头无效")
	}
	allowed := map[string]bool{}
	for _, field := range fields {
		allowed[field] = true
	}
	seen := map[string]bool{}
	for _, col := range header {
		if !allowed[col] {
			return nil, csvFieldError(1, col, "未知表头")
		}
		if seen[col] {
			return nil, csvFieldError(1, col, "重复表头")
		}
		seen[col] = true
	}
	for _, col := range csvRequired[kind] {
		if !seen[col] {
			return nil, csvFieldError(1, col, "缺少必填表头")
		}
	}
	objects := []map[string]any{}
	ids := map[string]bool{}
	line := 2
	for {
		record, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, csvFieldError(line, "CSV", readErr.Error())
		}
		line, _ = r.FieldPos(0)
		if len(record) != len(header) {
			return nil, csvFieldError(line, "CSV", "列数与表头不一致")
		}
		blank := true
		for _, v := range record {
			if strings.TrimSpace(v) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		values := map[string]string{}
		for i, col := range header {
			values[col] = strings.TrimSpace(record[i])
		}
		for _, col := range csvRequired[kind] {
			if values[col] == "" {
				return nil, csvFieldError(line, col, "必填值为空")
			}
		}
		id := values["id"]
		if !strings.HasPrefix(id, kind+":") {
			return nil, csvFieldError(line, "id", "ID 须以 "+kind+": 开头")
		}
		if ids[id] {
			return nil, csvFieldError(line, "id", "文件内重复 ID")
		}
		ids[id] = true
		obj := map[string]any{"id": id, "type": kind, "name": values["name"]}
		detail := map[string]any{}
		for _, col := range header {
			v := values[col]
			if v == "" || col == "id" || col == "name" {
				continue
			}
			switch col {
			case "rev", "vlan_id":
				n, e := strconv.ParseInt(v, 10, 64)
				if e != nil || n < 1 {
					return nil, csvFieldError(line, col, "必须为正整数")
				}
				if col == "rev" {
					obj[col] = n
				} else {
					detail[col] = n
				}
			case "status", "source", "confidence":
				obj[col] = v
			case "mgmt_url", "docs_url":
				if !catalog.ValidHTTPURL(v) {
					return nil, csvFieldError(line, col, "仅接受绝对 HTTP 或 HTTPS 地址")
				}
				detail[col] = v
			case "resources_json", "upstream_json":
				if !json.Valid([]byte(v)) {
					return nil, csvFieldError(line, col, "不是合法 JSON")
				}
				detail[col] = v
			default:
				detail[col] = v
			}
		}
		obj["fields"] = detail
		objects = append(objects, obj)
	}
	if len(objects) == 0 {
		return nil, fmt.Errorf("%w: CSV 没有数据行", ErrInvalid)
	}
	return yaml.Marshal(File{SchemaVersion: SchemaVersion, Kind: "merge", Objects: objects})
}
