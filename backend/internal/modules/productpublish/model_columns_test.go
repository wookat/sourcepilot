package productpublish

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

// Raw-SQL update maps in service_worker.go / service_create.go write
// external_spu_id, so the model must map ExternalSPUID to that column
// (GORM's default naming would produce external_sp_uid).
func TestProductPublicationExternalSPUIDColumn(t *testing.T) {
	s, err := schema.Parse(&ProductPublication{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	f := s.LookUpField("ExternalSPUID")
	if f == nil {
		t.Fatal("field ExternalSPUID not found")
	}
	if f.DBName != "external_spu_id" {
		t.Fatalf("ExternalSPUID column = %q, want external_spu_id", f.DBName)
	}
}
