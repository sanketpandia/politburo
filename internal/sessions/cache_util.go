package sessions

import (
	"encoding/json"
	"fmt"
	"strings"

	"infinite-experiment/politburo/infra/cache"
)

type ServerOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func GetSessionIDs(c cache.CacheInterface) ([]string, error) {
	val, found := c.Get(cache.KeySessionList)
	if !found {
		return nil, nil
	}

	sessionList, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("invalid session list type")
	}

	ids := make([]string, 0)
	for _, id := range strings.Split(sessionList, "|") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func GetSessionName(c cache.CacheInterface, sessionID string) string {
	val, found := c.Get(cache.SessionNameKey(sessionID))
	if !found {
		return sessionID
	}
	if name, ok := val.(string); ok && name != "" {
		return name
	}
	return sessionID
}

func GetAllServers(c cache.CacheInterface) ([]ServerOption, error) {
	val, found := c.Get(cache.KeyServerList)
	if !found {
		ids, err := GetSessionIDs(c)
		if err != nil {
			return nil, err
		}
		servers := make([]ServerOption, 0, len(ids))
		for _, id := range ids {
			servers = append(servers, ServerOption{ID: id, Name: GetSessionName(c, id)})
		}
		return servers, nil
	}

	data, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	var servers []ServerOption
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, err
	}
	return servers, nil
}
