// Package social 承载第三批最小基线（CR-v1.1 决策 2）：独立 feature flag
// 与 source_profile_by_model 六口径的只读投影。与 internal/delta、
// internal/competitor 完全正交，不修改其任何判定逻辑与默认值。
package social

import (
	"sync"

	"github.com/answerlens/answerlens/domain"
)

// Flag 是第三批最小基线总开关。默认 on；
// GEOLENS_FLAG_AEO_SOCIAL_SIGNALS_20261117=off 可秒级关闭，不影响其它 flag。
type Flag struct {
	mu      sync.RWMutex
	enabled bool
}

func NewFlag(enabled bool) *Flag { return &Flag{enabled: enabled} }

func (f *Flag) Enabled() bool {
	if f == nil {
		return false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.enabled
}

func (f *Flag) SetEnabled(v bool) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.enabled = v
	f.mu.Unlock()
}

func (f *Flag) Name() string { return domain.SocialFeatureFlag }

// ProfileDesk 是六口径只读投影的存放处。append 后不得改写历史行。
type ProfileDesk struct {
	mu       sync.RWMutex
	profiles []domain.SourceProfileByModel
}

func NewProfileDesk() *ProfileDesk { return &ProfileDesk{} }

// Append 追加一条口径观测。禁止 method 越界与第七种口径。
func (d *ProfileDesk) Append(p domain.SourceProfileByModel) bool {
	if !domain.IsValidSourceProfileMethod(p.Method) {
		return false
	}
	if p.MethodologyVersion != domain.MethodologyVersionSocial {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.profiles = append(d.profiles, p)
	return true
}

// ByModel 返回某模型的全部口径行（只并列，不合并）。
func (d *ProfileDesk) ByModel(model string) []domain.SourceProfileByModel {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]domain.SourceProfileByModel, 0)
	for _, p := range d.profiles {
		if p.ModelCode == model {
			out = append(out, p)
		}
	}
	return out
}
