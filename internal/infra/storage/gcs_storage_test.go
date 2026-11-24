package storage

import (
	"errors"
	"testing"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// fakeObjectIterator replays a fixed sequence of results from storage.ObjectIterator.
type fakeObjectIterator struct {
	attrs []*storage.ObjectAttrs
	err   error // returned after all attrs are consumed
}

func (f *fakeObjectIterator) Next() (*storage.ObjectAttrs, error) {
	if len(f.attrs) > 0 {
		attrs := f.attrs[0]
		f.attrs = f.attrs[1:]
		return attrs, nil
	}
	return nil, f.err
}

func TestListObjects(t *testing.T) {
	t.Run("collects objects until Done", func(t *testing.T) {
		it := &fakeObjectIterator{
			attrs: []*storage.ObjectAttrs{
				{Name: "a.txt", Size: 10, ContentType: "text/plain"},
				{Name: "b.bin", Size: 20},
			},
			err: iterator.Done,
		}
		objects, err := listObjects(it)
		if err != nil {
			t.Fatalf("listObjects() error = %v, want nil", err)
		}
		if len(objects) != 2 {
			t.Fatalf("len(objects) = %d, want 2", len(objects))
		}
		if objects[0].Key != "a.txt" || objects[0].Size != 10 || objects[0].ContentType != "text/plain" {
			t.Errorf("objects[0] = %+v, want a.txt/10/text/plain", objects[0])
		}
		if objects[1].Key != "b.bin" {
			t.Errorf("objects[1].Key = %q, want b.bin", objects[1].Key)
		}
	})

	t.Run("non-Done iterator error is propagated", func(t *testing.T) {
		boom := errors.New("storage: bucket not found")
		it := &fakeObjectIterator{
			attrs: []*storage.ObjectAttrs{{Name: "a.txt"}},
			err:   boom,
		}
		objects, err := listObjects(it)
		if err == nil {
			t.Fatal("listObjects() error = nil, want a wrapped storage error")
		}
		if !errors.Is(err, boom) {
			t.Errorf("listObjects() error = %v, want wrapping %v", err, boom)
		}
		if objects != nil {
			t.Errorf("listObjects() objects = %v, want nil on error", objects)
		}
	})
}
