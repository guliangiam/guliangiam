package ldapsync

import (
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
)

func TestNewScheduler(t *testing.T) {
	svc := &Service{}
	interval := 10 * time.Second
	scheduler := NewScheduler(svc, interval, nil)

	if scheduler.service != svc {
		t.Error("scheduler service not set correctly")
	}
	if scheduler.interval != interval {
		t.Errorf("scheduler interval = %v, want %v", scheduler.interval, interval)
	}
	if scheduler.locks == nil {
		t.Error("scheduler lock provider not defaulted")
	}
}

func TestLdapSyncTaskFromResult(t *testing.T) {
	now := time.Now()
	config := &ldaps.LdapSyncConfig{
		ID:             "config-001",
		OrganizationID: "org-001",
		LdapID:         "ldap-001",
	}
	result := &ldaps.LdapSyncResult{
		UsersSynced:   10,
		UsersCreated:  3,
		UsersUpdated:  5,
		UsersFailed:   2,
		GroupsSynced:  1,
		GroupsCreated: 1,
	}

	task := ldapSyncTaskFromResult("task-001", config, result, "scheduled", &now)

	if task.ID != "task-001" {
		t.Errorf("task ID = %q, want task-001", task.ID)
	}
	if task.OrganizationID != "org-001" {
		t.Errorf("task OrganizationID = %q, want org-001", task.OrganizationID)
	}
	if task.LdapID != "ldap-001" {
		t.Errorf("task LdapID = %q, want ldap-001", task.LdapID)
	}
	if task.TriggerType != "scheduled" {
		t.Errorf("task TriggerType = %q, want scheduled", task.TriggerType)
	}
	if task.UsersSynced != 10 {
		t.Errorf("task UsersSynced = %d, want 10", task.UsersSynced)
	}
	if task.UsersCreated != 3 {
		t.Errorf("task UsersCreated = %d, want 3", task.UsersCreated)
	}
	if task.UsersUpdated != 5 {
		t.Errorf("task UsersUpdated = %d, want 5", task.UsersUpdated)
	}
	if task.UsersFailed != 2 {
		t.Errorf("task UsersFailed = %d, want 2", task.UsersFailed)
	}
	if task.Status != "partial" {
		t.Errorf("task Status = %q, want partial (has failed users)", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("task CompletedAt should not be nil")
	}
}

func TestLdapSyncTaskFromResult_PartialFailure(t *testing.T) {
	now := time.Now()
	config := &ldaps.LdapSyncConfig{
		ID:             "config-002",
		OrganizationID: "org-002",
		LdapID:         "ldap-002",
	}
	result := &ldaps.LdapSyncResult{
		UsersFailed: 1,
	}

	task := ldapSyncTaskFromResult("task-002", config, result, "manual", &now)

	if task.Status != "partial" {
		t.Errorf("task Status = %q, want partial when there are failures", task.Status)
	}
}
