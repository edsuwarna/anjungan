package db

import (
	"context"
	"testing"
)

func TestGetSSHKey(t *testing.T) {
	pool := getTestPool(t)
	repo := NewRepository(&DB{Pool: pool})

	key, err := repo.GetSSHKeyByIDFull(context.Background(), "ba2c3b11-a9d5-4423-82aa-b083b3aff546")
	if err != nil {
		t.Fatalf("GetSSHKeyByIDFull failed: %v", err)
	}
	t.Logf("Key: %s / %s / len=%d", key.ID, key.Name, len(key.PrivateKey))
	if len(key.PrivateKey) == 0 {
		t.Fatal("PrivateKey is EMPTY!")
	}
	t.Logf("PrivateKey starts with: %s", key.PrivateKey[:40])
}
