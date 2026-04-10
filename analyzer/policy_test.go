package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPolicyMatch_Builtin(t *testing.T) {
	initPolicyLib()
	pm := BuildPolicyMatch("半导体", []string{"芯片", "国产替代"})
	if pm == nil {
		t.Fatal("BuildPolicyMatch returned nil")
	}
	if pm.MatchLevel != "强匹配" {
		t.Errorf("expected 强匹配, got %s", pm.MatchLevel)
	}
	if pm.Score < 80 {
		t.Errorf("expected score >= 80, got %d", pm.Score)
	}
	found := false
	for _, p := range pm.Policies {
		if p.Name == "国产替代" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected policy 国产替代 in results, got %+v", pm.Policies)
	}
}

func TestReloadPolicyLibrary_External(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "policy_library.json")
	content := `{
  "version": "test",
  "updated_at": "2026-04-01T12:00:00+08:00",
  "industries": {
    "测试行业": ["测试政策1", "测试政策2"]
  },
  "concepts": {
    "测试概念": ["测试政策3"]
  }
}`
	if err := os.WriteFile(jsonPath, []byte(content), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	if err := ReloadPolicyLibrary(tmpDir); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	source, _ := GetPolicyLibraryMeta()
	if source != "external" {
		t.Errorf("expected source external, got %s", source)
	}

	pm := BuildPolicyMatch("测试行业", []string{"测试概念"})
	if len(pm.Policies) != 3 {
		t.Errorf("expected 3 policies, got %d", len(pm.Policies))
	}
}

func TestSaveDefaultPolicyLibrary(t *testing.T) {
	tmpDir := t.TempDir()
	if err := SaveDefaultPolicyLibrary(tmpDir); err != nil {
		t.Fatalf("save default failed: %v", err)
	}
	jsonPath := filepath.Join(tmpDir, "policy_library.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Fatal("policy_library.json not created")
	}

	// 加载后验证
	if err := ReloadPolicyLibrary(tmpDir); err != nil {
		t.Fatalf("reload after save failed: %v", err)
	}
	pm := BuildPolicyMatch("半导体", nil)
	if pm.MatchLevel != "强匹配" {
		t.Errorf("expected 强匹配 after loading saved default, got %s", pm.MatchLevel)
	}
}
