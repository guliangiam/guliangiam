package crypto

import (
	"errors"
	"testing"
)

// 完备性证明：生成物 registryCapabilityOrder 里的每个能力位都必须在 suite.go
// 的探测分派表中有实现，且分派表不得包含生成物之外的 ID。
// 反向漏洞是「注册表加了能力位、生成物更新了，但忘记手写探测」。
func TestCapabilityDetectorsCoverGeneratedRegistry(t *testing.T) {
	for _, id := range registryCapabilityOrder {
		if _, ok := capabilityDetectors[id]; !ok {
			t.Errorf("capability %q is declared in contracts/registries/crypto.yaml but has no detector in capabilityDetectors", id)
		}
	}

	generated := make(map[string]bool, len(registryCapabilityOrder))
	for _, id := range registryCapabilityOrder {
		generated[id] = true
	}
	for id := range capabilityDetectors {
		if !generated[id] {
			t.Errorf("detector %q is not declared in contracts/registries/crypto.yaml (generated registry)", id)
		}
	}
}

// failure_mode 派生清单必须与声明顺序完全划分：每个能力位恰好属于
// refuse 或 degrade 之一，防止 YAML 新增能力位时漏写 failure_mode。
func TestGeneratedCapabilityListsPartitionRegistry(t *testing.T) {
	if len(registryCapabilityOrder) == 0 {
		t.Fatal("generated registryCapabilityOrder is empty")
	}

	mode := make(map[string]string, len(registryCapabilityOrder))
	for _, id := range gmRequiredCapabilities {
		if _, ok := mode[id]; ok {
			t.Errorf("capability %q appears in more than one failure_mode list", id)
		}
		mode[id] = "refuse"
	}
	for _, id := range gmDegradableCapabilities {
		if _, ok := mode[id]; ok {
			t.Errorf("capability %q appears in more than one failure_mode list", id)
		}
		mode[id] = "degrade"
	}

	if len(mode) != len(registryCapabilityOrder) {
		t.Fatalf("failure_mode lists cover %d capabilities, registry declares %d", len(mode), len(registryCapabilityOrder))
	}
	for _, id := range registryCapabilityOrder {
		if mode[id] == "" {
			t.Errorf("capability %q has no failure_mode-derived list entry", id)
		}
	}
}

// 单一探测口径：同一个伪造的 SM4 失败注入下，字段加密启用判定
// （sm4Available）与启动能力探测（DetectCapabilities）必须给出相同结论。
func TestSM4CapabilityProbeIsSingleSource(t *testing.T) {
	original := sm4CapabilityProbe
	t.Cleanup(func() { sm4CapabilityProbe = original })

	sm4CapabilityProbe = func() error { return errors.New("sm4 unavailable") }

	if sm4Available() {
		t.Fatal("sm4Available still reports true after the SM4 probe failed")
	}

	var sm4 CapabilityStatus
	for _, capability := range DetectCapabilities() {
		if capability.ID == CapabilitySM4 {
			sm4 = capability
			break
		}
	}
	if sm4.ID == "" {
		t.Fatal("DetectCapabilities did not report sm4")
	}
	if sm4.Available {
		t.Fatalf("DetectCapabilities reports sm4 available after the same injected failure: %+v", sm4)
	}
	if sm4.Detail != "sm4 unavailable" {
		t.Fatalf("sm4 detail = %q, want the injected probe error from the shared seam", sm4.Detail)
	}
}
