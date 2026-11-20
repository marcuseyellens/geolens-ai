// Package aeo 实现《WBS_AEO_Feature_Extraction_v1.0》增量工作包的可验证逻辑。
//
// 合规红线：这里只处理用户自述与公开可查信息，代码不含任何模拟登录态、
// 代认领、代填或绕过反爬的路径（文档 §7.1 / §7.2）。
package aeo

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/answerlens/answerlens/domain"
)

// ---- A-WP-1.1 / 1.2 三方一致性比对 ----

// ThirdPartySnapshot 是从公开可查页面得到的字段快照，附带证据片段与来源链接。
// 它只能来自公开页面，不接受登录态内容。
type ThirdPartySnapshot struct {
	Fields     map[string]string `json:"fields"`
	Excerpts   map[string]string `json:"excerpts"`
	SourceURLs map[string]string `json:"source_urls"`
	ObservedAt time.Time         `json:"observed_at"`
	StaleAfter time.Duration     `json:"stale_after"`
}

// OfficialEntryURL 返回某字段的官方核对入口。只给链接，不代认领、不代填（A-WP-1.3）。
func OfficialEntryURL(provider, field string) string {
	switch strings.ToLower(provider) {
	case "google":
		return "https://business.google.com/"
	case "bing":
		return "https://www.bingplaces.com/"
	case "apple":
		return "https://businessconnect.apple.com/"
	default:
		return "https://business.google.com/"
	}
}

// CompareProfiles 对 7 个字段做差异检测，返回 missing / conflict / stale 三类结果。
// 每个结果都带证据片段与官方入口链接；不写入任何第三方系统。
func CompareProfiles(profile domain.BusinessProfile, snapshot ThirdPartySnapshot, now time.Time, provider string) []domain.ProfileDiff {
	out := []domain.ProfileDiff{}
	staleAfter := snapshot.StaleAfter
	if staleAfter <= 0 {
		staleAfter = 90 * 24 * time.Hour
	}
	for _, field := range domain.ProfileFields {
		first := strings.TrimSpace(profile.Fields[field])
		third := strings.TrimSpace(snapshot.Fields[field])
		diff := domain.ProfileDiff{
			Field: field, FirstParty: first, ThirdParty: third,
			Excerpt:     strings.TrimSpace(snapshot.Excerpts[field]),
			OfficialURL: OfficialEntryURL(provider, field),
		}
		if third == "" {
			if first == "" {
				continue // 两边都没有：unknown，不产生差异
			}
			diff.Kind = domain.DiffMissing
			diff.Excerpt = "公开页面未找到该字段"
		} else if first == "" {
			diff.Kind = domain.DiffMissing
			diff.FirstParty = "（用户未自述）"
		} else if !sameValue(field, first, third) {
			diff.Kind = domain.DiffConflict
		} else if !snapshot.ObservedAt.IsZero() && now.Sub(snapshot.ObservedAt) > staleAfter {
			diff.Kind = domain.DiffStale
		} else {
			continue // 一致且新鲜
		}
		out = append(out, diff)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}

func sameValue(field, a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.Join(strings.Fields(s), " "))
		s = strings.TrimSuffix(s, ".")
		return strings.TrimSpace(s)
	}
	if field == domain.FieldWebsite {
		return strings.TrimSuffix(norm(a), "/") == strings.TrimSuffix(norm(b), "/")
	}
	if field == domain.FieldPhone {
		strip := func(s string) string {
			return strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", "+", "").Replace(s)
		}
		return strip(a) == strip(b)
	}
	return norm(a) == norm(b)
}

// ProfileStatusFor 只依据用户自述或公开可查信息推导状态，默认 unknown。
func ProfileStatusFor(profile domain.BusinessProfile, diffs []domain.ProfileDiff) domain.ProfileStatus {
	if profile.Status == domain.ProfileClaimed {
		for _, d := range diffs {
			if d.Kind == domain.DiffConflict {
				return domain.ProfileConflict
			}
			if d.Kind == domain.DiffStale {
				return domain.ProfileStale
			}
		}
		return domain.ProfileClaimed
	}
	if len(profile.Fields) == 0 {
		return domain.ProfileUnknown
	}
	return domain.ProfileUnclaimed
}

