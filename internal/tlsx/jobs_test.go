package tlsx

import "testing"

func TestJobBook(t *testing.T) {
	b := newJobBook()
	b.Begin("app", "issue")
	b.Append("app", "hello")
	j := b.Get("app")
	if j.Status != "running" || len(j.Lines) != 1 {
		t.Fatalf("%+v", j)
	}
	b.Finish("app", nil)
	j = b.Get("app")
	if j.Status != "success" {
		t.Fatal(j.Status)
	}
}
