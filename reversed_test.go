package tree

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReversedOrder(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	mn := 1000
	mmn := mn * mn
	r := NewReversed()
	ar := make([]string, 0, mn)
	for i := 0; i < mn; i++ {
		n := rand.Intn(mmn)
		s := fmt.Sprintf("%08d", n)
		for {
			if r.Get(s) == nil {
				break
			}
			n = rand.Intn(mmn)
			s = fmt.Sprintf("%08d", n)
		}
		ar = append(ar, s)
		r.Set(s, s)
	}
	assert.Equal(t, mn, r.Count())
	for i, s := range r.Names() {
		assert.Equal(t, s, ar[mn-i-1])
	}
	for i, v := range r.Values() {
		assert.Equal(t, v.(string), ar[mn-i-1])
	}
}
