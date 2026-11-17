package storage

import "testing"

func TestTheS3ClientNeedsEverythingItWillSignWith(t *testing.T) {
	good := S3Config{Endpoint: "127.0.0.1:3900", Bucket: "images", AccessKey: "GKabc", SecretKey: "secret"}
	if _, err := NewS3(good); err != nil {
		t.Fatalf("a complete configuration must work: %v", err)
	}
	for name, cfg := range map[string]S3Config{
		"endpoint": {Bucket: "images", AccessKey: "a", SecretKey: "b"},
		"bucket":   {Endpoint: "h:1", AccessKey: "a", SecretKey: "b"},
		"access":   {Endpoint: "h:1", Bucket: "images", SecretKey: "b"},
		"secret":   {Endpoint: "h:1", Bucket: "images", AccessKey: "a"},
	} {
		if _, err := NewS3(cfg); err == nil {
			t.Errorf("without the %s the client must be refused", name)
		}
	}
}
