package pool

import "testing"

type genPoolObject struct {
	Meta  map[string]string
	ID    string
	Items []string
	Count int
}

func (g *genPoolObject) Reset() {
	g.ID = ""
	g.Count = 0
	g.Items = g.Items[:0]
	clear(g.Meta)
}

func TestPool_GetReturnsZeroValueWhenEmpty(t *testing.T) {
	p := New[*genPoolObject]()

	obj := p.Get()
	if obj == nil {
		t.Fatal("expected non-nil object from empty pool")
	}
	if obj.ID != "" || obj.Count != 0 {
		t.Errorf("expected zero-value object, got %+v", obj)
	}
}

func TestPool_PutResetsBeforeReuse(t *testing.T) {
	p := New[*genPoolObject]()

	obj := p.Get()
	obj.ID = "abc"
	obj.Count = 42
	obj.Items = append(obj.Items, "x", "y")
	obj.Meta = map[string]string{"k": "v"}

	p.Put(obj)

	reused := p.Get()
	if reused.ID != "" || reused.Count != 0 {
		t.Errorf("expected reset object, got %+v", reused)
	}
	if len(reused.Items) != 0 {
		t.Errorf("expected empty Items after reset, got %v", reused.Items)
	}
	if len(reused.Meta) != 0 {
		t.Errorf("expected empty Meta after reset, got %v", reused.Meta)
	}
}

func TestPool_ReusesSameInstance(t *testing.T) {
	p := New[*genPoolObject]()

	first := p.Get()
	first.ID = "marker"
	p.Put(first)

	second := p.Get()
	if second != first {
		t.Error("expected pool to reuse the same instance")
	}
	if second.ID != "" {
		t.Errorf("expected reset state, got %q", second.ID)
	}
}

func TestPool_ConcurrentGetPut(t *testing.T) {
	p := New[*genPoolObject]()

	const workers = 50
	const iterations = 100

	done := make(chan struct{})

	for i := 0; i < workers; i++ {
		go func() {
			defer func() {
				done <- struct{}{}
			}()

			for j := 0; j < iterations; j++ {
				obj := p.Get()
				obj.Count = j
				p.Put(obj)
			}
		}()
	}

	for i := 0; i < workers; i++ {
		<-done
	}
}
