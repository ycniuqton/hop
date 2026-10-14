package workspace

import (
	"context"
	"sort"
)

// StatusManager reports usage per shelf and per store folder.
type StatusManager struct {
	ws *Workspace
}

// Area is the storage mode a status row belongs to.
type Area string

const (
	AreaHot   Area = "hot"
	AreaStore Area = "store"
)

// Usage is the object count and size of one shelf or store folder.
type Usage struct {
	Area    Area   `json:"area"`
	Name    string `json:"name"`
	Objects int    `json:"objects"`
	Bytes   int64  `json:"bytes"`
}

// Usage lists hot shelves (shared first) and then each device's store.
func (m *StatusManager) Usage(ctx context.Context) ([]Usage, error) {
	hot, err := m.aggregate(ctx, AreaHot, m.ws.Layout.HotRoot(), m.ws.Layout.ShelfOf)
	if err != nil {
		return nil, err
	}
	st, err := m.aggregate(ctx, AreaStore, m.ws.Layout.StoreRoot(), m.ws.Layout.DeviceOfStoreKey)
	return append(hot, st...), err
}

func (m *StatusManager) aggregate(ctx context.Context, area Area, prefix string, group func(string) string) ([]Usage, error) {
	objs, err := m.ws.listAll(ctx, prefix)
	if err != nil {
		return nil, err
	}
	byName := map[string]*Usage{}
	for _, o := range objs {
		name := group(o.Key)
		if byName[name] == nil {
			byName[name] = &Usage{Area: area, Name: name}
		}
		byName[name].Objects++
		byName[name].Bytes += o.Size
	}
	out := make([]Usage, 0, len(byName))
	for _, u := range byName {
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Name == string(SharedShelf)) != (out[j].Name == string(SharedShelf)) {
			return out[i].Name == string(SharedShelf)
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
