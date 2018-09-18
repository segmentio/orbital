package orbital

import (
	"context"
	"testing"
	"time"
)

func init() {
	Register(TestCase{
		Period: 400 * time.Microsecond,
		Func: func(ctx context.Context, o *O) {
			time.Sleep(550 * time.Microsecond)
			o.Log("in test case")
		},
		Name: "smoke test",
	})

	Register(TestCase{
		Period: 300 * time.Microsecond,
		Func: func(ctx context.Context, o *O) {
			time.Sleep(500 * time.Microsecond)
			o.Log("in test case")
		},
		Name: "secondary test",
	})

}

func TestOrbital(t *testing.T) {
	DefaultService.Run()
	time.Sleep(2 * time.Millisecond)
	DefaultService.Close()
}
