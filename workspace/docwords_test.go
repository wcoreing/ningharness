package workspace

import (
	"testing"
)

func TestCountPlain(t *testing.T) {
	if Count("你好") != 2 {
		t.Fatalf("plain")
	}
	if Count("  a b\n") != 2 {
		t.Fatalf("latin letters, no spaces counted")
	}
}

func TestCountStripsMarkdownShell(t *testing.T) {
	cover := `# 智慧园区综合运维服务

## 技术方案分册

| 项 | 内容 |
| --- | --- |
| 项目名称 | 某市智慧园区综合运维服务采购 |
| 采购人 | 某市园区管委会 |
| 分册 | 技术方案（评分项 40 分 · 设计方案冲三档） |
| 服务期 | 36 个月 |
| 响应时限 | 一般故障 4 小时到场 |
| 付款 | 按季验收合格后付当期 90% |

> 本册技术方案共四章：总体方案、统一运维值班与派单、弱电安防巡检、季度演练与报告。
> Word/WPS 中请 **F9 更新目录域** 以生成页码。
`
	n := Count(cover)
	raw := CountRaw(cover)
	if n >= raw {
		t.Fatalf("prose %d should be < raw %d", n, raw)
	}
	if n < 100 || n > 200 {
		t.Fatalf("cover prose count out of expected band: %d (raw=%d)", n, raw)
	}
	if Count("```mermaid\nflowchart TD\nA-->B\n```\n正文甲") != 3 {
		t.Fatalf("fence should not count; want 正文甲 = 3, got %d", Count("```mermaid\nflowchart TD\nA-->B\n```\n正文甲"))
	}
}

func TestCoverFileBand(t *testing.T) {
	// 用内嵌样例代替本机绝对路径，避免绑定外部产品目录。
	sample := "# 封面\n\n甲方名称\n项目名称\n"
	n := Count(sample)
	if n < 4 {
		t.Fatalf("prose count=%d", n)
	}
	t.Logf("cover prose=%d raw=%d", n, CountRaw(sample))
}
