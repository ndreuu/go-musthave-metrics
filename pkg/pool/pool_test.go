package pool

import (
	"testing"

	"go-musthave-metrics/internal/agent"
	models "go-musthave-metrics/internal/model"
)

// genPoolObject — структура, имитирующая объект, для которого
// сгенерирован метод Reset(), чтобы проверить сброс перед Put.
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
	// состояние было сброшено
	if second.ID != "" {
		t.Errorf("expected reset state, got %q", second.ID)
	}
}

func TestPool_WorksWithGeneratedResetTypes(t *testing.T) {
	// модели сгенерировали метод Reset() в прошлом инкременте
	pMetrics := New[*models.Metrics]()
	m := pMetrics.Get()
	m.ID = "gauge"
	delta := int64(5)
	m.Delta = &delta
	pMetrics.Put(m)
	rm := pMetrics.Get()
	if rm.ID != "" {
		t.Errorf("expected reset *models.Metrics, got %+v", rm)
	}
	// сгенерированный Reset() зануляет значение по указателю, но не обнуляет сам указатель
	if rm.Delta != nil && *rm.Delta != 0 {
		t.Errorf("expected zero delta after reset, got %d", *rm.Delta)
	}

	pAgent := New[*agent.Metric]()
	am := pAgent.Get()
	am.Name = "Alloc"
	am.Value = 12.5
	pAgent.Put(am)
	ram := pAgent.Get()
	if ram.Name != "" || ram.Value != 0 {
		t.Errorf("expected reset *agent.Metric, got %+v", ram)
	}
}

func TestPool_ConcurrentGetPut(t *testing.T) {
	p := New[*genPoolObject]()

	const workers = 50
	const iterations = 100

	done := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
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