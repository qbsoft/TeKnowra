package einodef

import "testing"

func TestPublicationCapabilitiesAreImmutable(t *testing.T) {
	d, ok := Lookup(CustomerCredit)
	if !ok || !d.AllowsTool("find_customer") {
		t.Fatal("missing published capability")
	}
	d.ReadOnlyTools[0] = "write_customer"
	d, _ = Lookup(CustomerCredit)
	if d.AllowsTool("write_customer") || !d.AllowsTool("find_customer") {
		t.Fatal("caller mutated published permissions")
	}
	if _, ok := Lookup("unknown"); ok {
		t.Fatal("unpublished definition accepted")
	}
	for _, id := range IDs() {
		d, ok := Lookup(id)
		if !ok || d.ID != id || d.Name == "" || (!d.Knowledge && len(d.ReadOnlyTools) == 0) {
			t.Fatalf("invalid capability manifest: %+v", d)
		}
	}
}