// ScoreWithProfile 实现专属验收：unknown 状态不降低 /100 分数。
// 有无档案状态，同分项得分必须一致。
func ScoreWithProfile(baseScore float64, status domain.ProfileStatus) float64 {
	if status == domain.ProfileUnknown {
		return baseScore
	}
	return baseScore
}

// ProfileSyncChecklist 生成认领引导修复件（A-WP-1.3）。
// 内容只含官方链接与证据，绝不代认领、不代填。
func ProfileSyncChecklist(brand string, diffs []domain.ProfileDiff) domain.FixArtifact {
	var b strings.Builder
	b.WriteString("# 第一方档案核对清单\n\n")
	b.WriteString("本清单只列出公开可查信息与你的自述之间的差异。请自行通过官方入口核对并修改；")
	b.WriteString("本平台不代认领、不代填、不模拟登录态。\n\n")
	b.WriteString("> " + domain.NeutralityStatement + "\n\n")
	if len(diffs) == 0 {
		b.WriteString("当前未发现可核验的差异。状态保持 unknown 不影响站点就绪度分数。\n")
	} else {
		b.WriteString("| 字段 | 差异类型 | 你的自述 | 公开页面 | 证据片段 | 官方入口 |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
		for _, d := range diffs {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
				d.Field, d.Kind, mdCell(d.FirstParty), mdCell(d.ThirdParty), mdCell(d.Excerpt), d.OfficialURL))
		}
	}
	b.WriteString("\n## 说明\n\n")
	b.WriteString("- 状态默认 `unknown`；未核对不扣分。\n")
	b.WriteString("- 差异类型：`missing` 缺字段 / `conflict` 值不一致 / `stale` 信息过期。\n")
	b.WriteString("- 所有结论为公开信息比对结果，非任何平台的官方判定。\n")
	return domain.FixArtifact{
		Type: "profile_sync_checklist", Filename: "profile_sync_checklist.md",
		TargetPath: "交付包 / profile_sync_checklist.md",
		Validation: "确认每行都附官方链接与证据片段；清单中不存在代认领或代填步骤。",
		Content:    b.String(),
	}
}

func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	if s == "" {
		return "—"
	}
	return s
}

// ---- A-WP-1.4 source_type 全链路与三类来源隔离 ----

// ValidateNumericComponent 是任何含数字组件的强制校验：
// 必须带 source_type、methodology_version 与 n_valid（§8 第 1、7 条）。
func ValidateNumericComponent(name string, p domain.Provenance) error {
	if p.SourceType == "" || p.SourceType == domain.SourceUnknown {
		return fmt.Errorf("%s 缺少 source_type", name)
	}
	if p.MethodologyVersion == "" {
		return fmt.Errorf("%s 缺少 methodology_version", name)
	}
	if p.NValid < 0 {
		return fmt.Errorf("%s 的 n_valid 非法", name)
	}
	return nil
}

// ValidateSourceSeparation 禁止三类来源数据同表合并（A-WP-1.4 / §8 第 1 条）。
// 调用方对每个表格传入该表所有行的来源类型；出现 >1 类即报错。
func ValidateSourceSeparation(tableName string, sources []domain.SourceType) error {
	seen := map[domain.SourceType]bool{}
	for _, s := range sources {
		if s == domain.SourceUnknown {
			continue // unknown 单独存放，不参与合并
		}
		seen[s] = true
	}
	if len(seen) > 1 {
		keys := make([]string, 0, len(seen))
		for k := range seen {
			keys = append(keys, string(k))
		}
		sort.Strings(keys)
		return fmt.Errorf("%s 混合了 %d 类来源（%s），三类来源必须分表展示", tableName, len(seen), strings.Join(keys, ", "))
	}
	return nil
}

// SplitBySourceType 把组件按来源类型分成三张独立表，供 API/报告/UI 使用。
func SplitBySourceType(items []domain.Provenance) map[domain.SourceType][]domain.Provenance {
	out := map[domain.SourceType][]domain.Provenance{}
	for _, s := range domain.AllSourceTypes() {
		out[s] = []domain.Provenance{}
	}
	out[domain.SourceUnknown] = []domain.Provenance{}
	for _, p := range items {
		out[p.SourceType] = append(out[p.SourceType], p)
	}
	return out
}
