package tree

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

func stacktrace() string {
	array := make([]byte, 2048)
	s := runtime.Stack(array, false)
	return string(array[0:s])
}

type tlog struct {
	out chan string
}

func (tl *tlog) pln(args ...interface{}) {
	tl.out <- fmt.Sprintln(args...)
}

func (tl *tlog) rec(ss string, args ...interface{}) {
	tl.out <- fmt.Sprintln(args...)
	tl.out <- fmt.Sprintln(ss)
}

func (tl *tlog) w(c <-chan interface{}, args ...interface{}) {
	<-c
	tl.out <- fmt.Sprintln(args...)
}

func (tl *tlog) tose(t *testing.T, millis int, pattern string) {
	tl.to(t, millis, func(line string) bool {
		return line == pattern
	})
}

func (tl *tlog) to(t *testing.T, millis int, checker func(string) bool) {
	done := make(chan interface{})
	go func() {
		for {
			timer := time.NewTimer(time.Duration(millis) * time.Millisecond)
			select {
			case <-timer.C:
				t.Error("timeout")
				done <- true
				return
			case line := <-tl.out:
				timer.Stop()
				fmt.Print(line)
				if checker(line) {
					done <- true
					return
				}
			}
		}
	}()
	<-done
}
