package workspace

import (
	"context"
	"sort"

	"github.com/ycniuqton/hop/internal/naming"
	"github.com/ycniuqton/hop/internal/target"
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

// areaRefs is what follows @name for each area: store locations take a colon.
var areaRefs = map[Area]string{AreaHot: "", AreaStore: target.Syntax.PathSep}

// Ref is the @ spelling of a usage row: @@ or @web-1 for shelves, @@: or @web-1: for stores.
func (u Usage) Ref() string {
	return target.Syntax.DeviceMark + naming.Devices.Alias(u.Name) + areaRefs[u.Area]
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
