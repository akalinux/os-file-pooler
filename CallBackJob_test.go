package osfp

import "testing"

func TestCallBackPanic(t *testing.T) {
	job := &CallBackJob{
		OnEventCallBack: func(config AsyncEvent) {
			t.Logf("Getting called")
			if config.Error() == nil {
				panic("Should catch this!")
			}
		},
	}

	event := &CallbackEvent{}
	job.safeEvent(event)
	if event.error != ERR_CALLBACK_PANIC {
		t.Fatalf("Did not get an error in the callback")
	}
	event.PollReadWrite()
}

func TestNewJobTimeout(t *testing.T) {
	NewJobTimeout(0, func(ce AsyncEvent) {})
}
