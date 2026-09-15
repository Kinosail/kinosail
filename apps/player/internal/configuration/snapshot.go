package configuration

import (
	"encoding/json"
	"maps"
	"sort"
	"strconv"
	"time"
)

func (snapshot Snapshot) String(key string) string { return snapshot.values[key].raw }
func (snapshot Snapshot) Bool(key string) bool {
	result, _ := strconv.ParseBool(snapshot.String(key))
	return result
}

func (snapshot Snapshot) Int(key string) int {
	result, _ := strconv.Atoi(snapshot.String(key))
	return result
}

func (snapshot Snapshot) Duration(key string) time.Duration {
	result, _ := time.ParseDuration(snapshot.String(key))
	return result
}

func (snapshot Snapshot) Strings(key string) []string {
	var result []string
	_ = json.Unmarshal([]byte(snapshot.String(key)), &result)
	return result
}
func (snapshot Snapshot) Source(key string) Source { return snapshot.values[key].source }
func (snapshot Snapshot) Managed(key string) bool {
	source := snapshot.Source(key)
	return source == YAML || source == Environment
}

func (snapshot Snapshot) Public(key string) PublicValue {
	spec, _ := find(key)
	item := snapshot.values[key]
	result := PublicValue{spec.Key, spec.Env, item.raw, item.source, spec.Secret, spec.Restart, item.raw != ""}
	if spec.Secret {
		result.Value = ""
	}
	return result
}

func (snapshot Snapshot) Fields() []PublicValue {
	result := make([]PublicValue, 0, len(specs))
	for _, spec := range specs {
		result = append(result, snapshot.Public(spec.Key))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

// Clone returns an independent configuration snapshot.
func (snapshot Snapshot) Clone() Snapshot {
	values := make(map[string]value, len(snapshot.values))
	maps.Copy(values, snapshot.values)
	return Snapshot{values: values}
}
