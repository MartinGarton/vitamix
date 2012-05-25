// Copyright 2012 Petar Maymounkov. All rights reserved.
// Use of this source code is governed by a 
// license that can be found in the LICENSE file.

package vtime

import (
	"sort"
	"time"
)

// Sleep is the virtualized version of time.Sleep
func Sleep(nsec time.Duration) {
	ch := make(chan struct{})
	vch <- &vsleep{
		duration: int64(nsec),
		wake:     ch,
	}
	<-ch
}

type vsleep struct {
	duration int64
	wake     chan struct{}
}

// Now is the virtualized version of time.Now
func Now() time.Time {
	ch := make(chan int64)
	vch <- &vnow{
		resp: ch,
	}
	return time.Unix(0, <-ch)
}

type vnow struct {
	resp chan int64
}

/*
	Go is invoked before go statements in the virtualized source.
	In particular, the virtualizing compiler rewrites go statements like so:

	Original:

		go FuncName()

	Virtualized:

		vtime.Go()
		go func() {
			FuncName()
			vtime.Die()
		}
*/
func Go() {
	vch <- vgo{}
}

type vgo struct{}

// Die is invoked after the end of functions called in go statements in the
// virtualized source. See the doc for Go.
func Die() {
	vch <- vdie{}
}

type vdie struct{}

// Block is invoked before every blocking channel operation (send, receive,
// select statements) in the transformed source
func Block() {
	vch <- vblock{}
}

type vblock struct{}

// Unblock is invoked after every blocking channel operation (send, receive,
// select statements) in the transformed source
func Unblock() {
	vch <- vunblock{}
}

type vunblock struct{}

// Runtime below

var vch chan interface{}

func init() {
	vch = make(chan interface{})
	go loop()
}

func loop() {
	var now     int64  // Current virtual time
	var ngo     int    // Number of active goroutines
	var nblock  int    // Number of blocked goroutines
	var q       queue  // Queue of waiting sleep calls

	ngo = 1     // count the main go routine
	for {
		vcmd := <-vch
		switch t := vcmd.(type) {
		case *vsleep:
			nblock++
			q.Add(makeUntil(t, now))
		case *vnow:
			t.resp <- now
			close(t.resp)
		case vgo:
			ngo++
		case vdie:
			if ngo < 1 {
				panic("no goroutines")
			}
			ngo--
		case vblock:
			nblock++
		case vunblock:
			if nblock < 1 {
				panic("no blocked goroutines")
			}
			nblock--
		}
		if ngo == 0 || nblock < ngo {
			continue
		}
		unsleep := q.DeleteMin()
		if unsleep == nil {
			//fmt.Fprintf(os.Stderr, "spinning\n")
			continue
		}
		nblock--
		if unsleep.when < now {
			panic("negative time")
		}
		now = unsleep.when
		close(unsleep.wake)
	}
	panic("virtual time loop exited")
}

// queue sorts until instances ascending by timestamp
type queue []*until

type until struct {
	when int64
	wake chan struct{}
}

func makeUntil(v *vsleep, now int64) *until {
	return &until{
		when: now + v.duration,
		wake: v.wake,
	}
}

func (t queue) Len() int {
	return len(t)
}

func (t queue) Less(i, j int) bool {
	return t[i].when < t[j].when
}

func (t queue) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t *queue) Add(u *until) {
	*t = append(*t, u)
	sort.Sort(t)
}

func (t *queue) DeleteMin() *until {
	if len(*t) == 0 {
		return nil
	}
	q := (*t)[0]
	*t = (*t)[1:]
	return q
}
