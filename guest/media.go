package guest

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// UserImage 进模图片（base64 无 data: 前缀）。
type UserImage struct {
	MIME string
	Data string
}

// UserMessage 构造 user schema：有图则走 UserInputMultiContent（OpenAI/千问多模态协议），无图保持纯文本。
func UserMessage(text string, images []UserImage) *schema.Message {
	text = strings.TrimSpace(text)
	if len(images) == 0 {
		return schema.UserMessage(text)
	}
	parts := make([]schema.MessageInputPart, 0, len(images)+1)
	if text != "" {
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: text,
		})
	}
	for _, img := range images {
		mime := strings.TrimSpace(img.MIME)
		data := strings.TrimSpace(img.Data)
		if mime == "" || data == "" {
			continue
		}
		m, d := mime, data
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: &d,
					MIMEType:   m,
				},
			},
		})
	}
	if len(parts) == 0 {
		return schema.UserMessage(text)
	}
	return &schema.Message{
		Role:                  schema.User,
		UserInputMultiContent: parts,
	}
}

// RewriteGenerateError 有图时把「明确不支持视觉」转成可行动提示；不维护模型名单。
func RewriteGenerateError(err error, hadMedia bool) error {
	if err == nil || !hadMedia {
		return err
	}
	raw := err.Error()
	if !looksLikeMediaReject(raw) {
		return err
	}
	return fmt.Errorf("当前接口不支持图片（已按多模态协议发送，被服务端拒绝）。请换视觉/多模态模型，或去掉图片后重试。\n%s", raw)
}

func looksLikeMediaReject(raw string) bool {
	s := strings.ToLower(raw)
	if strings.Contains(s, "image detail") {
		return false
	}
	media := strings.Contains(s, "image") ||
		strings.Contains(s, "vision") ||
		strings.Contains(s, "multimodal") ||
		strings.Contains(s, "image_url") ||
		strings.Contains(raw, "图片") ||
		strings.Contains(raw, "图像") ||
		strings.Contains(raw, "多模态")
	reject := strings.Contains(s, "unsupport") ||
		strings.Contains(s, "not support") ||
		strings.Contains(s, "not allow") ||
		strings.Contains(s, "unknown parameter") ||
		strings.Contains(raw, "不支持") ||
		strings.Contains(raw, "拒绝")
	return media && reject
}
