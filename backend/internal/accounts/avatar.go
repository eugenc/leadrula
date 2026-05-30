package accounts

import (
	"encoding/json"
	"strings"
)

const maxAvatarBytes = 2 << 20 // 2 MiB

var avatarExtByType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func avatarURLFromPrefs(prefs []byte) *string {
	if len(prefs) == 0 {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(prefs, &m) != nil {
		return nil
	}
	v, _ := m["avatar_url"].(string)
	if v == "" {
		return nil
	}
	return &v
}

func avatarExt(contentType string) (string, bool) {
	ext, ok := avatarExtByType[strings.ToLower(strings.TrimSpace(contentType))]
	return ext, ok
}
