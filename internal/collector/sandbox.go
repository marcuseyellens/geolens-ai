package collector

import (
	"time"

	"github.com/answerlens/answerlens/domain"
	"github.com/google/uuid"
)

// LocalSandbox 是本地可运行的沙箱端口实现，用于零配置体验完整采集链路。
//
// 重要：它不发起任何外部请求、不访问真实桌面端，产出的答案明确标注为
// local-simulated，provenance 也标为模拟，绝不冒充第三方实测数据。
// 生产环境应替换为 Offstage Cloud 按秒计费沙箱实现同一个 Sandbox 接口。
type LocalSandbox struct {
	Policy Policy
	Clock  func() time.Time
}

func NewLocalSandbox() *LocalSandbox {
	return &LocalSandbox{Policy: DefaultPolicy(), Clock: time.Now}
}

func (s *LocalSandbox) Acquire(req SandboxRequest) (Session, error) {
	if err := req.Validate(s.Policy); err != nil {
		return Session{Phase: PhaseFailed}, err
	}
	return Session{
		ID: "local-" + uuid.NewString(), Platform: "local-simulated",
		Phase: PhaseDriving, StartedAt: s.Clock(),
	}, nil
}

func (s *LocalSandbox) Drive(_ Session, question string) DriveResult {
	return DriveResult{
		RawAnswerText: "【本地模拟答案，非第三方实测】问题：" + question,
		Citations: []domain.Citation{{
			URL: "https://example.com/public-review", Domain: "example.com",
			PagePath: "/public-review", SourceClass: domain.SourceOther,
		}},
		SearchActivity: domain.SearchOn,
		ForcedSearch:   false,
		Simulated:      true,
	}
}

func (s *LocalSandbox) Release(_ Session) error { return nil }
