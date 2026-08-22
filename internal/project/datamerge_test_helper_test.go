package project

import "github.com/hypnotox/agentic-workflows/internal/config"

func withDefaultData(sc config.Sidecar, defaults map[string]any, listLayerExclusions ...string) config.Sidecar {
	if len(defaults) == 0 {
		return sc
	}
	excluded := make(map[string]bool, len(listLayerExclusions))
	for _, key := range listLayerExclusions {
		excluded[key] = true
	}
	merged := make(map[string]any, len(defaults)+len(sc.Data))
	for key, rawDefault := range defaults {
		defaultList, isList := rawDefault.([]any)
		if !isList || excluded[key] {
			merged[key] = cloneData(rawDefault)
			continue
		}
		authored, present := sc.Data[key]
		authoredList, authoredIsList := authored.([]any)
		keepDefault := true
		if value, configured := sc.DataDefaults[key]; configured && !value {
			keepDefault = false
		}
		capacity := len(authoredList)
		if keepDefault {
			capacity += len(defaultList)
		}
		list := make([]any, 0, capacity)
		if keepDefault {
			for _, item := range defaultList {
				list = append(list, cloneData(item))
			}
		}
		if present && authoredIsList {
			for _, item := range authoredList {
				list = append(list, cloneData(item))
			}
		}
		merged[key] = list
	}
	for key, value := range sc.Data {
		if _, catalogList := defaults[key].([]any); catalogList && !excluded[key] {
			continue
		}
		merged[key] = cloneData(value)
	}
	sc.Data = merged
	return sc
}

func cloneData(value any) any {
	switch typed := value.(type) {
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneData(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = cloneData(item)
		}
		return out
	case map[any]any:
		out := make(map[any]any, len(typed))
		for key, item := range typed {
			out[key] = cloneData(item)
		}
		return out
	default:
		return value
	}
}
