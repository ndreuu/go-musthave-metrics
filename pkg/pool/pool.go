// Package pool предоставляет обобщённый контейнер для переиспользования
// «тяжёлых» объектов, которые обладают методом Reset().
//
// Объекты, возвращаемые в пул методом Put, перед помещением в пул
// «сбрасываются» вызовом Reset(), поэтому повторно полученный через Get
// объект находится в начальном состоянии.
package pool

import (
	"reflect"
	"sync"
)

// Resetter ограничивает типы, допустимые для Pool: тип должен иметь
// метод Reset(), приводящий объект к начальному состоянию.
type Resetter interface {
	Reset()
}

// Pool — generic-контейнер, хранящий объекты одного конкретного типа T.
// T должен реализовывать Resetter. Тип T, как правило, является
// указателем на структуру (методы Reset() объявлены на указателях).
type Pool[T Resetter] struct {
	pool sync.Pool
}

// New создаёт и возвращает указатель на пул объектов типа T.
// Если пул пуст, Get вернёт свежевыделенный объект.
func New[T Resetter]() *Pool[T] {
	tp := reflect.TypeOf((*T)(nil)).Elem()

	var newFn func() any
	if tp.Kind() == reflect.Ptr {
		// для указателя на структуру создаём новый экземпляр по типу
		newFn = func() any { return reflect.New(tp.Elem()).Interface() }
	} else {
		// для значения возвращаем нулевое значение
		newFn = func() any { return reflect.Zero(tp).Interface() }
	}

	return &Pool[T]{
		pool: sync.Pool{New: newFn},
	}
}

// Get возвращает объект из пула. Если пул пуст, создаётся новый объект.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put помещает объект в пул. Перед помещением объект сбрасывается
// методом Reset(), поэтому его состояние не влияет на следующих
// получателей из пула.
func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.pool.Put(v)
}