package history

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ImageRef 用户消息中的图片外置引用（正文只留短标记，避免进模截断吃掉 base64）。
type ImageRef struct {
	ID   int64
	MIME string
}

var imageMarkerRe = regexp.MustCompile(`<!-- nh-image id="(\d+)" mime="([^"]+)" -->`)

// EncodeImageHead 把图片引用写在正文开头（MaxMsgRunes 从头部保留）。
func EncodeImageHead(refs []ImageRef) string {
	if len(refs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, r := range refs {
		if r.ID <= 0 {
			continue
		}
		mime := strings.TrimSpace(r.MIME)
		if mime == "" {
			mime = "image/png"
		}
		fmt.Fprintf(&b, `<!-- nh-image id="%d" mime="%s" -->`+"\n", r.ID, mime)
	}
	return b.String()
}

// SplitImageRefs 拆出图片标记与可见正文。
func SplitImageRefs(content string) (text string, refs []ImageRef) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil
	}
	matches := imageMarkerRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil
	}
	var stripped strings.Builder
	last := 0
	for _, loc := range matches {
		stripped.WriteString(content[last:loc[0]])
		id, _ := strconv.ParseInt(content[loc[2]:loc[3]], 10, 64)
		mime := strings.TrimSpace(content[loc[4]:loc[5]])
		if id > 0 {
			refs = append(refs, ImageRef{ID: id, MIME: mime})
		}
		last = loc[1]
	}
	stripped.WriteString(content[last:])
	return strings.TrimSpace(stripped.String()), refs
}
