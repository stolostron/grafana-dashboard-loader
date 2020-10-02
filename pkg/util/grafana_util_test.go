// Copyright (c) 2020 Red Hat, Inc.

package util

import "testing"

func TestGenerateUID(t *testing.T) {

	uid := GenerateUID("open-cluster-management", "test")
	if uid != "open-cluster-management-test" {
		t.Fatalf("the uid %v is not the expected %v", uid, "open-cluster-management-test")
	}

	uid = GenerateUID("open-cluster-management-observability", "test")
	if uid != "4e20548bdba37201faabf30d1c419981" {
		t.Fatalf("the uid %v should not equal to %v", uid, "4e20548bdba37201faabf30d1c419981")
	}

}
