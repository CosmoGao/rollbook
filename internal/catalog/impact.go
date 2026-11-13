package catalog

import (
	"context"
	"database/sql"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/search"
)

type ImpactItem struct {
	Object Object   `json:"object"`
	Path   []string `json:"path"`
	Source string   `json:"source"`
}

type ImpactResult struct {
	Root      Object       `json:"root"`
	Hosts     []ImpactItem `json:"hosts"`
	Services  []ImpactItem `json:"services"`
	Endpoints []ImpactItem `json:"endpoints"`
}

func (q Query) Impact(ctx context.Context, p auth.Principal, kind, id string, recursive bool) (ImpactResult, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return ImpactResult{}, err
	}
	return impact(ctx, q.Service.Store.DB, kind, id, recursive)
}

func impact(ctx context.Context, db interface {
	rowQuerier
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, kind, id string, recursive bool) (ImpactResult, error) {
	result := ImpactResult{Hosts: []ImpactItem{}, Services: []ImpactItem{}, Endpoints: []ImpactItem{}}
	root, err := get(ctx, db, id)
	if err != nil {
		return result, err
	}
	if root.Type != kind {
		return result, invalidField("id", "对象类型不匹配", kind+" ID")
	}
	result.Root = root
	graph, err := search.ImpactFrom(ctx, db, kind, id, recursive)
	if err != nil {
		return result, err
	}
	for _, part := range []struct {
		nodes []search.ImpactNode
		dst   *[]ImpactItem
	}{{graph.Hosts, &result.Hosts}, {graph.Services, &result.Services}, {graph.Endpoints, &result.Endpoints}} {
		for _, node := range part.nodes {
			o, e := get(ctx, db, node.ID)
			if e != nil {
				return result, e
			}
			*part.dst = append(*part.dst, ImpactItem{Object: o, Path: node.Path, Source: node.Source})
		}
	}
	return result, nil
}
